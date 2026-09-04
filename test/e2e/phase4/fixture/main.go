package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	failing := os.Getenv("FLEET_FIXTURE_FAIL") == "true"
	readyURL := os.Getenv("FLEET_FIXTURE_READY_URL")
	readyClient := &http.Client{Timeout: time.Second}
	proxy := &http.Server{Addr: ":8080", Handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}), ReadHeaderTimeout: 5 * time.Second}
	managementMux := http.NewServeMux()
	managementMux.HandleFunc("GET /livez", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	})
	managementMux.HandleFunc("GET /readyz", func(response http.ResponseWriter, _ *http.Request) {
		if failing {
			http.Error(response, "fixture not ready", http.StatusServiceUnavailable)
			return
		}
		if readyURL != "" {
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, readyURL, nil)
			if err != nil {
				http.Error(response, "invalid readiness dependency", http.StatusServiceUnavailable)
				return
			}
			dependency, err := readyClient.Do(request)
			if err != nil {
				http.Error(response, "readiness dependency unavailable", http.StatusServiceUnavailable)
				return
			}
			_ = dependency.Body.Close()
			if dependency.StatusCode < http.StatusOK || dependency.StatusCode >= http.StatusMultipleChoices {
				http.Error(response, "readiness dependency rejected", http.StatusServiceUnavailable)
				return
			}
		}
		response.WriteHeader(http.StatusOK)
	})
	management := &http.Server{Addr: ":8081", Handler: managementMux, ReadHeaderTimeout: 5 * time.Second}
	errorsCh := make(chan error, 2)
	go func() { errorsCh <- serve(proxy) }()
	go func() { errorsCh <- serve(management) }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-signals:
	case err := <-errorsCh:
		if err != nil {
			os.Exit(1)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = proxy.Shutdown(ctx)
	_ = management.Shutdown(ctx)
}

func serve(server *http.Server) error {
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
