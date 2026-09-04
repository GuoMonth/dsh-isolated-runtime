package controller

import (
	"context"
	"reflect"
	"testing"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

const (
	testUID    = types.UID("12345678-1234-1234-1234-123456789abc")
	testDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func TestReconcileBuildsNativeResourcesAndReadyStatus(t *testing.T) {
	t.Parallel()
	cell := testCell("main", dshv1alpha1.RetentionRetain)
	reconciler, kube := testReconciler(t, cell)
	reconcileCell(t, reconciler, cell)

	names := cellcontract.ResourceNames(string(cell.UID))
	data := get[*corev1.PersistentVolumeClaim](t, kube, cell.Namespace, names.DataPVC)
	private := get[*corev1.PersistentVolumeClaim](t, kube, cell.Namespace, names.PrivatePVC)
	if len(data.OwnerReferences) != 0 {
		t.Fatalf("Retain data PVC has owners: %#v", data.OwnerReferences)
	}
	if !metav1.IsControlledBy(private, cell) {
		t.Fatal("private PVC is not controlled by Cell")
	}
	if got := private.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse(cellcontract.PrivatePVCSize)) != 0 {
		t.Fatalf("private size = %s", got.String())
	}

	account := get[*corev1.ServiceAccount](t, kube, cell.Namespace, names.Base)
	if account.AutomountServiceAccountToken == nil || *account.AutomountServiceAccountToken {
		t.Fatal("workload ServiceAccount token is enabled")
	}
	headless := get[*corev1.Service](t, kube, cell.Namespace, names.Headless)
	access := get[*corev1.Service](t, kube, cell.Namespace, names.Base)
	if headless.Spec.ClusterIP != corev1.ClusterIPNone || access.Spec.ClusterIP == corev1.ClusterIPNone || access.Spec.Ports[0].Port != cellcontract.ProxyServicePort {
		t.Fatalf("unexpected Services: headless=%q access=%#v", headless.Spec.ClusterIP, access.Spec.Ports)
	}
	policy := get[*networkingv1.NetworkPolicy](t, kube, cell.Namespace, names.Base)
	if len(policy.Spec.PolicyTypes) != 1 || policy.Spec.PolicyTypes[0] != networkingv1.PolicyTypeIngress || len(policy.Spec.Egress) != 0 || len(policy.Spec.Ingress) != 1 {
		t.Fatalf("NetworkPolicy is not ingress-only: %#v", policy.Spec)
	}
	if policy.Spec.Ingress[0].Ports[0].Port.IntVal != cellcontract.ProxyContainerPort {
		t.Fatalf("NetworkPolicy access isolation drifted: %#v", policy.Spec.Ingress)
	}

	workload := get[*appsv1.StatefulSet](t, kube, cell.Namespace, names.Base)
	assertWorkloadContract(t, workload, names)
	markReady(t, kube, data, private, workload, access)
	reconcileCell(t, reconciler, cell)

	readyCell := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	if readyCell.Status.ObservedGeneration != cell.Generation || readyCell.Status.DSHVersion != cellcontract.DSHVersion || readyCell.Status.ImageDigest != testDigest {
		t.Fatalf("unexpected ready status: %#v", readyCell.Status)
	}
	for _, conditionType := range []string{
		dshv1alpha1.ConditionStorageReady,
		dshv1alpha1.ConditionWorkloadReady,
		dshv1alpha1.ConditionAccessReady,
		dshv1alpha1.ConditionReady,
	} {
		condition := meta.FindStatusCondition(readyCell.Status.Conditions, conditionType)
		if condition == nil || condition.Status != metav1.ConditionTrue || condition.ObservedGeneration != cell.Generation {
			t.Fatalf("condition %s is not current and true: %#v", conditionType, condition)
		}
	}
}

func TestReconcilePropagatesUpdatesWithoutSecretReads(t *testing.T) {
	t.Parallel()
	cell := testCell("updates", dshv1alpha1.RetentionDelete)
	reconciler, kube := testReconciler(t, cell)
	reconcileCell(t, reconciler, cell)

	current := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	current.Generation = 2
	current.Spec.Storage.Size = resource.MustParse("2Gi")
	current.Spec.CredentialsRef = &dshv1alpha1.LocalSecretReference{Name: "provider-v2"}
	current.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}
	if err := kube.Update(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	reconcileCell(t, reconciler, current)

	names := cellcontract.ResourceNames(string(cell.UID))
	data := get[*corev1.PersistentVolumeClaim](t, kube, cell.Namespace, names.DataPVC)
	if got := data.Spec.Resources.Requests[corev1.ResourceStorage]; got.Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatalf("data request = %s", got.String())
	}
	if !metav1.IsControlledBy(data, current) {
		t.Fatal("Delete data PVC is not controlled by Cell")
	}
	workload := get[*appsv1.StatefulSet](t, kube, cell.Namespace, names.Base)
	container := workload.Spec.Template.Spec.Containers[0]
	if len(container.EnvFrom) != 1 || container.EnvFrom[0].SecretRef == nil || container.EnvFrom[0].SecretRef.Name != "provider-v2" {
		t.Fatalf("credentialsRef was not projected: %#v", container.EnvFrom)
	}
	if got := container.Resources.Requests[corev1.ResourceCPU]; got.Cmp(resource.MustParse("100m")) != 0 {
		t.Fatalf("CPU request = %s", got.String())
	}
}

func TestSandboxedCellFailsClosedWithoutMapping(t *testing.T) {
	t.Parallel()
	cell := testCell("sandbox", dshv1alpha1.RetentionRetain)
	cell.Spec.SecurityClass = dshv1alpha1.SecuritySandboxed
	reconciler, kube := testReconciler(t, cell)
	reconcileCell(t, reconciler, cell)

	names := cellcontract.ResourceNames(string(cell.UID))
	var workload appsv1.StatefulSet
	if err := kube.Get(context.Background(), client.ObjectKey{Namespace: cell.Namespace, Name: names.Base}, &workload); err == nil {
		t.Fatal("sandboxed StatefulSet exists without an operator mapping")
	}
	observed := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	condition := meta.FindStatusCondition(observed.Status.Conditions, dshv1alpha1.ConditionWorkloadReady)
	if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != reasonSandboxRuntimeClassUnconfigured {
		t.Fatalf("unexpected sandbox condition: %#v", condition)
	}

	reconciler.SandboxedRuntimeClass = "gvisor"
	reconcileCell(t, reconciler, cell)
	workload = *get[*appsv1.StatefulSet](t, kube, cell.Namespace, names.Base)
	if workload.Spec.Template.Spec.RuntimeClassName == nil || *workload.Spec.Template.Spec.RuntimeClassName != "gvisor" {
		t.Fatalf("RuntimeClass mapping missing: %#v", workload.Spec.Template.Spec.RuntimeClassName)
	}
}

func TestForeignResourceFailsClosed(t *testing.T) {
	t.Parallel()
	cell := testCell("collision", dshv1alpha1.RetentionRetain)
	names := cellcontract.ResourceNames(string(cell.UID))
	foreign := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}}
	reconciler, kube := testReconciler(t, cell, foreign)
	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cell)})
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("collision result=%#v, error=%v", result, err)
	}
	observed := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	condition := meta.FindStatusCondition(observed.Status.Conditions, dshv1alpha1.ConditionAccessReady)
	if condition == nil || condition.Reason != reasonOwnershipConflict {
		t.Fatalf("unexpected collision condition: %#v", condition)
	}
	unchanged := get[*corev1.Service](t, kube, cell.Namespace, names.Base)
	if len(unchanged.OwnerReferences) != 0 || len(unchanged.Spec.Ports) != 0 {
		t.Fatal("foreign Service was adopted or overwritten")
	}
}

type createCollisionClient struct {
	client.Client
	foreign client.Object
	fired   bool
}

func (c *createCollisionClient) Create(ctx context.Context, object client.Object, options ...client.CreateOption) error {
	if _, target := object.(*corev1.Service); target && !c.fired && object.GetName() == c.foreign.GetName() {
		c.fired = true
		if err := c.Client.Create(ctx, c.foreign); err != nil {
			return err
		}
		return apierrors.NewAlreadyExists(schema.GroupResource{Resource: "services"}, object.GetName())
	}
	return c.Client.Create(ctx, object, options...)
}

func TestCreateRaceNeverAdoptsForeignObject(t *testing.T) {
	t.Parallel()
	cell := testCell("create-race", dshv1alpha1.RetentionRetain)
	reconciler, kube := testReconciler(t, cell)
	names := cellcontract.ResourceNames(string(cell.UID))
	foreign := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: names.Headless, Namespace: cell.Namespace}}
	reconciler.Client = &createCollisionClient{Client: kube, foreign: foreign}
	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cell)})
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("create collision result=%#v, error=%v", result, err)
	}
	observed := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	condition := meta.FindStatusCondition(observed.Status.Conditions, dshv1alpha1.ConditionWorkloadReady)
	if condition == nil || condition.Reason != reasonOwnershipConflict {
		t.Fatalf("unexpected create-race condition: %#v", condition)
	}
	unchanged := get[*corev1.Service](t, kube, cell.Namespace, names.Headless)
	if len(unchanged.OwnerReferences) != 0 || len(unchanged.Annotations) != 0 || len(unchanged.Spec.Ports) != 0 {
		t.Fatalf("foreign object was adopted in create race: %#v", unchanged)
	}
}

func TestAccessReadyRequiresManagedServiceSliceAndPodChain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cell := testCell("endpoint-ownership", dshv1alpha1.RetentionRetain)
	reconciler, kube := testReconciler(t, cell)
	reconcileCell(t, reconciler, cell)
	names := cellcontract.ResourceNames(string(cell.UID))
	service := get[*corev1.Service](t, kube, cell.Namespace, names.Base)
	service.UID = types.UID("access-service-uid")
	if err := kube.Update(ctx, service); err != nil {
		t.Fatal(err)
	}
	workload := get[*appsv1.StatefulSet](t, kube, cell.Namespace, names.Base)
	workload.UID = types.UID("workload-uid")
	if err := kube.Update(ctx, workload); err != nil {
		t.Fatal(err)
	}
	ready := true
	spoof := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spoof", Namespace: cell.Namespace,
			Labels: map[string]string{discoveryv1.LabelServiceName: service.Name},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{{
			Addresses: []string{"10.0.0.99"}, Conditions: discoveryv1.EndpointConditions{Ready: &ready},
		}},
	}
	if err := kube.Create(ctx, spoof); err != nil {
		t.Fatal(err)
	}
	if observed, err := reconciler.accessEndpointReady(ctx, cell); err != nil || observed {
		t.Fatalf("foreign labeled EndpointSlice became ready: ready=%v err=%v", observed, err)
	}

	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: workload.Name + "-0", Namespace: cell.Namespace, UID: types.UID("pod-uid"),
		Labels: workloadSelector(cell), Annotations: cellAnnotations(cell),
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "StatefulSet", Name: workload.Name,
			UID: workload.UID, Controller: ptr.To(true),
		}},
	}}
	if err := kube.Create(ctx, pod); err != nil {
		t.Fatal(err)
	}
	managed := spoof.DeepCopy()
	managed.Name = "managed"
	managed.ResourceVersion = ""
	managed.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "v1", Kind: "Service", Name: service.Name, UID: service.UID, Controller: ptr.To(true),
	}}
	managed.Endpoints[0].TargetRef = &corev1.ObjectReference{
		// EndpointSlice uses the empty apiVersion representation for core/v1.
		Kind: "Pod", Namespace: cell.Namespace, Name: pod.Name, UID: pod.UID,
	}
	if err := kube.Create(ctx, managed); err != nil {
		t.Fatal(err)
	}
	if observed, err := reconciler.accessEndpointReady(ctx, cell); err != nil || !observed {
		t.Fatalf("managed endpoint chain was not ready: ready=%v err=%v", observed, err)
	}
}

func TestStaleSnapshotLockSelfRepairsByUID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cell := testCell("recreated", dshv1alpha1.RetentionRetain)
	cell.UID = types.UID("new-cell-uid")
	cell.Annotations = map[string]string{cellcontract.ActiveSnapshotAnnotation: string(testSnapshotUID)}
	snapshot := &dshv1alpha1.CellSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "old-operation", Namespace: cell.Namespace, UID: testSnapshotUID},
		Spec:       dshv1alpha1.CellSnapshotSpec{CellRef: dshv1alpha1.LocalCellReference{Name: cell.Name}, VolumeSnapshotClassName: "class"},
		Status:     dshv1alpha1.CellSnapshotStatus{SourceCellUID: string(testUID)},
	}
	reconciler, kube := testReconciler(t, cell, snapshot)
	activity, err := reconciler.snapshotActivity(ctx, cell)
	if err != nil || !activity.StaleLockCleared || activity.Active {
		t.Fatalf("stale lock repair = %#v err=%v", activity, err)
	}
	observed := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	if observed.Annotations[cellcontract.ActiveSnapshotAnnotation] != "" {
		t.Fatal("same-name recreated Cell retained an old-UID operation lock")
	}
}

func TestSnapshotLockSurvivesAcceptedStatusCASWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cell := testCell("source", dshv1alpha1.RetentionRetain)
	cell.Annotations = map[string]string{cellcontract.ActiveSnapshotAnnotation: string(testSnapshotUID)}
	snapshot := &dshv1alpha1.CellSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "acquiring", Namespace: cell.Namespace, UID: testSnapshotUID, Generation: 1},
		Spec: dshv1alpha1.CellSnapshotSpec{
			CellRef: dshv1alpha1.LocalCellReference{Name: cell.Name}, VolumeSnapshotClassName: "class",
		},
		Status: dshv1alpha1.CellSnapshotStatus{
			ObservedGeneration: 1, SourceCellUID: string(cell.UID), SourcePVCUID: "pvc-uid", SourceGeneration: cell.Generation,
			Conditions: []metav1.Condition{{
				Type: dshv1alpha1.ConditionSnapshotAccepted, Status: metav1.ConditionFalse,
				ObservedGeneration: 1, Reason: reasonSnapshotAcquiringLock,
			}},
		},
	}
	reconciler, kube := testReconciler(t, cell, snapshot)
	activity, err := reconciler.snapshotActivity(ctx, cell)
	if err != nil || !activity.Active || activity.StopWriter || activity.StaleLockCleared {
		t.Fatalf("CAS-window activity = %#v err=%v", activity, err)
	}
	observed := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	if observed.Annotations[cellcontract.ActiveSnapshotAnnotation] != string(testSnapshotUID) {
		t.Fatal("newly acquired operation lock was cleared before Accepted status persisted")
	}
}

func TestAggregateReadyReasonPreservesSnapshotState(t *testing.T) {
	t.Parallel()
	state := pendingState()
	state.Workload = falseCondition(reasonSnapshotInProgress, "writer stopping")
	state.Access = falseCondition(reasonSnapshotInProgress, "endpoint removed")
	condition := aggregateReadyState(state, false)
	if condition.Status != metav1.ConditionFalse || condition.Reason != reasonSnapshotInProgress {
		t.Fatalf("snapshot aggregate = %#v", condition)
	}

	condition = aggregateReadyState(observedState{
		Storage:  trueCondition(reasonPVCsBound, "bound"),
		Workload: trueCondition(reasonStatefulSetReady, "ready"),
		Access:   trueCondition(reasonEndpointReady, "ready"),
	}, true)
	if condition.Status != metav1.ConditionTrue || condition.Reason != reasonComponentsReady {
		t.Fatalf("ready aggregate = %#v", condition)
	}
}

func assertWorkloadContract(t *testing.T, workload *appsv1.StatefulSet, names cellcontract.Names) {
	t.Helper()
	if workload.Spec.Replicas == nil || *workload.Spec.Replicas != 1 || workload.Spec.ServiceName != names.Headless {
		t.Fatalf("unexpected StatefulSet identity: %#v", workload.Spec)
	}
	pod := workload.Spec.Template.Spec
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken || pod.EnableServiceLinks == nil || *pod.EnableServiceLinks {
		t.Fatal("workload token or service links are enabled")
	}
	if pod.SecurityContext == nil || pod.SecurityContext.RunAsUser == nil || *pod.SecurityContext.RunAsUser != 1000 {
		t.Fatalf("unexpected Pod security context: %#v", pod.SecurityContext)
	}
	container := pod.Containers[0]
	if container.Image != "example.test/cell@"+testDigest || container.Command[0] != cellcontract.LauncherPath || container.WorkingDir != cellcontract.DataRoot {
		t.Fatalf("unexpected container contract: %#v", container)
	}
	if container.SecurityContext == nil || container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
		t.Fatalf("root filesystem is writable: %#v", container.SecurityContext)
	}
	if container.StartupProbe == nil || container.ReadinessProbe == nil || container.LivenessProbe == nil {
		t.Fatal("launcher probes are missing")
	}
}

func markReady(
	t *testing.T,
	kube client.Client,
	data, private *corev1.PersistentVolumeClaim,
	workload *appsv1.StatefulSet,
	access *corev1.Service,
) {
	t.Helper()
	ctx := context.Background()
	if workload.UID == "" {
		workload.UID = types.UID("managed-statefulset-uid")
		if err := kube.Update(ctx, workload); err != nil {
			t.Fatal(err)
		}
	}
	if access.UID == "" {
		access.UID = types.UID("managed-service-uid")
		if err := kube.Update(ctx, access); err != nil {
			t.Fatal(err)
		}
	}
	for _, claim := range []*corev1.PersistentVolumeClaim{data, private} {
		claim.Status.Phase = corev1.ClaimBound
		if err := kube.Status().Update(ctx, claim); err != nil {
			t.Fatal(err)
		}
	}
	workload.Status = appsv1.StatefulSetStatus{
		ObservedGeneration: workload.Generation,
		Replicas:           1,
		ReadyReplicas:      1,
		CurrentReplicas:    1,
		UpdatedReplicas:    1,
		CurrentRevision:    "ready",
		UpdateRevision:     "ready",
	}
	if err := kube.Status().Update(ctx, workload); err != nil {
		t.Fatal(err)
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        workload.Name + "-0",
			Namespace:   workload.Namespace,
			UID:         types.UID("managed-pod-uid"),
			Labels:      workload.Spec.Template.Labels,
			Annotations: workload.Spec.Template.Annotations,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "StatefulSet", Name: workload.Name,
				UID: workload.UID, Controller: ptr.To(true),
			}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	if err := kube.Create(ctx, pod); err != nil {
		t.Fatal(err)
	}
	ready := true
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      access.Name + "-ready",
			Namespace: access.Namespace,
			Labels:    map[string]string{discoveryv1.LabelServiceName: access.Name},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1", Kind: "Service", Name: access.Name, UID: access.UID, Controller: ptr.To(true),
			}},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{"10.0.0.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: &ready},
			TargetRef: &corev1.ObjectReference{
				APIVersion: "v1", Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name, UID: pod.UID,
			},
		}},
	}
	if err := kube.Create(ctx, slice); err != nil {
		t.Fatal(err)
	}
}

func testCell(name string, retention dshv1alpha1.RetentionPolicy) *dshv1alpha1.Cell {
	return &dshv1alpha1.Cell{
		TypeMeta: metav1.TypeMeta{APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "Cell"},
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  "tenant-a",
			UID:        testUID,
			Generation: 1,
		},
		Spec: dshv1alpha1.CellSpec{
			Image:         "example.test/cell@" + testDigest,
			SecurityClass: dshv1alpha1.SecurityStandard,
			Storage: dshv1alpha1.CellStorageSpec{
				Size:            resource.MustParse("1Gi"),
				RetentionPolicy: retention,
			},
		},
	}
}

func testReconciler(t *testing.T, objects ...client.Object) (*CellReconciler, client.Client) {
	t.Helper()
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
	kube := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dshv1alpha1.Cell{}, &dshv1alpha1.CellSnapshot{}, &corev1.PersistentVolumeClaim{}, &appsv1.StatefulSet{}, &volumesnapshotv1.VolumeSnapshot{}).
		WithObjects(objects...).
		Build()
	return &CellReconciler{Client: kube, Scheme: scheme, SystemNamespace: "dsh-system"}, kube
}

func reconcileCell(t *testing.T, reconciler *CellReconciler, cell *dshv1alpha1.Cell) {
	t.Helper()
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cell)}); err != nil {
		t.Fatal(err)
	}
}

func get[T client.Object](t *testing.T, kube client.Client, namespace, name string) T {
	t.Helper()
	var zero T
	object := reflectNew(zero)
	if err := kube.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, object); err != nil {
		t.Fatal(err)
	}
	return object
}

func reflectNew[T client.Object](zero T) T {
	return reflect.New(reflect.TypeOf(zero).Elem()).Interface().(T)
}
