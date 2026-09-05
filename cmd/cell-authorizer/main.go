package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/accesscontract"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/authorizer"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "cell-authorizer: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var issuer, clientID, groupsClaim string
	var grpcAddress, callbackAddress, healthAddress, metricsAddress string
	var route accesscontract.Config
	flag.StringVar(&issuer, "oidc-issuer", "", "exact OIDC issuer URL")
	flag.StringVar(&clientID, "oidc-client-id", "", "OIDC client ID accepted in ID token audience")
	flag.StringVar(&groupsClaim, "oidc-groups-claim", "groups", "optional string-array OIDC groups claim")
	flag.StringVar(&route.GatewayName, "gateway-name", "", "Gateway containing the protected Cell listener")
	flag.StringVar(&route.GatewayNamespace, "gateway-namespace", "dsh-system", "Gateway namespace")
	flag.StringVar(&route.GatewaySection, "gateway-section-name", "https", "Gateway listener section name")
	flag.StringVar(&route.BaseDomain, "base-domain", "", "base domain used for derived Cell hosts")
	flag.IntVar(&route.ExternalHTTPSPort, "external-https-port", 443, "external HTTPS authority port")
	flag.StringVar(&grpcAddress, "grpc-address", ":9001", "Envoy ext_authz gRPC listen address")
	flag.StringVar(&callbackAddress, "callback-address", ":8080", "fixed 404 OAuth callback backend listen address")
	flag.StringVar(&healthAddress, "health-address", ":8081", "health probe listen address")
	flag.StringVar(&metricsAddress, "metrics-bind-address", "0", "Prometheus listen address; 0 disables metrics")
	flag.Parse()
	if issuer == "" || clientID == "" {
		return errors.New("--oidc-issuer and --oidc-client-id are required")
	}
	if err := route.Validate(); err != nil || !route.Enabled() {
		return errors.Join(errors.New("valid Gateway routing configuration is required"), err)
	}
	metricsAddress = strings.TrimSpace(metricsAddress)
	if strings.TrimSpace(metricsAddress) == "" {
		return errors.New("--metrics-bind-address must be 0 or a non-empty listen address")
	}

	verifier, err := authorizer.NewOIDCVerifier(context.Background(), issuer, clientID, groupsClaim)
	if err != nil {
		return err
	}
	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	// One native DSH page issues a burst of requests, each requiring fresh
	// Cell/Route reads and a SAR inside the Gateway's two-second deadline.
	// client-go's controller defaults (5 QPS / 10 burst) otherwise turn an
	// ordinary page load into fail-closed 503s even on a healthy API server.
	clusterConfig.QPS = 100
	clusterConfig.Burst = 200
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, dshv1alpha1.AddToScheme, gatewayv1.Install} {
		if err := add(scheme); err != nil {
			return err
		}
	}
	reader, err := client.New(clusterConfig, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("create Kubernetes reader: %w", err)
	}
	kube, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return fmt.Errorf("create Kubernetes authorization client: %w", err)
	}
	metricsRegistry, decisions := authorizer.NewMetricsRegistry()
	service := &authorizer.Server{
		Reader:      reader,
		Reviewer:    kube.AuthorizationV1().SubjectAccessReviews(),
		Verifier:    verifier,
		Issuer:      issuer,
		RouteConfig: route,
		Decisions:   decisions,
	}
	if err := service.Validate(); err != nil {
		return err
	}

	grpcListener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		return fmt.Errorf("listen for ext_authz: %w", err)
	}
	defer func() { _ = grpcListener.Close() }()
	grpcServer := grpc.NewServer()
	authv3.RegisterAuthorizationServer(grpcServer, service)

	var ready atomic.Bool
	callbackServer := &http.Server{Addr: callbackAddress, Handler: http.NotFoundHandler(), ReadHeaderTimeout: 5 * time.Second}
	healthServer := &http.Server{Addr: healthAddress, Handler: healthHandler(&ready), ReadHeaderTimeout: 5 * time.Second}
	errorsCh := make(chan error, 4)
	go func() { errorsCh <- grpcServer.Serve(grpcListener) }()
	go func() { errorsCh <- serveHTTP(callbackServer) }()
	go func() { errorsCh <- serveHTTP(healthServer) }()
	var metricsServer *http.Server
	if metricsAddress != "0" {
		metricsServer = &http.Server{
			Addr:              metricsAddress,
			Handler:           promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{}),
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() { errorsCh <- serveHTTP(metricsServer) }()
	}
	ready.Store(true)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	var runErr error
	select {
	case <-signals:
	case runErr = <-errorsCh:
	}
	ready.Store(false)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	grpcDone := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcDone)
	}()
	select {
	case <-grpcDone:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
	var metricsErr error
	if metricsServer != nil {
		metricsErr = metricsServer.Shutdown(shutdownCtx)
	}
	return errors.Join(runErr, callbackServer.Shutdown(shutdownCtx), healthServer.Shutdown(shutdownCtx), metricsErr)
}

func serveHTTP(server *http.Server) error {
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func healthHandler(ready *atomic.Bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /readyz", func(response http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			http.Error(response, "not ready", http.StatusServiceUnavailable)
			return
		}
		response.WriteHeader(http.StatusOK)
	})
	return mux
}
