// Package leakcheck combines the runtime's permanently blocked goroutine
// profile with goleak's checks for goroutines that outlive their owner.
// It belongs in TestMain, after all test cleanups have run.
package leakcheck

import (
	"bytes"
	"fmt"
	"os"
	"runtime/pprof"
	"testing"

	"go.uber.org/goleak"
)

// VerifyTestMain runs tests and exits with a failure if either leak detector
// finds an orphaned goroutine. Profiles stay in test diagnostics; no HTTP
// profiling endpoint is installed in the environment.
func VerifyTestMain(m *testing.M) {
	goleak.VerifyTestMain(profiledTests{m})
}

type profiledTests struct{ *testing.M }

func (m profiledTests) Run() int {
	status := m.M.Run()
	var stacks bytes.Buffer
	profile := pprof.Lookup("goroutineleak")
	// WriteTo performs the special GC that detects leaks; Count alone does not.
	if err := profile.WriteTo(&stacks, 1); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "goroutineleak profile: %v\n", err)
		return 1
	}
	if profile.Count() != 0 {
		_, _ = fmt.Fprintf(os.Stderr, "goroutineleak profile detected permanently blocked goroutines:\n%s", stacks.Bytes())
		return 1
	}
	return status
}
