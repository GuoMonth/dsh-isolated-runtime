package launcher

import (
	"context"
	"errors"
	"net"
	"testing"
	"testing/synctest"
	"time"
)

func TestConnectionDrainWaitsForCloseOrDeadline(t *testing.T) {
	for _, expire := range []bool{false, true} {
		name := "last connection closes"
		if expire {
			name = "drain deadline"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				tracker := newConnectionTracker()
				connection, peer := net.Pipe()
				defer func() { _ = peer.Close() }()
				tracked := tracker.track(connection)
				defer tracker.closeAll()
				ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
				defer cancel()
				result := make(chan error, 1)
				go func() { result <- tracker.wait(ctx) }()
				synctest.Sleep(time.Minute - time.Nanosecond)
				select {
				case err := <-result:
					t.Fatalf("drain returned while a connection is open: %v", err)
				default:
				}
				if expire {
					synctest.Sleep(time.Nanosecond)
					if err := <-result; !errors.Is(err, context.DeadlineExceeded) {
						t.Fatalf("expired drain: %v", err)
					}
				} else {
					_ = tracked.Close()
					synctest.Wait()
					if err := <-result; err != nil {
						t.Fatalf("closed drain: %v", err)
					}
				}
			})
		})
	}
}
