package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
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
		discoveryv1.AddToScheme,
		storagev1.AddToScheme,
		dshv1alpha1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	kube, err := client.New(configuration, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "envtest-tenant"}}
	if err := kube.Create(ctx, namespace); err != nil {
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
}
