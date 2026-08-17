// Command scheduler is the placement process (③). Today it is a skeleton that
// exposes the FirstFit seam; the real scheduler (cluster inventory + placement
// loop) lands at M1.
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
	// TODO(M1): discover cluster inventory and run a placement loop.
	sched := scheduling.FirstFit{Runtimes: []string{"runtime-default"}}
	log.Printf("scheduler ready: %d candidate runtime(s)", len(sched.Runtimes))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("scheduler stopped")
}
