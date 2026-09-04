package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

func TestManagementHandler(t *testing.T) {
	t.Parallel()
	var live atomic.Bool
	var ready atomic.Bool
	quiesceRequests := make(chan quiesceRequest)
	handler := newManagementHandler(&live, &ready, quiesceRequests)

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
	if version["contractVersion"] != cellcontract.ContractVersion || version["dshVersion"] != cellcontract.DSHVersion {
		t.Fatalf("unexpected version: %#v", version)
	}

	quiesceDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		request := httptest.NewRequest(http.MethodPost, cellcontract.QuiescePath, strings.NewReader(`{"operationUID":"12345678-abcd"}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		quiesceDone <- response
	}()
	operation := <-quiesceRequests
	if operation.operationUID != "12345678-abcd" {
		t.Fatalf("operation UID = %q", operation.operationUID)
	}
	operation.result <- nil
	response = <-quiesceDone
	if response.Code != http.StatusNoContent {
		t.Fatalf("quiesce status = %d", response.Code)
	}
}

func TestQuiesceMarker(t *testing.T) {
	path := t.TempDir() + "/marker"
	if _, exists, err := readQuiesceMarker(path); err != nil || exists {
		t.Fatalf("missing marker: exists=%t err=%v", exists, err)
	}
	if err := writeQuiesceMarker(path, "12345678-abcd"); err != nil {
		t.Fatal(err)
	}
	operationUID, exists, err := readQuiesceMarker(path)
	if err != nil || !exists || operationUID != "12345678-abcd" {
		t.Fatalf("marker: uid=%q exists=%t err=%v", operationUID, exists, err)
	}
}

func TestQuiescedStateRetriesAndRejectsAnotherOperation(t *testing.T) {
	t.Parallel()
	requests := make(chan quiesceRequest)
	managementErrors := make(chan error, 1)
	var live atomic.Bool
	live.Store(true)
	done := make(chan error, 1)
	go func() {
		done <- waitWhileQuiesced(&http.Server{}, &live, requests, managementErrors, "snapshot-a")
	}()

	request := func(operationUID string) error {
		t.Helper()
		operation := quiesceRequest{
			operationUID: operationUID,
			result:       make(chan error, 1),
			acknowledged: make(chan struct{}),
		}
		requests <- operation
		err := <-operation.result
		close(operation.acknowledged)
		return err
	}
	if err := request("snapshot-a"); err != nil {
		t.Fatalf("idempotent retry failed: %v", err)
	}
	if err := request("snapshot-b"); !errors.Is(err, errQuiesceConflict) {
		t.Fatalf("competing operation error = %v", err)
	}

	sentinel := errors.New("management stopped")
	managementErrors <- sentinel
	select {
	case err := <-done:
		if !errors.Is(err, sentinel) {
			t.Fatalf("wait error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("quiesced state did not stop")
	}
}

func TestManagementHandlerRejectsInvalidQuiescePayload(t *testing.T) {
	t.Parallel()
	var live atomic.Bool
	var ready atomic.Bool
	handler := newManagementHandler(&live, &ready, make(chan quiesceRequest))
	for _, body := range []string{
		`{}`,
		`{"operationUID":"UPPERCASE"}`,
		`{"operationUID":"valid-id","extra":true}`,
		`{"operationUID":"valid-id"}{}`,
	} {
		request := httptest.NewRequest(http.MethodPost, cellcontract.QuiescePath, strings.NewReader(body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("payload %q status = %d", body, response.Code)
		}
	}
}
