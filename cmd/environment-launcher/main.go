package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/GuoMonth/dsh-isolated-runtime/compat/dsh"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/dshcompat/launcher"
)

const (
	readyTimeout    = 90 * time.Second
	drainTimeout    = 5 * time.Second
	shutdownTimeout = 20 * time.Second
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "environment-launcher: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	authority := os.Getenv("DSH_AUTHORITY")
	if authority == "" {
		return errors.New("DSH_AUTHORITY is required")
	}
	for _, directory := range []string{
		"/var/lib/dsh/data/dsh",
		"/var/lib/dsh/data/home",
		"/var/lib/dsh/data/workspace",
		filepath.Join("/tmp", ".cache"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("prepare %s: %w", directory, err)
		}
	}

	return serveEnvironment(runCtx, launcher.Config{
		// Launch the supported official web profile with plain Node.
		DSHCommand:      []string{"/usr/local/bin/node", "/opt/dsh/node_modules/@deepseek-ai/dsh/lib/bin.js"},
		PatchFiles:      []string{"/etc/dsh/environment.patch.yml"},
		WorkingDir:      "/var/lib/dsh/data/workspace",
		Environment:     os.Environ(),
		PublicAuthority: authority,
		ListenAddress:   ":8080",
		ReadyTimeout:    readyTimeout,
		ShutdownTimeout: shutdownTimeout,
		LogWriter:       os.Stdout,
	}, ":8081")
}

func serveEnvironment(ctx context.Context, cfg launcher.Config, managementAddress string) (result error) {
	var live atomic.Bool
	var ready atomic.Bool
	live.Store(true)
	management := &http.Server{
		Addr:              managementAddress,
		Handler:           newManagementHandler(&live, &ready),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", management.Addr)
	if err != nil {
		return fmt.Errorf("management listen: %w", err)
	}
	managementErrors := make(chan error, 1)
	var background sync.WaitGroup
	background.Go(func() {
		managementErrors <- management.Serve(listener)
	})
	defer func() {
		ready.Store(false)
		live.Store(false)
		err := shutdownManagement(management)
		background.Wait()
		if err != nil {
			result = errors.Join(result, fmt.Errorf("close management server: %w", err))
		}
	}()

	instance, err := launcher.StartContext(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		ready.Store(false)
		drain, cancel := context.WithTimeout(context.Background(), drainTimeout)
		defer cancel()
		if err := instance.Close(drain); err != nil {
			result = errors.Join(result, fmt.Errorf("close launcher: %w", err))
		}
	}()
	ready.Store(true)

	select {
	case <-ctx.Done():
		return nil
	case <-instance.Done():
		if err := instance.Wait(); err != nil {
			return fmt.Errorf("DSH exited unexpectedly: %w", err)
		}
		return errors.New("DSH exited unexpectedly without an error")
	case err := <-managementErrors:
		return fmt.Errorf("management server: %w", err)
	}
}

func newManagementHandler(live, ready *atomic.Bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(response http.ResponseWriter, _ *http.Request) {
		if !live.Load() {
			http.Error(response, "not live", http.StatusServiceUnavailable)
			return
		}
		response.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /readyz", func(response http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			http.Error(response, "not ready", http.StatusServiceUnavailable)
			return
		}
		response.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /version", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(struct {
			ContractVersion string `json:"contractVersion"`
			DSHVersion      string `json:"dshVersion"`
		}{
			ContractVersion: dsh.ContractVersion,
			DSHVersion:      dsh.Version(),
		})
	})
	return mux
}

func shutdownManagement(server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(ctx)
	if shutdownErr == nil {
		return nil
	}
	closeErr := server.Close()
	if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
		return errors.Join(shutdownErr, closeErr)
	}
	if errors.Is(shutdownErr, context.DeadlineExceeded) || errors.Is(shutdownErr, net.ErrClosed) {
		return nil
	}
	return shutdownErr
}
