package launcher

import (
	"context"
	"net/http"
	"os"
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

	var logs lockedBuffer
	instance, err := Start(Config{
		DSHCommand:      []string{"node", cli},
		WorkingDir:      t.TempDir(),
		Environment:     append(os.Environ(), "DSH_HOME="+t.TempDir(), "DSH_TELEMETRY_DISABLED=1"),
		PublicAuthority: "cell.test",
		ReadyTimeout:    90 * time.Second,
		LogWriter:       &logs,
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
		t.Fatalf("token exchange status = %d", exchange.StatusCode)
	}
	if strings.Contains(exchange.Header.Get("Location"), "token=") {
		t.Fatal("launch token escaped into the public redirect")
	}
	cookies := exchange.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatalf("unexpected browser cookie: %#v", cookies)
	}
	credentialPattern := regexp.MustCompile(`[?&]token=[A-Za-z0-9_-]{43}`)
	if strings.Contains(logs.String(), cookies[0].Value) || credentialPattern.MatchString(logs.String()) {
		t.Fatalf("credential appeared in launcher logs: %q", logs.String())
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
}
