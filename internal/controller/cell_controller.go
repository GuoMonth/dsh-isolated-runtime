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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

const pendingRequeue = 3 * time.Second

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
)

// CellReconciler reconciles Cells across all namespaces.
type CellReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	SystemNamespace       string
	SandboxedRuntimeClass string
	RouteConfig           RouteConfig
	Recorder              record.EventRecorder
	routeAPIAvailable     bool
}

// The access verb is required by Kubernetes RBAC escalation prevention: the
// controller creates namespace Roles that grant this verb, even though it never
// performs Cell access checks itself.
// +kubebuilder:rbac:groups=dsh.isolated.io,resources=cells,verbs=get;list;watch;access
// +kubebuilder:rbac:groups=dsh.isolated.io,resources=cells/status,verbs=get;update;patch
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
		For(&dshv1alpha1.Cell{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Watches(&rbacv1.Role{}, handler.EnqueueRequestsFromMapFunc(r.mapDerivedAccessObject)).
		Owns(&networkingv1.NetworkPolicy{}).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(r.mapManagedObject)).
		Watches(&discoveryv1.EndpointSlice{}, handler.EnqueueRequestsFromMapFunc(r.mapEndpointSlice))
	if r.routeAPIAvailable {
		builder = builder.Watches(&gatewayv1.HTTPRoute{}, handler.EnqueueRequestsFromMapFunc(r.mapDerivedAccessObject))
	}
	return builder.Complete(r)
}

// Reconcile converges a Cell entirely through Kubernetes observed state.
func (r *CellReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var cell dshv1alpha1.Cell
	if err := r.Get(ctx, request.NamespacedName, &cell); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if cell.UID == "" {
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}

	state := pendingState()
	dataPVC, err := r.reconcileDataPVC(ctx, &cell)
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

	workload, err := r.reconcileStatefulSet(ctx, &cell)
	if err != nil {
		state.Workload = failedCondition(err, "StatefulSet reconciliation failed")
		return r.finish(ctx, &cell, state, err)
	}
	if statefulSetReady(workload) {
		state.Workload = trueCondition(reasonStatefulSetReady, "current StatefulSet revision has one ready replica")
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
	result.RequeueAfter = pendingRequeue
	return result, nil
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
	readyState := falseCondition(reasonComponentsNotReady, "one or more Cell components are not ready")
	if ready {
		readyState = trueCondition(reasonComponentsReady, "all Cell components are ready")
	}

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
		return ctrl.Result{}, reconcileErr
	}
	if !ready {
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}
	return ctrl.Result{}, nil
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
	var slices discoveryv1.EndpointSliceList
	if err := r.List(
		ctx,
		&slices,
		client.InNamespace(cell.Namespace),
		client.MatchingLabels{discoveryv1.LabelServiceName: names.Base},
	); err != nil {
		return false, err
	}
	for _, slice := range slices.Items {
		for _, endpoint := range slice.Endpoints {
			if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *CellReconciler) mapManagedObject(_ context.Context, object client.Object) []reconcile.Request {
	annotations := object.GetAnnotations()
	name := annotations[cellcontract.CellNameAnnotation]
	if name == "" || annotations[cellcontract.CellUIDAnnotation] == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: object.GetNamespace(), Name: name}}}
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

func copyStatus(status dshv1alpha1.CellStatus) *dshv1alpha1.CellStatus {
	copy := status
	copy.Conditions = append([]metav1.Condition(nil), status.Conditions...)
	return &copy
}
