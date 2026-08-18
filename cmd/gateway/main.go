// Command gateway runs the trusted runtime admission/router boundary (④).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GuoMonth/dsh-isolated-runtime/internal/version"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/gateway"
)

func main() {
	log.Printf("gateway %s", version.String())
	// TODO(M2): wire a real transport Authenticator, Authorizer, SessionResolver,
	// and DSH HTTP/WebSocket/stream reverse proxy. nil dependencies deny all.
	admitter := gateway.NewAdmitter(nil, nil)
	server := gateway.NewServer(nil, admitter)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("gateway listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Println("gateway stopped")
}
