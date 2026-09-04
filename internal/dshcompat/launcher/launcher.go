// Package launcher owns the Cell-local DSH child process and exposes its
// authority-bound access seam without releasing the launch token.
package launcher

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/GuoMonth/dsh-isolated-runtime/internal/accesscontract"
)

const (
	defaultReadyTimeout    = 90 * time.Second
	defaultShutdownTimeout = 10 * time.Second
)

var (
	readyURLPattern = regexp.MustCompile(`dsh web: (http://[^\s)]+)`)
	tokenPattern    = regexp.MustCompile(`([?&]token=)[^\s)]+`)
	tokenValue      = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

// Config describes one Cell-local DSH process and its public proxy endpoint.
type Config struct {
	// DSHCommand is the executable and fixed prefix arguments. The launcher
	// appends: web, configured --patch overlays, and the fixed web flags.
	DSHCommand      []string
	PatchFiles      []string
	WorkingDir      string
	Environment     []string
	PublicAuthority string
	ListenAddress   string
	ReadyTimeout    time.Duration
	ShutdownTimeout time.Duration
	LogWriter       io.Writer
}

// Instance is a running DSH child and its Cell-local access endpoint.
type Instance struct {
	URL string

	server          *http.Server
	listener        net.Listener
	process         *os.Process
	processState    *processState
	shutdownTimeout time.Duration
	draining        *atomic.Bool
	connections     *connectionTracker
	closeOnce       sync.Once
	closeErr        error
}

type readiness struct {
	target *url.URL
	token  string
}

type processState struct {
	done chan struct{}
	mu   sync.Mutex
	err  error
}

// Start launches DSH, captures its process-only launch token, and starts an
// opaque HTTP/WebSocket proxy. The raw token is never written to LogWriter.
func Start(cfg Config) (*Instance, error) {
	if len(cfg.DSHCommand) == 0 || strings.TrimSpace(cfg.DSHCommand[0]) == "" {
		return nil, errors.New("launcher: DSH command is required")
	}
	if err := validateAuthority(cfg.PublicAuthority); err != nil {
		return nil, err
	}
	if cfg.ListenAddress == "" {
		cfg.ListenAddress = "127.0.0.1:0"
	}
	if cfg.ReadyTimeout <= 0 {
		cfg.ReadyTimeout = defaultReadyTimeout
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = defaultShutdownTimeout
	}
	if cfg.LogWriter == nil {
		cfg.LogWriter = io.Discard
	}

	args := append([]string(nil), cfg.DSHCommand[1:]...)
	args = append(args, "web")
	for _, patch := range cfg.PatchFiles {
		if strings.TrimSpace(patch) == "" {
			return nil, errors.New("launcher: patch path cannot be empty")
		}
		args = append(args, "--patch", patch)
	}
	args = append(args, "--no-open", "--port", "0", "--trusted-host", cfg.PublicAuthority)
	cmd := exec.Command(cfg.DSHCommand[0], args...)
	cmd.Dir = cfg.WorkingDir
	if cfg.Environment != nil {
		cmd.Env = append([]string(nil), cfg.Environment...)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("launcher: DSH stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("launcher: DSH stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launcher: start DSH: %w", err)
	}

	state := &processState{done: make(chan struct{})}
	go state.complete(cmd.Wait)
	ready := make(chan readiness, 1)
	var readyOnce sync.Once
	var logMu sync.Mutex
	scan := func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if match := readyURLPattern.FindStringSubmatch(line); len(match) == 2 {
				if observed, parseErr := parseReadyURL(match[1]); parseErr == nil {
					readyOnce.Do(func() { ready <- observed })
				}
			}
			logMu.Lock()
			_, _ = fmt.Fprintln(cfg.LogWriter, redactTokens(line))
			logMu.Unlock()
		}
	}
	go scan(stdout)
	go scan(stderr)

	timer := time.NewTimer(cfg.ReadyTimeout)
	defer timer.Stop()
	var observed readiness
	select {
	case observed = <-ready:
	case <-state.done:
		return nil, fmt.Errorf("launcher: DSH exited before readiness: %w", exitError(state.result()))
	case <-timer.C:
		_ = terminate(cmd.Process, cfg.ShutdownTimeout, state)
		return nil, errors.New("launcher: DSH readiness timeout")
	}

	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		_ = terminate(cmd.Process, cfg.ShutdownTimeout, state)
		return nil, fmt.Errorf("launcher: listen: %w", err)
	}
	proxy := newProxy(observed.target, observed.token)
	connections := newConnectionTracker()
	draining := &atomic.Bool{}
	server := &http.Server{
		Handler: http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			if draining.Load() {
				response.Header().Set("Connection", "close")
				http.Error(response, "Cell is stopping", http.StatusServiceUnavailable)
				return
			}
			proxy.ServeHTTP(response, request)
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	instance := &Instance{
		URL:             "http://" + listener.Addr().String(),
		server:          server,
		listener:        listener,
		process:         cmd.Process,
		processState:    state,
		shutdownTimeout: cfg.ShutdownTimeout,
		draining:        draining,
		connections:     connections,
	}
	go func() {
		tracked := &trackingListener{Listener: listener, tracker: connections}
		if serveErr := server.Serve(tracked); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logMu.Lock()
			_, _ = fmt.Fprintf(cfg.LogWriter, "launcher: proxy stopped: %v\n", serveErr)
			logMu.Unlock()
		}
	}()
	go func() {
		<-state.done
		_ = server.Close()
	}()
	return instance, nil
}

// Close drains the proxy, sends SIGTERM to DSH, and escalates to SIGKILL only
// after the configured shutdown timeout.
func (i *Instance) Close(ctx context.Context) error {
	i.closeOnce.Do(func() {
		i.draining.Store(true)
		serverErr := i.server.Shutdown(ctx)
		if serverErr == nil {
			serverErr = i.connections.wait(ctx)
		}
		if errors.Is(serverErr, context.DeadlineExceeded) || errors.Is(serverErr, context.Canceled) {
			_ = i.server.Close()
			i.connections.closeAll()
			serverErr = nil
		}
		processErr := terminate(i.process, i.shutdownTimeout, i.processState)
		i.closeErr = errors.Join(serverErr, processErr)
	})
	return i.closeErr
}

type trackingListener struct {
	net.Listener
	tracker *connectionTracker
}

func (l *trackingListener) Accept() (net.Conn, error) {
	connection, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return l.tracker.track(connection), nil
}

type trackedConnection struct {
	net.Conn
	tracker *connectionTracker
	once    sync.Once
}

func (c *trackedConnection) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { c.tracker.remove(c) })
	return err
}

type connectionTracker struct {
	mu          sync.Mutex
	connections map[*trackedConnection]struct{}
	changed     chan struct{}
}

func newConnectionTracker() *connectionTracker {
	return &connectionTracker{
		connections: map[*trackedConnection]struct{}{},
		changed:     make(chan struct{}),
	}
}

func (t *connectionTracker) track(connection net.Conn) net.Conn {
	tracked := &trackedConnection{Conn: connection, tracker: t}
	t.mu.Lock()
	t.connections[tracked] = struct{}{}
	t.notifyLocked()
	t.mu.Unlock()
	return tracked
}

func (t *connectionTracker) remove(connection *trackedConnection) {
	t.mu.Lock()
	delete(t.connections, connection)
	t.notifyLocked()
	t.mu.Unlock()
}

func (t *connectionTracker) wait(ctx context.Context) error {
	for {
		t.mu.Lock()
		if len(t.connections) == 0 {
			t.mu.Unlock()
			return nil
		}
		changed := t.changed
		t.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func (t *connectionTracker) closeAll() {
	t.mu.Lock()
	connections := make([]*trackedConnection, 0, len(t.connections))
	for connection := range t.connections {
		connections = append(connections, connection)
	}
	t.mu.Unlock()
	for _, connection := range connections {
		_ = connection.Close()
	}
}

func (t *connectionTracker) notifyLocked() {
	close(t.changed)
	t.changed = make(chan struct{})
}

// Done closes when the DSH child exits for any reason.
func (i *Instance) Done() <-chan struct{} {
	return i.processState.done
}

// Wait returns the DSH child result. It is intended for unexpected-exit
// monitoring; callers performing a managed shutdown should use Close.
func (i *Instance) Wait() error {
	return i.processState.result()
}

func newProxy(target *url.URL, token string) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalHost := request.Host
		director(request)
		request.Host = originalHost
		request.Header.Del("Authorization")
		request.Header.Del("Proxy-Authorization")
		for _, name := range []string{
			"X-Authenticated-User",
			"X-Cell-Principal",
			"X-Forwarded-User",
			"X-Remote-User",
			"X-Dsh-Oidc-Token",
		} {
			request.Header.Del(name)
		}
		stripEnvoyOAuthCookies(request.Header)
		if request.Method == http.MethodGet && request.URL.Path == "/" && request.URL.RawQuery == "" && !hasDSHBrowserCookie(request) {
			query := request.URL.Query()
			query.Set("token", token)
			request.URL.RawQuery = query.Encode()
		}
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		cookies := response.Header.Values("Set-Cookie")
		if len(cookies) == 0 {
			return nil
		}
		response.Header.Del("Set-Cookie")
		for _, cookie := range cookies {
			if name, ok := responseCookieName(cookie); !ok || accesscontract.IsEnvoyOAuthCookie(name) {
				continue
			}
			cookie = normalizeDSHBrowserCookie(cookie)
			if !hasCookieAttribute(cookie, "Secure") {
				cookie += "; Secure"
			}
			response.Header.Add("Set-Cookie", cookie)
		}
		return nil
	}
	return proxy
}

func responseCookieName(raw string) (string, bool) {
	pair, _, _ := strings.Cut(raw, ";")
	name, _, found := strings.Cut(strings.TrimSpace(pair), "=")
	name = strings.TrimSpace(name)
	return name, found && name != ""
}

// DSH uses SameSite=Strict for its authority-bound browser cookie. That cookie
// is not returned during the top-level redirect chain from an external OIDC
// provider, leaving the browser in DSH's token-exchange redirect loop. Lax
// retains cross-site subrequest protection while allowing this safe top-level
// navigation to complete.
func normalizeDSHBrowserCookie(raw string) string {
	parts := strings.Split(raw, ";")
	if len(parts) == 0 {
		return raw
	}
	name, _, found := strings.Cut(strings.TrimSpace(parts[0]), "=")
	if !found || !strings.HasPrefix(name, "dsh-auth-") {
		return raw
	}

	normalized := make([]string, 0, len(parts)+1)
	normalized = append(normalized, strings.TrimSpace(parts[0]))
	for _, part := range parts[1:] {
		attribute := strings.TrimSpace(part)
		attributeName, _, _ := strings.Cut(attribute, "=")
		if strings.EqualFold(attributeName, "SameSite") {
			continue
		}
		normalized = append(normalized, attribute)
	}
	normalized = append(normalized, "SameSite=Lax")
	return strings.Join(normalized, "; ")
}

// stripEnvoyOAuthCookies removes every credential cookie owned by the pinned
// Envoy OAuth2 filter while preserving DSH and unrelated application cookies.
// Malformed cookie segments are dropped rather than forwarded across the Cell
// boundary. Values are never returned or logged by this function.
func stripEnvoyOAuthCookies(header http.Header) {
	lines := header.Values("Cookie")
	header.Del("Cookie")
	kept := make([]string, 0)
	for _, line := range lines {
		for _, segment := range strings.Split(line, ";") {
			segment = strings.TrimSpace(segment)
			name, _, valid := strings.Cut(segment, "=")
			if !valid || strings.TrimSpace(name) == "" || accesscontract.IsEnvoyOAuthCookie(strings.TrimSpace(name)) {
				continue
			}
			kept = append(kept, segment)
		}
	}
	if len(kept) != 0 {
		header.Set("Cookie", strings.Join(kept, "; "))
	}
}

func hasDSHBrowserCookie(request *http.Request) bool {
	for _, cookie := range request.Cookies() {
		if strings.HasPrefix(cookie.Name, "dsh-auth-") {
			return true
		}
	}
	return false
}

func validateAuthority(authority string) error {
	if strings.TrimSpace(authority) == "" {
		return errors.New("launcher: public authority is required")
	}
	parsed, err := url.Parse("http://" + authority)
	if err != nil || parsed.Host != authority || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("launcher: invalid public authority %q", authority)
	}
	return nil
}

func parseReadyURL(raw string) (readiness, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Path != "/" || parsed.RawPath != "" || parsed.Fragment != "" {
		return readiness{}, errors.New("launcher: DSH announced an invalid loopback URL")
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || host != "127.0.0.1" || portNumber < 1 || portNumber > 65535 {
		return readiness{}, errors.New("launcher: DSH announced an invalid loopback URL")
	}
	query := parsed.Query()
	tokens := query["token"]
	if len(query) != 1 || len(tokens) != 1 || !tokenValue.MatchString(tokens[0]) {
		return readiness{}, errors.New("launcher: DSH announced an invalid launch token")
	}
	token := tokens[0]
	parsed.RawQuery = ""
	return readiness{target: parsed, token: token}, nil
}

func redactTokens(line string) string {
	return tokenPattern.ReplaceAllString(line, `${1}<redacted>`)
}

func hasCookieAttribute(cookie, attribute string) bool {
	for _, segment := range strings.Split(cookie, ";") {
		if strings.EqualFold(strings.TrimSpace(segment), attribute) {
			return true
		}
	}
	return false
}

func terminate(process *os.Process, timeout time.Duration, state *processState) error {
	if process == nil {
		return nil
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("launcher: signal DSH: %w", err)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-state.done:
		return normalizeExit(state.result())
	case <-timer.C:
		if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("launcher: kill DSH: %w", err)
		}
		<-state.done
		err := state.result()
		if err == nil {
			return errors.New("launcher: DSH exceeded shutdown timeout")
		}
		return fmt.Errorf("launcher: DSH exceeded shutdown timeout: %w", err)
	}
}

func (s *processState) complete(wait func() error) {
	err := wait()
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
	close(s.done)
}

func (s *processState) result() error {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func normalizeExit(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() && status.Signal() == syscall.SIGTERM {
			return nil
		}
	}
	return err
}

func exitError(err error) error {
	if err == nil {
		return errors.New("DSH exited without an error")
	}
	return err
}
