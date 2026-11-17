package leakcheck

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	VerifyTestMain(m)
}

func TestLeakGateRejectsOrphans(t *testing.T) {
	for _, test := range []struct{ mode, diagnostic string }{
		{"unreachable", "goroutineleak profile detected"},
		{"reachable", "goleak: Errors on successful test run"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestLeakFixture$")
			command.Env = append(os.Environ(), "DSH_LEAKCHECK_FIXTURE="+test.mode)
			output, err := command.CombinedOutput()
			if exit, ok := errors.AsType[*exec.ExitError](err); !ok || exit.ExitCode() != 1 {
				t.Fatalf("leak fixture must fail with exit 1: %v\n%s", err, output)
			}
			if !strings.Contains(string(output), test.diagnostic) {
				t.Fatalf("missing %s diagnostic:\n%s", test.mode, output)
			}
		})
	}
}

var reachable = make(chan struct{})

// These leaks exist only in subprocesses, so the negative controls cannot
// contaminate the parent test run or require detector ignore lists.
func TestLeakFixture(t *testing.T) {
	started := make(chan struct{})
	switch os.Getenv("DSH_LEAKCHECK_FIXTURE") {
	case "unreachable":
		go func() {
			close(started)
			<-make(chan struct{})
		}()
	case "reachable":
		go func() {
			close(started)
			<-reachable
		}()
	default:
		t.Skip("subprocess fixture")
	}
	<-started
}
