// Command controller is the control plane (⑥): it orchestrates the session
// lifecycle across admission, scheduling, provisioning, and the isolation
// boundary. Today it is a skeleton with no watch loop.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/controlplane"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime/kubernetes"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/scheduling"
)

func main() {
	// TODO(M1): replace the in-memory backend with a real cluster client and
	// run a controller-runtime reconciler over RuntimeSession objects.
	rt := kubernetes.New()
	sched := &scheduling.FirstFit{Runtimes: []string{"runtime-default"}}
	_ = controlplane.New(rt, sched)

	log.Println("controller ready (skeleton: no watch loop yet)")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("controller stopped")
}
