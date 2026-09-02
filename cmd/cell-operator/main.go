package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	cellcontroller "github.com/GuoMonth/dsh-isolated-runtime/internal/controller"
)

func main() {
	var systemNamespace string
	var sandboxedRuntimeClass string
	flag.StringVar(&systemNamespace, "system-namespace", "", "namespace whose labelled access Pods may reach Cells (defaults to POD_NAMESPACE)")
	flag.StringVar(&sandboxedRuntimeClass, "sandboxed-runtime-class", "", "cluster-owned RuntimeClass used for sandboxed Cells")
	flag.Parse()
	if strings.TrimSpace(systemNamespace) == "" {
		systemNamespace = os.Getenv("POD_NAMESPACE")
	}
	if strings.TrimSpace(systemNamespace) == "" {
		fatal(fmt.Errorf("system namespace is required through --system-namespace or POD_NAMESPACE"))
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		fatal(err)
	}
	if err := dshv1alpha1.AddToScheme(scheme); err != nil {
		fatal(err)
	}

	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))
	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: ":8081",
		LeaderElection:         false,
	})
	if err != nil {
		fatal(err)
	}
	reconciler := &cellcontroller.CellReconciler{
		Client:                manager.GetClient(),
		Scheme:                manager.GetScheme(),
		SystemNamespace:       systemNamespace,
		SandboxedRuntimeClass: sandboxedRuntimeClass,
	}
	if err := reconciler.SetupWithManager(manager); err != nil {
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
