package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/dshcompat/launcher"
)

var errQuiesceConflict = errors.New("another snapshot operation already quiesced this Cell")

type quiesceRequest struct {
	operationUID string
	result       chan error
	acknowledged chan struct{}
}

type quiescePayload struct {
	OperationUID string `json:"operationUID"`
}

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
	quiesceRequests := make(chan quiesceRequest)
	management := &http.Server{
		Addr:              fmt.Sprintf(":%d", cellcontract.ManagementPort),
		Handler:           newManagementHandler(&live, &ready, quiesceRequests),
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

	completedOperation, quiesced, err := readQuiesceMarker(cellcontract.QuiesceMarker)
	if err != nil {
		live.Store(false)
		shutdownManagement(management)
		return err
	}
	if quiesced {
		return waitWhileQuiesced(management, &live, quiesceRequests, managementErrors, completedOperation)
	}

	instance, err := launcher.Start(launcher.Config{
		// The official 0.1.2 RC web profile uses the Node module loader's
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

	for {
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
		case request := <-quiesceRequests:
			ready.Store(false)
			ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
			quiesceErr := instance.Close(ctx)
			cancel()
			if quiesceErr == nil {
				quiesceErr = writeQuiesceMarker(cellcontract.QuiesceMarker, request.operationUID)
			}
			respondToQuiesce(request, quiesceErr)
			if quiesceErr != nil {
				live.Store(false)
				shutdownManagement(management)
				return quiesceErr
			}
			return waitWhileQuiesced(management, &live, quiesceRequests, managementErrors, request.operationUID)
		}
	}
}

func newManagementHandler(live, ready *atomic.Bool, quiesceRequests chan<- quiesceRequest) http.Handler {
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
	mux.HandleFunc("POST "+cellcontract.QuiescePath, func(response http.ResponseWriter, request *http.Request) {
		decoder := json.NewDecoder(io.LimitReader(request.Body, 1024))
		decoder.DisallowUnknownFields()
		var payload quiescePayload
		if err := decoder.Decode(&payload); err != nil || !validOperationUID(payload.OperationUID) {
			http.Error(response, "invalid quiesce request", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			http.Error(response, "invalid quiesce request", http.StatusBadRequest)
			return
		}

		operation := quiesceRequest{
			operationUID: payload.OperationUID,
			result:       make(chan error, 1),
			acknowledged: make(chan struct{}),
		}
		select {
		case quiesceRequests <- operation:
		case <-request.Context().Done():
			return
		}
		defer close(operation.acknowledged)
		select {
		case err := <-operation.result:
			switch {
			case err == nil:
				response.WriteHeader(http.StatusNoContent)
			case errors.Is(err, errQuiesceConflict):
				http.Error(response, "Cell is quiesced for another operation", http.StatusConflict)
			default:
				http.Error(response, "Cell quiesce failed", http.StatusInternalServerError)
			}
		case <-request.Context().Done():
		}
	})
	return mux
}

func waitWhileQuiesced(
	management *http.Server,
	live *atomic.Bool,
	requests <-chan quiesceRequest,
	managementErrors <-chan error,
	operationUID string,
) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	for {
		select {
		case <-signals:
			live.Store(false)
			shutdownManagement(management)
			return nil
		case err := <-managementErrors:
			live.Store(false)
			return fmt.Errorf("management server: %w", err)
		case request := <-requests:
			if request.operationUID == operationUID {
				respondToQuiesce(request, nil)
			} else {
				respondToQuiesce(request, errQuiesceConflict)
			}
		}
	}
}

func respondToQuiesce(request quiesceRequest, err error) {
	request.result <- err
	select {
	case <-request.acknowledged:
	case <-time.After(time.Second):
	}
}

func validOperationUID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

func readQuiesceMarker(path string) (string, bool, error) {
	value, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read quiesce marker: %w", err)
	}
	operationUID := strings.TrimSpace(string(value))
	if !validOperationUID(operationUID) {
		return "", false, errors.New("invalid quiesce marker")
	}
	return operationUID, true, nil
}

func writeQuiesceMarker(path, operationUID string) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(operationUID+"\n"), 0o600); err != nil {
		return fmt.Errorf("write quiesce marker: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit quiesce marker: %w", err)
	}
	return nil
}

func shutdownManagement(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
