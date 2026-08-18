// Command controller is the control plane (⑥): it orchestrates the session
// lifecycle across runtime allocation, provisioning, persistence, and the
// isolation boundary. Today it is a skeleton with no watch loop.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/GuoMonth/dsh-isolated-runtime/internal/version"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/controlplane"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime/kubernetes"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/scheduling"
)

func main() {
	log.Printf("controller %s", version.String())
	// TODO(M1): replace the in-memory backend with a real cluster client and
	// reconcile RuntimeSession objects into tenant-owned Runtime Pods.
	rt := kubernetes.New()
	allocator := &scheduling.FirstFit{}
	_ = controlplane.New(rt, allocator)

	log.Println("controller ready (skeleton: no watch loop yet)")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("controller stopped")
}
