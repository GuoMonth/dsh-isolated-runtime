// Test-only TLS carrier for the real npm distribution. No platform authorization.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/GuoMonth/dsh-isolated-runtime/internal/dshcompat/launcher"
)

func main() {
	root := os.Getenv("RC2_DATA")
	for _, dir := range []string{"home", "dsh", "workspace"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			panic(err)
		}
	}
	var handler http.Handler
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
	authority := "environment.test:" + fmt.Sprint(server.Listener.Addr().(*net.TCPAddr).Port)
	instance, err := launcher.Start(launcher.Config{PatchFiles: []string{os.Getenv("RC2_PATCH")}, DSHCommand: []string{"node", "--expose-internals", os.Getenv("DSH_REAL_CLI")}, WorkingDir: filepath.Join(root, "workspace"), Environment: append(os.Environ(), "HOME="+filepath.Join(root, "home"), "DSH_HOME="+filepath.Join(root, "dsh"), "DSH_TELEMETRY_DISABLED=1"), PublicAuthority: authority, LogWriter: os.Stderr})
	if err != nil {
		panic(err)
	}
	target, _ := url.Parse(instance.URL)
	handler = httputil.NewSingleHostReverseProxy(target)
	server.StartTLS()
	defer server.Close()
	if err := os.WriteFile(os.Getenv("RC2_URL_FILE"), []byte("https://"+authority), 0600); err != nil {
		panic(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := instance.Close(shutdown); err != nil {
		panic(err)
	}
}
