package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

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

	var live atomic.Bool
	var ready atomic.Bool
	live.Store(true)
	management := &http.Server{
		Addr:              fmt.Sprintf(":%d", 8081),
		Handler:           newManagementHandler(&live, &ready),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	managementErrors := make(chan error, 1)
	go func() {
		err := management.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			managementErrors <- err
		}
	}()

	instance, err := launcher.StartContext(runCtx, launcher.Config{
		// The official web profile uses the Node module loader's
		// watch service and therefore requires Node's internal loader API.
		DSHCommand:      []string{"/usr/local/bin/node", "--expose-internals", "/opt/dsh/node_modules/@deepseek-ai/dsh/lib/bin.js"},
		PatchFiles:      []string{"/etc/dsh/environment.patch.yml"},
		WorkingDir:      "/var/lib/dsh/data/workspace",
		Environment:     os.Environ(),
		PublicAuthority: authority,
		ListenAddress:   fmt.Sprintf(":%d", 8080),
		ReadyTimeout:    readyTimeout,
		ShutdownTimeout: shutdownTimeout,
		LogWriter:       os.Stdout,
	})
	if err != nil {
		live.Store(false)
		shutdownManagement(management)
		return err
	}
	ready.Store(true)

	for {
		select {
		case <-runCtx.Done():
			ready.Store(false)
			ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
			err := instance.Close(ctx)
			cancel()
			live.Store(false)
			shutdownManagement(management)
			return err
		case <-instance.Done():
			ready.Store(false)
			live.Store(false)
			shutdownManagement(management)
			if err := instance.Wait(); err != nil {
				return fmt.Errorf("DSH exited unexpectedly: %w", err)
			}
			return errors.New("DSH exited unexpectedly without an error")
		case err := <-managementErrors:
			ready.Store(false)
			ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
			closeErr := instance.Close(ctx)
			cancel()
			live.Store(false)
			return errors.Join(fmt.Errorf("management server: %w", err), closeErr)
		}
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
			ContractVersion: "dsh-rc2-single-pvc-v1",
			DSHVersion:      "0.2.0-rc.2",
		})
	})
	return mux
}

func shutdownManagement(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
	}
}
