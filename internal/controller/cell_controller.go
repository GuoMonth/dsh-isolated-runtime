// Package controller translates the narrow Cell API into ordinary Kubernetes
// resources and reports topology-free observed state.
package controller

//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0 rbac:roleName=cell-operator paths=../../... output:rbac:artifacts:config=../../config/rbac

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controlleroptions "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

const externalPrerequisiteRequeue = time.Minute

func immediateRequeueResult() ctrl.Result {
	return ctrl.Result{RequeueAfter: time.Nanosecond}
}

const (
	reasonPVCsPending                     = "PVCsPending"
	reasonPVCsBound                       = "PVCsBound"
	reasonStatefulSetPending              = "StatefulSetPending"
	reasonStatefulSetReady                = "StatefulSetReady"
	reasonEndpointPending                 = "EndpointPending"
	reasonEndpointReady                   = "EndpointReady"
	reasonComponentsNotReady              = "ComponentsNotReady"
	reasonComponentsReady                 = "ComponentsReady"
	reasonOwnershipConflict               = "OwnershipConflict"
	reasonReconcileFailed                 = "ReconcileFailed"
	reasonSandboxRuntimeClassUnconfigured = "SandboxRuntimeClassUnconfigured"
	reasonSnapshotInProgress              = "SnapshotInProgress"
)

// CellReconciler reconciles Cells across all namespaces.
type CellReconciler struct {
	client.Client
	APIReader               client.Reader
	Scheme                  *runtime.Scheme
	SystemNamespace         string
	SandboxedRuntimeClass   string
	RouteConfig             RouteConfig
	Recorder                record.EventRecorder
	routeAPIAvailable       bool
	SnapshotEnabled         bool
	MaxConcurrentReconciles int
}

// The access verb is required by Kubernetes RBAC escalation prevention: the
// controller creates namespace Roles that grant this verb, even though it never
// performs Cell access checks itself.
// +kubebuilder:rbac:groups=dsh.isolated.io,resources=cells,verbs=get;list;watch;access
// +kubebuilder:rbac:groups=dsh.isolated.io,resources=cells/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dsh.isolated.io,resources=cellsnapshots,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;serviceaccounts;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager registers owned-resource and ready-endpoint watches.
func (r *CellReconciler) SetupWithManager(manager ctrl.Manager) error {
	if strings.TrimSpace(r.SystemNamespace) == "" {
		return errors.New("system namespace is required")
	}
	if err := r.RouteConfig.Validate(); err != nil {
		return err
	}
	_, routeErr := manager.GetRESTMapper().RESTMapping(schema.GroupKind{
		Group: gatewayv1.GroupVersion.Group,
		Kind:  "HTTPRoute",
	}, gatewayv1.GroupVersion.Version)
	r.routeAPIAvailable = routeErr == nil
	if r.RouteConfig.Enabled() && routeErr != nil {
		return fmt.Errorf("gateway routing requires the HTTPRoute CRD: %w", routeErr)
	}

	builder := ctrl.NewControllerManagedBy(manager).
		WithOptions(controlleroptions.Options{MaxConcurrentReconciles: normalizedConcurrency(r.MaxConcurrentReconciles)}).
		For(&dshv1alpha1.Cell{}).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(r.mapManagedObject)).
		Watches(&appsv1.StatefulSet{}, handler.EnqueueRequestsFromMapFunc(r.mapManagedObject)).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(r.mapManagedObject)).
		Watches(&rbacv1.Role{}, handler.EnqueueRequestsFromMapFunc(r.mapDerivedAccessObject)).
		Watches(&networkingv1.NetworkPolicy{}, handler.EnqueueRequestsFromMapFunc(r.mapManagedObject)).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(r.mapManagedObject)).
		Watches(&discoveryv1.EndpointSlice{}, handler.EnqueueRequestsFromMapFunc(r.mapEndpointSlice)).
		Watches(&dshv1alpha1.CellSnapshot{}, handler.EnqueueRequestsFromMapFunc(r.mapCellSnapshot))
	if r.routeAPIAvailable {
		builder = builder.Watches(&gatewayv1.HTTPRoute{}, handler.EnqueueRequestsFromMapFunc(r.mapDerivedAccessObject))
	}
	return builder.Complete(r)
}

func normalizedConcurrency(configured int) int {
	if configured < 1 {
		return 1
	}
	return configured
}

// Reconcile converges a Cell entirely through Kubernetes observed state.
func (r *CellReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var cell dshv1alpha1.Cell
	if err := r.Get(ctx, request.NamespacedName, &cell); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if cell.UID == "" {
		return ctrl.Result{}, nil
	}
	if cell.DeletionTimestamp != nil {
		handled, err := r.cleanupDeletingRestore(ctx, &cell)
		if handled {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	state := pendingState()
	restore, restoreState, err := r.resolveRestore(ctx, &cell)
	if err != nil {
		state.Storage = failedCondition(err, "restore source reconciliation failed")
		return r.finish(ctx, &cell, state, err)
	}
	if restoreState != nil {
		state.Storage = *restoreState
		return r.finish(ctx, &cell, state, nil)
	}
	dataPVC, err := r.reconcileDataPVC(ctx, &cell, restore)
	if err != nil {
		state.Storage = failedCondition(err, "managed storage reconciliation failed")
		return r.finish(ctx, &cell, state, err)
	}
	privatePVC, err := r.reconcilePrivatePVC(ctx, &cell, dataPVC.Spec.StorageClassName)
	if err != nil {
		state.Storage = failedCondition(err, "managed storage reconciliation failed")
		return r.finish(ctx, &cell, state, err)
	}
	if dataPVC.Status.Phase == corev1.ClaimBound && privatePVC.Status.Phase == corev1.ClaimBound {
		state.Storage = trueCondition(reasonPVCsBound, "data and private PVCs are bound")
	}
	snapshotActivity, err := r.snapshotActivity(ctx, &cell)
	if err != nil {
		state.Workload = failedCondition(err, "snapshot state observation failed")
		return r.finish(ctx, &cell, state, err)
	}
	if snapshotActivity.StaleLockCleared {
		return immediateRequeueResult(), nil
	}

	if err := r.reconcileServiceAccount(ctx, &cell); err != nil {
		state.Workload = failedCondition(err, "workload identity reconciliation failed")
		return r.finish(ctx, &cell, state, err)
	}
	if err := r.reconcileHeadlessService(ctx, &cell); err != nil {
		state.Workload = failedCondition(err, "workload identity reconciliation failed")
		return r.finish(ctx, &cell, state, err)
	}
	if err := r.reconcileAccessService(ctx, &cell); err != nil {
		state.Access = failedCondition(err, "access Service reconciliation failed")
		return r.finish(ctx, &cell, state, err)
	}
	if err := r.reconcileNetworkPolicy(ctx, &cell); err != nil {
		state.Access = failedCondition(err, "ingress policy reconciliation failed")
		return r.finish(ctx, &cell, state, err)
	}
	routeErr := r.reconcilePublicAccess(ctx, &cell)

	if cell.Spec.SecurityClass == dshv1alpha1.SecuritySandboxed && strings.TrimSpace(r.SandboxedRuntimeClass) == "" {
		if err := r.deleteStatefulSet(ctx, &cell); err != nil {
			state.Workload = failedCondition(err, "sandboxed workload cleanup failed")
			return r.finish(ctx, &cell, state, err)
		}
		state.Workload = falseCondition(
			reasonSandboxRuntimeClassUnconfigured,
			"sandboxed RuntimeClass mapping is not configured",
		)
		return r.finishPublic(ctx, &cell, state, routeErr)
	}

	replicas := int32(1)
	if snapshotActivity.StopWriter {
		replicas = 0
	}
	workload, err := r.reconcileStatefulSet(ctx, &cell, replicas)
	if err != nil {
		state.Workload = failedCondition(err, "StatefulSet reconciliation failed")
		return r.finish(ctx, &cell, state, err)
	}
	if statefulSetReady(workload) && restore != nil && !restore.Initialized {
		if len(workload.Spec.Template.Spec.Containers) != 1 || imageDigest(workload.Spec.Template.Spec.Containers[0].Image) != restore.ImageDigest {
			state.Workload = falseCondition(reasonRestoreImageMismatch, "recorded restore image has not become the Ready reader")
			return r.finish(ctx, &cell, state, nil)
		}
		if err := r.completeRestoreInitialization(ctx, &cell, dataPVC, restore); err != nil {
			state.Workload = failedCondition(err, "restore initialization completion failed")
			return r.finish(ctx, &cell, state, err)
		}
		return immediateRequeueResult(), nil
	}
	if statefulSetReady(workload) && !snapshotActivity.Active {
		state.Workload = trueCondition(reasonStatefulSetReady, "current StatefulSet revision has one ready replica")
	}
	if snapshotActivity.Active {
		state.Workload = falseCondition(reasonSnapshotInProgress, "CellSnapshot is stopping the sole managed writer")
		state.Access = falseCondition(reasonSnapshotInProgress, "CellSnapshot intentionally removed the access endpoint")
		return r.finishPublic(ctx, &cell, state, routeErr)
	}

	accessReady, err := r.accessEndpointReady(ctx, &cell)
	if err != nil {
		state.Access = falseCondition(reasonReconcileFailed, "ready endpoint observation failed")
		return r.finish(ctx, &cell, state, err)
	}
	if accessReady {
		state.Access = trueCondition(reasonEndpointReady, "access Service has a ready endpoint")
	}
	return r.finishPublic(ctx, &cell, state, routeErr)
}

func (r *CellReconciler) finishPublic(
	ctx context.Context,
	cell *dshv1alpha1.Cell,
	state observedState,
	publicErr error,
) (ctrl.Result, error) {
	result, err := r.finish(ctx, cell, state, nil)
	if err != nil || publicErr == nil {
		return result, err
	}
	var conflict *ownershipConflictError
	if errors.As(publicErr, &conflict) {
		return result, nil
	}
	return result, publicErr
}

type componentState struct {
	Status  metav1.ConditionStatus
	Reason  string
	Message string
}

type observedState struct {
	Storage  componentState
	Workload componentState
	Access   componentState
}

func pendingState() observedState {
	return observedState{
		Storage:  falseCondition(reasonPVCsPending, "waiting for data and private PVCs"),
		Workload: falseCondition(reasonStatefulSetPending, "waiting for the current StatefulSet revision"),
		Access:   falseCondition(reasonEndpointPending, "waiting for a ready access endpoint"),
	}
}

func trueCondition(reason, message string) componentState {
	return componentState{Status: metav1.ConditionTrue, Reason: reason, Message: message}
}

func falseCondition(reason, message string) componentState {
	return componentState{Status: metav1.ConditionFalse, Reason: reason, Message: message}
}

func failedCondition(err error, message string) componentState {
	reason := reasonReconcileFailed
	var conflict *ownershipConflictError
	if errors.As(err, &conflict) {
		reason = reasonOwnershipConflict
		message = "managed resource ownership conflict"
	}
	return falseCondition(reason, message)
}

func (r *CellReconciler) finish(
	ctx context.Context,
	cell *dshv1alpha1.Cell,
	state observedState,
	reconcileErr error,
) (ctrl.Result, error) {
	ready := state.Storage.Status == metav1.ConditionTrue &&
		state.Workload.Status == metav1.ConditionTrue &&
		state.Access.Status == metav1.ConditionTrue
	readyState := aggregateReadyState(state, ready)

	desired := copyStatus(cell.Status)
	desired.ObservedGeneration = cell.Generation
	desired.Conditions = []metav1.Condition{
		conditionFor(cell, dshv1alpha1.ConditionStorageReady, state.Storage),
		conditionFor(cell, dshv1alpha1.ConditionWorkloadReady, state.Workload),
		conditionFor(cell, dshv1alpha1.ConditionAccessReady, state.Access),
		conditionFor(cell, dshv1alpha1.ConditionReady, readyState),
	}
	if state.Workload.Status == metav1.ConditionTrue {
		desired.DSHVersion = cellcontract.DSHVersion
		desired.ImageDigest = imageDigest(cell.Spec.Image)
	} else {
		desired.DSHVersion = ""
		desired.ImageDigest = ""
	}

	if !reflect.DeepEqual(cell.Status, *desired) {
		original := cell.DeepCopy()
		cell.Status = *desired
		if err := r.Status().Patch(ctx, cell, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, errors.Join(reconcileErr, fmt.Errorf("update Cell status: %w", err))
		}
	}
	if reconcileErr != nil {
		var conflict *ownershipConflictError
		if errors.As(reconcileErr, &conflict) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, reconcileErr
	}
	return ctrl.Result{}, nil
}

func aggregateReadyState(state observedState, ready bool) componentState {
	if ready {
		return trueCondition(reasonComponentsReady, "all Cell components are ready")
	}
	if state.Workload.Reason == reasonSnapshotInProgress || state.Access.Reason == reasonSnapshotInProgress {
		return falseCondition(reasonSnapshotInProgress, "CellSnapshot is stopping the sole managed writer")
	}
	return falseCondition(reasonComponentsNotReady, "one or more Cell components are not ready")
}

func conditionFor(cell *dshv1alpha1.Cell, conditionType string, state componentState) metav1.Condition {
	transition := metav1.Now()
	for _, existing := range cell.Status.Conditions {
		if existing.Type == conditionType && existing.Status == state.Status && existing.Reason == state.Reason && existing.Message == state.Message {
			transition = existing.LastTransitionTime
			break
		}
	}
	return metav1.Condition{
		Type:               conditionType,
		Status:             state.Status,
		ObservedGeneration: cell.Generation,
		LastTransitionTime: transition,
		Reason:             state.Reason,
		Message:            state.Message,
	}
}

func imageDigest(image string) string {
	const separator = "@"
	index := strings.LastIndex(image, separator)
	if index < 0 || index == len(image)-1 {
		return ""
	}
	return image[index+1:]
}

func statefulSetReady(statefulSet *appsv1.StatefulSet) bool {
	return statefulSet.Status.ObservedGeneration >= statefulSet.Generation &&
		statefulSet.Status.Replicas == 1 &&
		statefulSet.Status.ReadyReplicas == 1 &&
		statefulSet.Status.CurrentReplicas == 1 &&
		statefulSet.Status.UpdatedReplicas == 1 &&
		statefulSet.Status.CurrentRevision != "" &&
		statefulSet.Status.CurrentRevision == statefulSet.Status.UpdateRevision
}

func (r *CellReconciler) accessEndpointReady(ctx context.Context, cell *dshv1alpha1.Cell) (bool, error) {
	names := cellcontract.ResourceNames(string(cell.UID))
	var service corev1.Service
	if err := r.cellRead(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.Base}, &service); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := validateManaged(cell, &service, true); err != nil {
		return false, err
	}
	var workload appsv1.StatefulSet
	if err := r.cellRead(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.Base}, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if err := validateManaged(cell, &workload, true); err != nil {
		return false, err
	}
	var slices discoveryv1.EndpointSliceList
	if err := r.cellList(
		ctx,
		&slices,
		client.InNamespace(cell.Namespace),
		client.MatchingLabels{discoveryv1.LabelServiceName: names.Base},
	); err != nil {
		return false, err
	}
	for _, slice := range slices.Items {
		owner := metav1.GetControllerOf(&slice)
		if owner == nil || owner.APIVersion != "v1" || owner.Kind != "Service" || owner.Name != service.Name || owner.UID != service.UID {
			continue
		}
		for _, endpoint := range slice.Endpoints {
			if endpoint.Conditions.Ready == nil || !*endpoint.Conditions.Ready || endpoint.TargetRef == nil ||
				(endpoint.TargetRef.APIVersion != "" && endpoint.TargetRef.APIVersion != "v1") ||
				endpoint.TargetRef.Kind != "Pod" || endpoint.TargetRef.Name == "" || endpoint.TargetRef.UID == "" ||
				(endpoint.TargetRef.Namespace != "" && endpoint.TargetRef.Namespace != cell.Namespace) {
				continue
			}
			var pod corev1.Pod
			if err := r.cellRead(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: endpoint.TargetRef.Name}, &pod); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return false, err
			}
			podOwner := metav1.GetControllerOf(&pod)
			if pod.Name == workload.Name+"-0" && pod.UID == endpoint.TargetRef.UID && pod.DeletionTimestamp == nil &&
				pod.Annotations[cellcontract.CellUIDAnnotation] == string(cell.UID) &&
				pod.Annotations[cellcontract.CellNameAnnotation] == cell.Name &&
				podOwner != nil && podOwner.APIVersion == appsv1.SchemeGroupVersion.String() &&
				podOwner.Kind == "StatefulSet" && podOwner.Name == workload.Name && podOwner.UID == workload.UID {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *CellReconciler) mapManagedObject(ctx context.Context, object client.Object) []reconcile.Request {
	annotations := object.GetAnnotations()
	name := annotations[cellcontract.CellNameAnnotation]
	if name != "" && annotations[cellcontract.CellUIDAnnotation] != "" {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: object.GetNamespace(), Name: name}}}
	}

	// Foreign collisions intentionally have no Cell annotations. Resolve them
	// by the UID-derived name so deletion or replacement wakes the affected
	// Cell without a permanent polling loop.
	var cells dshv1alpha1.CellList
	if err := r.List(ctx, &cells, client.InNamespace(object.GetNamespace())); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0, 1)
	for index := range cells.Items {
		cell := &cells.Items[index]
		names := cellcontract.ResourceNames(string(cell.UID))
		if managedObjectNameMatches(object, names) {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cell)})
		}
	}
	return requests
}

func managedObjectNameMatches(object client.Object, names cellcontract.Names) bool {
	switch object.(type) {
	case *corev1.PersistentVolumeClaim:
		return object.GetName() == names.DataPVC || object.GetName() == names.PrivatePVC
	case *corev1.Service:
		return object.GetName() == names.Base || object.GetName() == names.Headless
	case *corev1.ServiceAccount, *appsv1.StatefulSet, *networkingv1.NetworkPolicy:
		return object.GetName() == names.Base
	default:
		return false
	}
}

func (r *CellReconciler) mapEndpointSlice(ctx context.Context, object client.Object) []reconcile.Request {
	serviceName := object.GetLabels()[discoveryv1.LabelServiceName]
	if serviceName == "" {
		return nil
	}
	var service corev1.Service
	if err := r.Get(ctx, types.NamespacedName{Namespace: object.GetNamespace(), Name: serviceName}, &service); err != nil {
		return nil
	}
	return r.mapManagedObject(ctx, &service)
}

func (r *CellReconciler) mapCellSnapshot(_ context.Context, object client.Object) []reconcile.Request {
	snapshot, ok := object.(*dshv1alpha1.CellSnapshot)
	if !ok || snapshot.Spec.CellRef.Name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.Spec.CellRef.Name}}}
}

type cellSnapshotActivity struct {
	Active           bool
	StopWriter       bool
	StaleLockCleared bool
}

func (r *CellReconciler) snapshotActivity(ctx context.Context, cell *dshv1alpha1.Cell) (cellSnapshotActivity, error) {
	activeUID := cell.Annotations[cellcontract.ActiveSnapshotAnnotation]
	if activeUID == "" {
		return cellSnapshotActivity{}, nil
	}
	var snapshots dshv1alpha1.CellSnapshotList
	if err := r.cellList(ctx, &snapshots, client.InNamespace(cell.Namespace)); err != nil {
		return cellSnapshotActivity{}, err
	}
	for _, snapshot := range snapshots.Items {
		if string(snapshot.UID) != activeUID {
			continue
		}
		validBinding := snapshot.Spec.CellRef.Name == cell.Name && snapshot.Status.SourceCellUID == string(cell.UID)
		terminal := conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotReady) ||
			conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotFailed)
		if validBinding && !terminal {
			stopWriter := conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) ||
				failureCleanupRequired(snapshot.Status.Conditions)
			return cellSnapshotActivity{Active: true, StopWriter: stopWriter}, nil
		}
		break
	}
	original := cell.DeepCopy()
	copy := cell.DeepCopy()
	delete(copy.Annotations, cellcontract.ActiveSnapshotAnnotation)
	if err := r.Patch(ctx, copy, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})); err != nil {
		return cellSnapshotActivity{}, err
	}
	return cellSnapshotActivity{StaleLockCleared: true}, nil
}

func (r *CellReconciler) cellRead(ctx context.Context, key client.ObjectKey, object client.Object) error {
	if r.APIReader != nil {
		return r.APIReader.Get(ctx, key, object)
	}
	return r.Get(ctx, key, object)
}

func (r *CellReconciler) cellList(ctx context.Context, list client.ObjectList, options ...client.ListOption) error {
	if r.APIReader != nil {
		return r.APIReader.List(ctx, list, options...)
	}
	return r.List(ctx, list, options...)
}

func copyStatus(status dshv1alpha1.CellStatus) *dshv1alpha1.CellStatus {
	copy := status
	copy.Conditions = append([]metav1.Condition(nil), status.Conditions...)
	return &copy
}
