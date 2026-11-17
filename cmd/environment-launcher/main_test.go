package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/GuoMonth/dsh-isolated-runtime/compat/dsh"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/dshcompat/launcher"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/testutil/leakcheck"
)

func TestMain(m *testing.M) {
	leakcheck.VerifyTestMain(m)
}

func TestManagementBindFailurePreventsChildStart(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	err = serveEnvironment(t.Context(), launcher.Config{
		DSHCommand:      []string{"must-not-be-started"},
		PublicAuthority: "cell.test",
	}, listener.Addr().String())
	if network, ok := errors.AsType[*net.OpError](err); !ok || network.Op != "listen" {
		t.Fatalf("management bind must fail before child start: %v", err)
	}
}

func TestFailedChildStartJoinsManagement(t *testing.T) {
	for range 3 {
		err := serveEnvironment(t.Context(), launcher.Config{
			DSHCommand:      []string{t.TempDir() + "/missing-dsh"},
			PublicAuthority: "cell.test",
		}, "127.0.0.1:0")
		if _, ok := errors.AsType[*os.PathError](err); !ok {
			t.Fatalf("expected child start failure: %v", err)
		}
	}
}

func TestChildStartupCancellationJoinsManagement(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	err := serveEnvironment(ctx, commandFixture("wait"), "127.0.0.1:0")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected startup cancellation: %v", err)
	}
}

func TestUnexpectedChildExitJoinsManagementAndProxy(t *testing.T) {
	err := serveEnvironment(t.Context(), commandFixture("exit"), "127.0.0.1:0")
	if exit, ok := errors.AsType[*exec.ExitError](err); !ok || exit.ExitCode() != 23 {
		t.Fatalf("expected unexpected child exit 23: %v", err)
	}
}

func commandFixture(mode string) launcher.Config {
	return launcher.Config{
		DSHCommand:      []string{os.Args[0], "-test.run=^TestCommandDSHProcess$", "--"},
		Environment:     append(os.Environ(), "DSH_COMMAND_FIXTURE="+mode),
		PublicAuthority: "cell.test",
		ReadyTimeout:    5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}
}

func TestCommandDSHProcess(t *testing.T) {
	switch os.Getenv("DSH_COMMAND_FIXTURE") {
	case "wait":
		ctx, stop := signal.NotifyContext(t.Context(), syscall.SIGTERM)
		defer stop()
		<-ctx.Done()
	case "exit":
		_, _ = fmt.Fprintln(os.Stdout, "dsh web: http://127.0.0.1:9/?token="+"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		os.Exit(23)
	default:
		t.Skip("subprocess fixture")
	}
}

func TestManagementHandler(t *testing.T) {
	t.Parallel()
	var live atomic.Bool
	var ready atomic.Bool
	handler := newManagementHandler(&live, &ready)

	assertStatus := func(path string, want int) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("%s status = %d, want %d", path, response.Code, want)
		}
		return response
	}

	assertStatus("/livez", http.StatusServiceUnavailable)
	assertStatus("/readyz", http.StatusServiceUnavailable)
	live.Store(true)
	ready.Store(true)
	assertStatus("/livez", http.StatusOK)
	assertStatus("/readyz", http.StatusOK)

	response := assertStatus("/version", http.StatusOK)
	var version map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &version); err != nil {
		t.Fatal(err)
	}
	if version["contractVersion"] != dsh.ContractVersion || version["dshVersion"] != dsh.Version() {
		t.Fatalf("unexpected version: %#v", version)
	}
	request := httptest.NewRequest(http.MethodPost, "/quiesce", nil)
	removed := httptest.NewRecorder()
	handler.ServeHTTP(removed, request)
	if removed.Code != http.StatusNotFound {
		t.Fatalf("removed quiesce endpoint status = %d", removed.Code)
	}
}
