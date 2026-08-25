// Command controller is the self-hosted M1 business control plane. Kubernetes
// supplies infrastructure primitives; this process owns Runtime business state,
// reconciliation, and the minimal Admin UI/API.
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
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/controlplane"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime/kubernetes"
)

func main() {
	log.Printf("controller %s", version.String())
	namespace := envOr("DSH_RUNTIME_NAMESPACE", "dsh-isolated-system")
	listenAddr := envOr("DSH_LISTEN_ADDR", "127.0.0.1:8080")

	backend, err := kubernetes.NewInCluster(namespace)
	if err != nil {
		log.Fatalf("controller: initialize Kubernetes backend: %v", err)
	}
	admin, err := controlplane.NewAdminServer(backend)
	if err != nil {
		log.Fatalf("controller: initialize admin server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go reconcileLoop(ctx, backend)

	srv := &http.Server{Addr: listenAddr, Handler: admin.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("control plane listening on %s (runtime namespace %s)", listenAddr, namespace)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("controller: %v", err)
		}
	}()

	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	log.Println("controller stopped")
}

func reconcileLoop(ctx context.Context, backend *kubernetes.Backend) {
	run := func() {
		child, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := backend.ReconcileAll(child); err != nil && ctx.Err() == nil {
			log.Printf("reconcile: %v", err)
		}
	}
	run()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
