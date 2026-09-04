package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

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
	if version["contractVersion"] != cellcontract.ContractVersion || version["dshVersion"] != cellcontract.DSHVersion {
		t.Fatalf("unexpected version: %#v", version)
	}
	request := httptest.NewRequest(http.MethodPost, "/quiesce", nil)
	removed := httptest.NewRecorder()
	handler.ServeHTTP(removed, request)
	if removed.Code != http.StatusNotFound {
		t.Fatalf("removed quiesce endpoint status = %d", removed.Code)
	}
}
