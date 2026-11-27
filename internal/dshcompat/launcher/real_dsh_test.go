package launcher

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRealDSHBrowserExchange(t *testing.T) {
	cli := os.Getenv("DSH_REAL_CLI")
	if cli == "" {
		t.Skip("DSH_REAL_CLI is set by the exact-upstream compatibility gate")
	}

	patch, err := filepath.Abs(filepath.Join("..", "..", "..", "images", "environment", "environment.patch.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var logs lockedBuffer
	instance, err := Start(Config{
		DSHCommand:         []string{"node", cli},
		PatchFiles:         []string{patch},
		WorkingDir:         t.TempDir(),
		Environment:        append(os.Environ(), "DSH_HOME="+t.TempDir(), "DSH_TELEMETRY_DISABLED=1"),
		PublicAuthority:    "cell.test",
		AdvertisePublicURL: true,
		ReadyTimeout:       90 * time.Second,
		LogWriter:          &logs,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if closeErr := instance.Close(ctx); closeErr != nil {
			t.Errorf("close real DSH: %v", closeErr)
		}
		if !strings.Contains(logs.String(), "dsh web: https://cell.test/") {
			t.Error("DSH did not advertise the configured HTTPS origin")
		}
	})

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, instance.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "cell.test"
	exchange, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = exchange.Body.Close() }()
	if exchange.StatusCode != http.StatusSeeOther {
		t.Fatalf("token exchange status = %d; stdout/stderr:\n%s", exchange.StatusCode, logs.String())
	}
	if strings.Contains(exchange.Header.Get("Location"), "token=") {
		t.Fatal("launch token escaped into the public redirect")
	}
	cookies := exchange.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatal("expected one secure, HttpOnly native browser cookie")
	}
	credentialPattern := regexp.MustCompile(`[?&]token=[A-Za-z0-9_-]{43}`)
	if strings.Contains(logs.String(), cookies[0].Value) || credentialPattern.MatchString(logs.String()) {
		t.Fatal("credential appeared in launcher logs")
	}

	authorized, err := http.NewRequestWithContext(context.Background(), http.MethodGet, instance.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	authorized.Host = "cell.test"
	authorized.AddCookie(cookies[0])
	response, err := client.Do(authorized)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("authorized root status = %d", response.StatusCode)
	}
	for _, test := range []struct {
		name, authority, origin string
		cookie                  bool
		status                  int
	}{
		{name: "anonymous API", authority: "cell.test", origin: "https://cell.test", status: http.StatusUnauthorized},
		{name: "foreign Origin", authority: "cell.test", origin: "https://foreign.test", cookie: true, status: http.StatusForbidden},
		{name: "foreign authority", authority: "foreign.test", origin: "https://foreign.test", cookie: true, status: http.StatusForbidden},
		{name: "native settings", authority: "cell.test", origin: "https://cell.test", cookie: true, status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, instance.URL+"/api/settings/describe", strings.NewReader(`{"type":"client-request","rpcId":"compat","method":"settings/describe","payload":{"args":{}}}`))
			if err != nil {
				t.Fatal(err)
			}
			request.Host = test.authority
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Content-Type", "application/json")
			if test.cookie {
				request.AddCookie(cookies[0])
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			if response.StatusCode != test.status {
				t.Fatalf("native request status = %d, want %d", response.StatusCode, test.status)
			}
			if test.status == http.StatusOK {
				var body struct {
					Result struct {
						OK    bool
						Value struct{ Writable bool }
					}
				}
				if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if !body.Result.OK || !body.Result.Value.Writable {
					t.Fatal("native authenticated-browser settings must remain writable")
				}
			}
		})
	}
}
