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

	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/dshcompat/launcher"
)

const (
	readyTimeout    = 90 * time.Second
	drainTimeout    = 5 * time.Second
	shutdownTimeout = 20 * time.Second
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "cell-launcher: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	authority := os.Getenv("CELL_AUTHORITY")
	if authority == "" {
		return errors.New("CELL_AUTHORITY is required")
	}
	for _, directory := range []string{
		cellcontract.DSHHome,
		cellcontract.AgentsHome,
		cellcontract.Workspace,
		cellcontract.PrivateRoot,
		filepath.Join(cellcontract.TemporaryRoot, ".cache"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("prepare %s: %w", directory, err)
		}
	}

	var live atomic.Bool
	var ready atomic.Bool
	live.Store(true)
	management := &http.Server{
		Addr:              fmt.Sprintf(":%d", cellcontract.ManagementPort),
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

	instance, err := launcher.Start(launcher.Config{
		// The official alpha.4 web profile uses the Node module loader's
		// watch service and therefore requires Node's internal loader API.
		DSHCommand:      []string{cellcontract.NodePath, "--expose-internals", cellcontract.DSHPath},
		PatchFiles:      []string{cellcontract.PatchPath},
		WorkingDir:      cellcontract.Workspace,
		Environment:     os.Environ(),
		PublicAuthority: authority,
		ListenAddress:   fmt.Sprintf(":%d", cellcontract.ProxyContainerPort),
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

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)

	select {
	case <-signals:
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
			ContractVersion: cellcontract.ContractVersion,
			DSHVersion:      cellcontract.DSHVersion,
		})
	})
	return mux
}

func shutdownManagement(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
