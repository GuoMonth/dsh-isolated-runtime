package launcher

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/GuoMonth/dsh-isolated-runtime/internal/accesscontract"
)

const helperToken = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(value)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func TestLauncherBrokersDSHWithoutLeakingItsToken(t *testing.T) {
	var logs lockedBuffer
	instance, err := Start(Config{
		DSHCommand:      []string{os.Args[0], "-test.run=TestDSHHelperProcess", "--"},
		PatchFiles:      []string{"/etc/dsh/cell.patch.yml"},
		Environment:     append(os.Environ(), "GO_WANT_DSH_HELPER=1", "DSH_HELPER_EXPECT_PATCH=/etc/dsh/cell.patch.yml"),
		PublicAuthority: "cell.example.test",
		ListenAddress:   "127.0.0.1:0",
		ReadyTimeout:    5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		LogWriter:       &logs,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := instance.Close(ctx); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	exchange := request(t, client, http.MethodGet, instance.URL+"/", "", nil)
	if exchange.StatusCode != http.StatusSeeOther {
		t.Fatalf("exchange status=%d", exchange.StatusCode)
	}
	setCookie := exchange.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, "HttpOnly") || !strings.Contains(setCookie, "SameSite=Lax") || strings.Contains(setCookie, "SameSite=Strict") || !strings.Contains(setCookie, "Secure") {
		t.Fatalf("unexpected Set-Cookie: %s", setCookie)
	}
	if strings.Contains(exchange.Header.Get("Location"), "token=") {
		t.Fatal("launch token leaked into redirect")
	}
	cookie := strings.SplitN(setCookie, ";", 2)[0]
	_ = exchange.Body.Close()
	secondExchange := request(t, client, http.MethodGet, instance.URL+"/", "", nil)
	if secondExchange.StatusCode != http.StatusSeeOther || secondExchange.Header.Get("Set-Cookie") == "" {
		t.Fatalf("second client bootstrap failed: status=%d", secondExchange.StatusCode)
	}
	_ = secondExchange.Body.Close()
	authorizedRoot := request(t, client, http.MethodGet, instance.URL+"/", "", http.Header{"Cookie": []string{cookie}})
	_ = authorizedRoot.Body.Close()
	if authorizedRoot.StatusCode != http.StatusOK {
		t.Fatalf("authorized root was re-exchanged: status=%d", authorizedRoot.StatusCode)
	}

	headers := http.Header{
		"Cookie": []string{cookie + "; theme=dark; AccessToken-5f93c2e4=access-secret; OauthHMAC-5f93c2e4=hmac-secret; " +
			"OauthExpires-5f93c2e4=123; IdToken-5f93c2e4=id-secret; RefreshToken-5f93c2e4=refresh-secret; " +
			"OauthNonce-5f93c2e4=nonce-secret; CodeVerifier-5f93c2e4=verifier-secret; BearerToken=legacy-secret; " +
			"AccessToken-5f93c2e=short-access; IdToken-0=short-id; OauthHMAC-ab=short-hmac; " +
			"OauthExpires-abc=123; RefreshToken-1234=short-refresh; OauthNonce-abcde=short-nonce; CodeVerifier-ABCDEF=short-verifier"},
		"Origin":               []string{"https://cell.example.test"},
		"Authorization":        []string{"Bearer must-not-reach-dsh"},
		"X-Cell-Principal":     []string{"spoofed"},
		"X-Authenticated-User": []string{"spoofed"},
		"X-Dsh-Oidc-Token":     []string{"must-not-reach-dsh"},
	}
	echo := request(t, client, http.MethodPost, instance.URL+"/api/settings/describe", "{}", headers)
	defer func() { _ = echo.Body.Close() }()
	var observed map[string]string
	if err := json.NewDecoder(echo.Body).Decode(&observed); err != nil {
		t.Fatal(err)
	}
	if echo.StatusCode != http.StatusOK || observed["host"] != "cell.example.test" || observed["origin"] != "https://cell.example.test" {
		t.Fatalf("proxy did not preserve authority facts: status=%d body=%v", echo.StatusCode, observed)
	}
	if observed["authorization"] != "" || observed["principal"] != "" || observed["oidc"] != "" {
		t.Fatalf("proxy forwarded outer identity: %v", observed)
	}
	if observed["cookie"] != cookie+"; theme=dark" {
		t.Fatalf("proxy forwarded unexpected cookies: %q", observed["cookie"])
	}
	for _, setCookie := range echo.Header.Values("Set-Cookie") {
		name, ok := responseCookieName(setCookie)
		if ok && accesscontract.IsEnvoyOAuthCookie(name) {
			t.Fatalf("DSH could overwrite an ingress credential cookie: %q", setCookie)
		}
	}
	if len(echo.Header.Values("Set-Cookie")) != 1 {
		t.Fatal("only the ordinary application cookie may leave the Cell")
	}
	if got := echo.Header.Get("Set-Cookie"); !strings.HasPrefix(got, "dsh-preference=compact") || !strings.Contains(got, "Secure") {
		t.Fatalf("ordinary DSH response cookie was not preserved safely: %q", got)
	}

	get := request(t, client, http.MethodGet, instance.URL+"/download", "", http.Header{"Cookie": []string{cookie}})
	data, err := io.ReadAll(get.Body)
	_ = get.Body.Close()
	if err != nil || string(data) != "artifact" {
		t.Fatalf("GET fetch: body=%q err=%v", data, err)
	}
	head := request(t, client, http.MethodHead, instance.URL+"/download", "", http.Header{"Cookie": []string{cookie}})
	_ = head.Body.Close()
	if head.StatusCode != http.StatusOK || head.ContentLength != int64(len("artifact")) {
		t.Fatalf("HEAD fetch: status=%d length=%d", head.StatusCode, head.ContentLength)
	}

	assertOpaqueUpgrade(t, instance.URL, cookie)
	if strings.Contains(logs.String(), helperToken) || strings.Contains(logs.String(), "token="+helperToken) {
		t.Fatalf("launch token leaked into logs: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "token=<redacted>") {
		t.Fatalf("redacted readiness was not logged: %s", logs.String())
	}
}

func TestNormalizeDSHBrowserCookie(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "strict DSH cookie",
			in:   "dsh-auth-test=signed; Path=/; HttpOnly; SameSite=Strict",
			want: "dsh-auth-test=signed; Path=/; HttpOnly; SameSite=Lax",
		},
		{
			name: "case insensitive attribute",
			in:   "dsh-auth-test=signed; samesite=None; Secure",
			want: "dsh-auth-test=signed; Secure; SameSite=Lax",
		},
		{
			name: "unrelated cookie",
			in:   "application=value; SameSite=Strict",
			want: "application=value; SameSite=Strict",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeDSHBrowserCookie(test.in); got != test.want {
				t.Fatalf("normalizeDSHBrowserCookie(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

func TestReadyURLMustBeLoopbackAndCarryOneToken(t *testing.T) {
	valid := "http://127.0.0.1:3080/?token=" + helperToken
	if _, err := parseReadyURL(valid); err != nil {
		t.Fatalf("valid readiness rejected: %v", err)
	}
	for _, candidate := range []string{
		"https://127.0.0.1:3080/?token=" + helperToken,
		"http://0.0.0.0:3080/?token=" + helperToken,
		"http://user@127.0.0.1:3080/?token=" + helperToken,
		"http://127.0.0.1/?token=" + helperToken,
		"http://127.0.0.1:3080/?token=short",
		"http://127.0.0.1:3080/?token=" + strings.Repeat("!", 43),
		valid + "&other=value",
		valid + "#fragment",
		valid + "&token=" + helperToken,
	} {
		if _, err := parseReadyURL(candidate); err == nil {
			t.Fatalf("invalid readiness accepted: %s", candidate)
		}
	}
}

func TestLauncherClosesIngressWhenDSHCrashes(t *testing.T) {
	instance, err := Start(Config{
		DSHCommand:      []string{os.Args[0], "-test.run=TestDSHHelperProcess", "--"},
		Environment:     append(os.Environ(), "GO_WANT_DSH_HELPER=1", "DSH_HELPER_CRASH_AFTER_READY=1"),
		PublicAuthority: "cell.example.test",
		ReadyTimeout:    5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, dialErr := net.DialTimeout("tcp", strings.TrimPrefix(instance.URL, "http://"), 100*time.Millisecond)
		if dialErr != nil {
			break
		}
		_ = connection.Close()
		time.Sleep(10 * time.Millisecond)
	}
	if connection, dialErr := net.DialTimeout("tcp", strings.TrimPrefix(instance.URL, "http://"), 100*time.Millisecond); dialErr == nil {
		_ = connection.Close()
		t.Fatal("launcher ingress remained open after DSH crashed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if closeErr := instance.Close(ctx); closeErr == nil || !strings.Contains(closeErr.Error(), "exit status 23") {
		t.Fatalf("DSH crash was not surfaced: %v", closeErr)
	}
}

func TestLauncherReadinessTimeoutStopsDSH(t *testing.T) {
	t.Parallel()
	started := time.Now()
	_, err := Start(Config{
		DSHCommand:      []string{os.Args[0], "-test.run=TestDSHHelperProcess", "--"},
		Environment:     append(os.Environ(), "GO_WANT_DSH_HELPER=1", "DSH_HELPER_NO_READY=1"),
		PublicAuthority: "cell.example.test",
		ReadyTimeout:    100 * time.Millisecond,
		ShutdownTimeout: time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "readiness timeout") {
		t.Fatalf("readiness timeout error = %v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("readiness timeout did not terminate the DSH child promptly")
	}
}

func TestCloseBoundsHijackedConnectionsBeforeStoppingDSH(t *testing.T) {
	t.Parallel()
	instance, err := Start(Config{
		DSHCommand:      []string{os.Args[0], "-test.run=TestDSHHelperProcess", "--"},
		Environment:     append(os.Environ(), "GO_WANT_DSH_HELPER=1"),
		PublicAuthority: "cell.example.test",
		ReadyTimeout:    5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	exchange := request(t, client, http.MethodGet, instance.URL+"/", "", nil)
	cookie := strings.SplitN(exchange.Header.Get("Set-Cookie"), ";", 2)[0]
	_ = exchange.Body.Close()
	parsed, err := url.Parse(instance.URL)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", parsed.Host, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	if _, err := fmt.Fprintf(connection, "GET /api/remote.mux HTTP/1.1\r\nHost: cell.example.test\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nCookie: %s\r\n\r\n", cookie); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(connection)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	if err != nil || response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade: response=%v err=%v", response, err)
	}

	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = instance.Close(ctx)
	cancel()
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("hijacked connection was not bounded by the drain deadline")
	}
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := reader.ReadByte(); err == nil {
		t.Fatal("hijacked connection remained open after bounded drain")
	}
}

func request(t *testing.T, client *http.Client, method, target, body string, headers http.Header) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "cell.example.test"
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func assertOpaqueUpgrade(t *testing.T, instanceURL, cookie string) {
	t.Helper()
	parsed, err := url.Parse(instanceURL)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := fmt.Fprintf(conn, "GET /api/remote.mux HTTP/1.1\r\nHost: cell.example.test\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nCookie: %s; IdToken=websocket-secret; RefreshToken=websocket-refresh\r\n\r\n", cookie); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade status=%d", response.StatusCode)
	}
	payload := []byte("opaque-websocket-frame")
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	echoed := make([]byte, len(payload))
	if _, err := io.ReadFull(reader, echoed); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(echoed, payload) {
		t.Fatalf("upgrade payload changed: %q", echoed)
	}
}

// TestDSHHelperProcess is a process-level fixture with the exact surface the
// launcher must preserve. The real DSH contract is verified separately by the
// pinned upstream suite in compat/dsh.
func TestDSHHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_DSH_HELPER") != "1" {
		return
	}
	authority := argumentAfter("--trusted-host")
	if expected := os.Getenv("DSH_HELPER_EXPECT_PATCH"); expected != "" && argumentAfter("--patch") != expected {
		os.Exit(4)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	server := &http.Server{Handler: helperHandler(authority)}
	go func() { _ = server.Serve(listener) }()
	if os.Getenv("DSH_HELPER_NO_READY") != "1" {
		fmt.Printf("dsh web: http://%s/?token=%s\n", listener.Addr().String(), helperToken)
	}
	if os.Getenv("DSH_HELPER_CRASH_AFTER_READY") == "1" {
		time.Sleep(100 * time.Millisecond)
		os.Exit(23)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	<-signals
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	os.Exit(0)
}

func argumentAfter(name string) string {
	for index, value := range os.Args {
		if value == name && index+1 < len(os.Args) {
			return os.Args[index+1]
		}
	}
	return ""
}

func helperHandler(authority string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Host != authority {
			http.Error(writer, "forbidden", http.StatusForbidden)
			return
		}
		if request.URL.Path == "/" {
			if strings.Contains(request.Header.Get("Cookie"), "dsh-auth-test=signed") {
				writer.WriteHeader(http.StatusOK)
				return
			}
			if request.URL.Query().Get("token") != helperToken {
				http.Error(writer, "unauthorized", http.StatusUnauthorized)
				return
			}
			writer.Header().Set("Set-Cookie", "dsh-auth-test=signed; Path=/; HttpOnly; SameSite=Strict")
			writer.Header().Set("Location", "/")
			writer.WriteHeader(http.StatusSeeOther)
			return
		}
		if !strings.Contains(request.Header.Get("Cookie"), "dsh-auth-test=signed") {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		for _, cookie := range request.Cookies() {
			if accesscontract.IsEnvoyOAuthCookie(cookie.Name) {
				http.Error(writer, "oauth cookie crossed Cell boundary", http.StatusBadRequest)
				return
			}
		}
		switch request.URL.Path {
		case "/api/settings/describe":
			writer.Header().Set("Content-Type", "application/json")
			writer.Header().Add("Set-Cookie", "AccessToken-5f93c2e4=child-forged; Path=/; Domain=.cells.test")
			writer.Header().Add("Set-Cookie", "IdToken-5f93c2e=child-forged-short; Path=/; Domain=.cells.test")
			writer.Header().Add("Set-Cookie", "dsh-preference=compact; Path=/")
			_ = json.NewEncoder(writer).Encode(map[string]string{
				"host": request.Host, "origin": request.Header.Get("Origin"),
				"authorization": request.Header.Get("Authorization"),
				"principal":     request.Header.Get("X-Cell-Principal"),
				"oidc":          request.Header.Get("X-Dsh-Oidc-Token"),
				"cookie":        request.Header.Get("Cookie"),
			})
		case "/download":
			writer.Header().Set("Content-Length", fmt.Sprint(len("artifact")))
			if request.Method != http.MethodHead {
				_, _ = io.WriteString(writer, "artifact")
			}
		case "/api/remote.mux":
			hijacker, ok := writer.(http.Hijacker)
			if !ok {
				http.Error(writer, "upgrade unavailable", http.StatusInternalServerError)
				return
			}
			conn, buffered, err := hijacker.Hijack()
			if err != nil {
				return
			}
			_, _ = buffered.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
			_ = buffered.Flush()
			_, _ = io.Copy(conn, conn)
			_ = conn.Close()
		default:
			http.NotFound(writer, request)
		}
	})
}
