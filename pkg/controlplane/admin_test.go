package controlplane

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime/kubernetes"
)

func TestAdminLoginAndCreateRuntime(t *testing.T) {
	backend := kubernetes.NewMemory()
	server, err := NewAdminServer(backend)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{"username": {AdminUsername}, "password": {AdminPassword}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("login status=%d", res.Code)
	}
	cookies := res.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing session cookie")
	}

	create := url.Values{"name": {"demo"}, "tenant": {"tenant-a"}, "image": {"nginxinc/nginx-unprivileged:alpine"}, "securityClass": {"standard"}, "networkIsolation": {"on"}}
	req = httptest.NewRequest(http.MethodPost, "/runtimes", strings.NewReader(create.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookies[0])
	res = httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther {
		t.Fatalf("create status=%d body=%s", res.Code, res.Body.String())
	}
	if _, err := backend.Get(req.Context(), "tenant-a", "demo"); err != nil {
		t.Fatalf("runtime not created: %v", err)
	}
}

func TestAdminAPIBasicAuth(t *testing.T) {
	backend := kubernetes.NewMemory()
	server, err := NewAdminServer(backend)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(runtime.Spec{Name: "api-demo", Tenant: "tenant-a", Image: "nginxinc/nginx-unprivileged:alpine", SecurityClass: runtime.SecurityStandard})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtimes", bytes.NewReader(payload))
	req.SetBasicAuth(AdminUsername, AdminPassword)
	res := httptest.NewRecorder()
	server.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
}
