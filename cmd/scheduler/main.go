// Command scheduler is the runtime-allocation process (③).
//
// It decides reuse/create at the tenant runtime level; Kubernetes remains
// responsible for Pod-to-Node scheduling.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/GuoMonth/dsh-isolated-runtime/internal/version"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/scheduling"
)

func main() {
	log.Printf("scheduler %s", version.String())
	// TODO(M1): maintain tenant runtime inventory and run the allocation loop.
	allocator := scheduling.FirstFit{}
	log.Printf("runtime allocator ready: %d known runtime(s)", len(allocator.Runtimes))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("scheduler stopped")
}
