package launcher

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestStartCancellationAndRepeatedClose(t *testing.T) {
	for range 3 {
		cfg := Config{
			DSHCommand:      []string{os.Args[0], "-test.run=TestDSHHelperProcess", "--"},
			Environment:     append(os.Environ(), "GO_WANT_DSH_HELPER=1"),
			PublicAuthority: "cell.example.test",
			ReadyTimeout:    5 * time.Second,
			// Race-instrumented helper processes sleep before exiting.
			ShutdownTimeout: 5 * time.Second,
		}
		instance, err := StartContext(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := instance.Close(ctx); err != nil {
			t.Fatal(err)
		}
		if err := instance.Close(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case <-instance.Done():
		default:
			t.Fatal("Close returned before child was reaped")
		}
		cfg.Environment = append(cfg.Environment, "DSH_HELPER_NO_READY=1")
		startup, stop := context.WithTimeout(t.Context(), 100*time.Millisecond)
		_, err = StartContext(startup, cfg)
		stop()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("startup error = %v", err)
		}
	}
}
