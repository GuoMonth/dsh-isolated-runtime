package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	cellcontroller "github.com/GuoMonth/dsh-isolated-runtime/internal/controller"
)

func main() {
	var systemNamespace string
	var sandboxedRuntimeClass string
	var gatewayName string
	var gatewayNamespace string
	var gatewaySection string
	var baseDomain string
	var externalHTTPSPort int
	var enableSnapshots bool
	var writerStopTimeout time.Duration
	var snapshotTimeout time.Duration
	var metricsBindAddress string
	var cellConcurrency int
	var snapshotConcurrency int
	flag.StringVar(&systemNamespace, "system-namespace", "", "namespace whose labelled access Pods may reach Cells (defaults to POD_NAMESPACE)")
	flag.StringVar(&sandboxedRuntimeClass, "sandboxed-runtime-class", "", "cluster-owned RuntimeClass used for sandboxed Cells")
	flag.StringVar(&gatewayName, "gateway-name", "", "Gateway used for derived Cell HTTPRoutes; empty disables public routing")
	flag.StringVar(&gatewayNamespace, "gateway-namespace", "dsh-system", "namespace containing the public Gateway")
	flag.StringVar(&gatewaySection, "gateway-section-name", "https", "Gateway HTTPS listener section name")
	flag.StringVar(&baseDomain, "base-domain", "", "base domain used for cell-<UID> hostnames")
	flag.IntVar(&externalHTTPSPort, "external-https-port", 443, "external HTTPS authority port")
	flag.BoolVar(&enableSnapshots, "enable-snapshots", false, "enable CellSnapshot reconciliation and require CSI snapshot APIs")
	flag.DurationVar(&writerStopTimeout, "writer-stop-timeout", 2*time.Minute, "maximum time for StatefulSet scale-down and managed Pod removal")
	flag.DurationVar(&snapshotTimeout, "snapshot-timeout", 30*time.Minute, "maximum time for one CSI VolumeSnapshot to become ready")
	flag.StringVar(&metricsBindAddress, "metrics-bind-address", "0", "Prometheus listen address; 0 disables metrics")
	flag.IntVar(&cellConcurrency, "cell-concurrency", 1, "maximum concurrent Cell reconciles")
	flag.IntVar(&snapshotConcurrency, "snapshot-concurrency", 1, "maximum concurrent CellSnapshot reconciles")
	flag.Parse()
	if strings.TrimSpace(systemNamespace) == "" {
		systemNamespace = os.Getenv("POD_NAMESPACE")
	}
	if strings.TrimSpace(systemNamespace) == "" {
		fatal(fmt.Errorf("system namespace is required through --system-namespace or POD_NAMESPACE"))
	}
	metricsBindAddress = strings.TrimSpace(metricsBindAddress)
	if strings.TrimSpace(metricsBindAddress) == "" {
		fatal(fmt.Errorf("--metrics-bind-address must be 0 or a non-empty listen address"))
	}
	if cellConcurrency < 1 || snapshotConcurrency < 1 {
		fatal(fmt.Errorf("controller concurrency must be at least 1"))
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		fatal(err)
	}
	if err := dshv1alpha1.AddToScheme(scheme); err != nil {
		fatal(err)
	}
	if err := gatewayv1.Install(scheme); err != nil {
		fatal(err)
	}
	if err := volumesnapshotv1.AddToScheme(scheme); err != nil {
		fatal(err)
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))
	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsBindAddress},
		HealthProbeBindAddress: ":8081",
		LeaderElection:         false,
	})
	if err != nil {
		fatal(err)
	}
	reconciler := &cellcontroller.CellReconciler{
		Client:                manager.GetClient(),
		APIReader:             manager.GetAPIReader(),
		Scheme:                manager.GetScheme(),
		SystemNamespace:       systemNamespace,
		SandboxedRuntimeClass: sandboxedRuntimeClass,
		RouteConfig: cellcontroller.RouteConfig{
			GatewayName:       gatewayName,
			GatewayNamespace:  gatewayNamespace,
			GatewaySection:    gatewaySection,
			BaseDomain:        baseDomain,
			ExternalHTTPSPort: externalHTTPSPort,
		},
		Recorder:                manager.GetEventRecorderFor("cell-operator"),
		SnapshotEnabled:         enableSnapshots,
		MaxConcurrentReconciles: cellConcurrency,
	}
	if err := reconciler.SetupWithManager(manager); err != nil {
		fatal(err)
	}
	snapshotReconciler := &cellcontroller.CellSnapshotReconciler{
		Client:    manager.GetClient(),
		APIReader: manager.GetAPIReader(),
		Scheme:    manager.GetScheme(),
		Config: cellcontroller.SnapshotConfig{
			Enabled:           enableSnapshots,
			WriterStopTimeout: writerStopTimeout,
			SnapshotTimeout:   snapshotTimeout,
		},
		MaxConcurrentReconciles: snapshotConcurrency,
	}
	if err := snapshotReconciler.SetupWithManager(manager); err != nil {
		fatal(err)
	}
	if err := manager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		fatal(err)
	}
	if err := manager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		fatal(err)
	}
	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "cell-operator: %v\n", err)
	os.Exit(1)
}
