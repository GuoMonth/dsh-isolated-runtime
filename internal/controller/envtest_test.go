package controller

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/accesscontract"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

func TestEnvtestReconcileAndAdmission(t *testing.T) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("KUBEBUILDER_ASSETS is not configured")
	}
	crdPath, err := filepath.Abs("../../config/crd/bases")
	if err != nil {
		t.Fatal(err)
	}
	testEnvironment := &envtest.Environment{
		CRDDirectoryPaths:     []string{crdPath},
		ErrorIfCRDPathMissing: true,
	}
	configuration, err := testEnvironment.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := testEnvironment.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	})

	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		appsv1.AddToScheme,
		networkingv1.AddToScheme,
		rbacv1.AddToScheme,
		discoveryv1.AddToScheme,
		storagev1.AddToScheme,
		volumesnapshotv1.AddToScheme,
		dshv1alpha1.AddToScheme,
		gatewayv1.Install,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	kube, err := client.New(configuration, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := ctrl.NewManager(configuration, ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	enabled := &CellReconciler{
		Client: manager.GetClient(), Scheme: scheme, SystemNamespace: "dsh-system",
		RouteConfig: accesscontract.Config{GatewayName: "dsh", GatewayNamespace: "dsh-system", GatewaySection: "https", BaseDomain: "cells.test"},
	}
	if err := enabled.SetupWithManager(manager); err == nil || !strings.Contains(err.Error(), "HTTPRoute CRD") {
		t.Fatalf("enabled routing without Gateway API CRD error = %v", err)
	}
	disabled := &CellReconciler{Client: manager.GetClient(), Scheme: scheme, SystemNamespace: "dsh-system"}
	if err := disabled.SetupWithManager(manager); err != nil {
		t.Fatalf("Phase 1 mode depended on Gateway API CRD: %v", err)
	}
	enabledSnapshots := &CellSnapshotReconciler{
		Client: manager.GetClient(), Scheme: scheme,
		Config: SnapshotConfig{Enabled: true, QuiesceTimeout: 2 * time.Minute, SnapshotTimeout: 30 * time.Minute},
	}
	if err := enabledSnapshots.SetupWithManager(manager); err == nil || !strings.Contains(err.Error(), "VolumeSnapshot") {
		t.Fatalf("enabled snapshots without CSI snapshot CRDs error = %v", err)
	}
	disabledSnapshots := &CellSnapshotReconciler{Client: manager.GetClient(), Scheme: scheme, Config: SnapshotConfig{Enabled: false}}
	if err := disabledSnapshots.SetupWithManager(manager); err != nil {
		t.Fatalf("snapshot-disabled mode depended on CSI snapshot CRDs: %v", err)
	}
	ctx := context.Background()
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "envtest-tenant"}}
	if err := kube.Create(ctx, namespace); err != nil {
		t.Fatal(err)
	}
	lockProbe := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": dshv1alpha1.GroupVersion.String(),
		"kind":       "Cell",
		"metadata": map[string]any{
			"name": "lock-patch-probe", "namespace": namespace.Name,
		},
		"spec": map[string]any{
			"image": "example.test/cell@" + testDigest,
			"storage": map[string]any{
				"size": "1Gi", "retentionPolicy": string(dshv1alpha1.RetentionRetain),
			},
		},
	}}
	lockProbe.SetGroupVersionKind(dshv1alpha1.GroupVersion.WithKind("Cell"))
	if err := kube.Create(ctx, lockProbe); err != nil {
		t.Fatal(err)
	}
	initialGeneration := lockProbe.GetGeneration()
	var typedProbe dshv1alpha1.Cell
	probeKey := client.ObjectKeyFromObject(lockProbe)
	if err := kube.Get(ctx, probeKey, &typedProbe); err != nil {
		t.Fatal(err)
	}
	lockReconciler := &CellSnapshotReconciler{Client: kube}
	if err := lockReconciler.patchCellOperation(ctx, &typedProbe, "snapshot-operation"); err != nil {
		t.Fatal(err)
	}
	if err := kube.Get(ctx, probeKey, lockProbe); err != nil {
		t.Fatal(err)
	}
	if lockProbe.GetGeneration() != initialGeneration {
		t.Fatalf("metadata lock changed Cell generation from %d to %d", initialGeneration, lockProbe.GetGeneration())
	}
	if _, found, err := unstructured.NestedFieldNoCopy(lockProbe.Object, "spec", "resources"); err != nil || found {
		t.Fatalf("metadata lock materialized spec.resources: found=%v err=%v", found, err)
	}
	if err := kube.Get(ctx, probeKey, &typedProbe); err != nil {
		t.Fatal(err)
	}
	if err := lockReconciler.patchCellOperation(ctx, &typedProbe, ""); err != nil {
		t.Fatal(err)
	}
	if err := kube.Delete(ctx, lockProbe); err != nil {
		t.Fatal(err)
	}
	allowExpansion := true
	storageClass := &storagev1.StorageClass{
		ObjectMeta:           metav1.ObjectMeta{Name: "envtest-expandable"},
		Provisioner:          "example.test/envtest",
		AllowVolumeExpansion: &allowExpansion,
	}
	if err := kube.Create(ctx, storageClass); err != nil {
		t.Fatal(err)
	}
	cell := &dshv1alpha1.Cell{
		TypeMeta:   metav1.TypeMeta{APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "Cell"},
		ObjectMeta: metav1.ObjectMeta{Name: "vertical", Namespace: namespace.Name},
		Spec: dshv1alpha1.CellSpec{
			Image: "example.test/cell@" + testDigest,
			Storage: dshv1alpha1.CellStorageSpec{
				Size:             resource.MustParse("1Gi"),
				StorageClassName: &storageClass.Name,
				RetentionPolicy:  dshv1alpha1.RetentionRetain,
			},
		},
	}
	if err := kube.Create(ctx, cell); err != nil {
		t.Fatal(err)
	}
	reconciler := &CellReconciler{Client: kube, Scheme: scheme, SystemNamespace: "dsh-system"}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: cell.Namespace, Name: cell.Name}}
	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := kube.Get(ctx, client.ObjectKeyFromObject(cell), cell); err != nil {
		t.Fatal(err)
	}
	names := cellcontract.ResourceNames(string(cell.UID))
	for _, expected := range []struct {
		name   string
		object client.Object
	}{
		{names.DataPVC, &corev1.PersistentVolumeClaim{}},
		{names.PrivatePVC, &corev1.PersistentVolumeClaim{}},
		{names.Base, &corev1.ServiceAccount{}},
		{names.Base, &corev1.Service{}},
		{names.Headless, &corev1.Service{}},
		{names.Base, &appsv1.StatefulSet{}},
		{names.Base, &networkingv1.NetworkPolicy{}},
	} {
		if err := kube.Get(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: expected.name}, expected.object); err != nil {
			t.Fatalf("get %T/%s: %v", expected.object, expected.name, err)
		}
	}
	if cell.Status.ObservedGeneration != cell.Generation || len(cell.Status.Conditions) != 4 {
		t.Fatalf("status did not represent current generation: %#v", cell.Status)
	}
	var data corev1.PersistentVolumeClaim
	if err := kube.Get(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.DataPVC}, &data); err != nil {
		t.Fatal(err)
	}
	data.Status.Phase = corev1.ClaimBound
	if err := kube.Status().Update(ctx, &data); err != nil {
		t.Fatal(err)
	}

	original := cell.DeepCopy()
	cell.Spec.Storage.RetentionPolicy = dshv1alpha1.RetentionDelete
	if err := kube.Update(ctx, cell); !apierrors.IsInvalid(err) {
		t.Fatalf("retention mutation error = %v, want Invalid", err)
	}
	cell = original
	cell.Spec.Storage.Size = resource.MustParse("2Gi")
	if err := kube.Update(ctx, cell); err != nil {
		t.Fatalf("grow Cell: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := kube.Get(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.DataPVC}, &data); err != nil {
		t.Fatal(err)
	}
	if got := data.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatalf("PVC request = %s", got.String())
	}
	if err := kube.Get(ctx, client.ObjectKeyFromObject(cell), cell); err != nil {
		t.Fatal(err)
	}
	cell.Spec.Storage.Size = resource.MustParse("1Gi")
	if err := kube.Update(ctx, cell); !apierrors.IsInvalid(err) {
		t.Fatalf("shrink error = %v, want Invalid", err)
	}

	if err := kube.Get(ctx, request.NamespacedName, cell); err != nil {
		t.Fatal(err)
	}
	cell.Spec.Storage.RestoreFrom = &dshv1alpha1.LocalCellSnapshotReference{Name: "late-restore"}
	if err := kube.Update(ctx, cell); !apierrors.IsInvalid(err) {
		t.Fatalf("late restoreFrom error = %v, want Invalid", err)
	}

	snapshot := &dshv1alpha1.CellSnapshot{
		TypeMeta:   metav1.TypeMeta{APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "CellSnapshot"},
		ObjectMeta: metav1.ObjectMeta{Name: "immutable", Namespace: namespace.Name},
		Spec: dshv1alpha1.CellSnapshotSpec{
			CellRef: dshv1alpha1.LocalCellReference{Name: cell.Name}, VolumeSnapshotClassName: "snapshot-class",
		},
	}
	if err := kube.Create(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Spec.VolumeSnapshotClassName = "other-class"
	if err := kube.Update(ctx, snapshot); !apierrors.IsInvalid(err) {
		t.Fatalf("CellSnapshot spec mutation error = %v, want Invalid", err)
	}
}
