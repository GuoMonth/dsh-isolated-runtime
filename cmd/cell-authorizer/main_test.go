package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestShutdownHTTPClosesActiveRequests(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		for range 3 {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			started := make(chan struct{})
			finished := make(chan struct{})
			server := &http.Server{Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				close(started)
				<-r.Context().Done()
				close(finished)
			})}
			t.Cleanup(func() { _ = server.Close() })
			serveDone := make(chan error, 1)
			go func() { serveDone <- server.Serve(listener) }()
			transport := http.DefaultTransport.(*http.Transport).Clone()
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			t.Cleanup(transport.CloseIdleConnections)
			requestDone := make(chan struct{})
			go func() {
				defer close(requestDone)
				request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+listener.Addr().String(), nil)
				response, err := client.Do(request)
				if err == nil {
					_ = response.Body.Close()
				}
			}()
			await(t, started)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			if canceled {
				cancel()
			}
			err = shutdownHTTP(ctx, server)
			cancel()
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("shutdown error = %v", err)
			}
			await(t, finished)
			await(t, requestDone)
			if err := <-serveDone; !errors.Is(err, http.ErrServerClosed) {
				t.Fatalf("serve error = %v", err)
			}
		}
	}
}

func await(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server lifecycle did not finish")
	}
}
