package controller

import (
	"context"
	"errors"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/accesscontract"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

type RouteConfig = accesscontract.Config

func (r *CellReconciler) cellAuthority(cell *dshv1alpha1.Cell) string {
	if r.RouteConfig.Enabled() {
		return r.RouteConfig.Authority(string(cell.UID))
	}
	return cellcontract.Authority(cell.Namespace, string(cell.UID))
}

func (r *CellReconciler) mapDerivedAccessObject(ctx context.Context, object client.Object) []reconcile.Request {
	var cells dshv1alpha1.CellList
	if err := r.List(ctx, &cells, client.InNamespace(object.GetNamespace())); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0, 1)
	for i := range cells.Items {
		cell := &cells.Items[i]
		names := cellcontract.ResourceNames(string(cell.UID))
		matched := false
		switch object.(type) {
		case *rbacv1.Role:
			matched = object.GetName() == names.Base+"-access"
		case *gatewayv1.HTTPRoute:
			matched = object.GetName() == names.Base
		}
		if matched {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cell)})
		}
	}
	return requests
}

func (r *CellReconciler) reconcilePublicAccess(ctx context.Context, cell *dshv1alpha1.Cell) error {
	var err error
	if r.RouteConfig.Enabled() {
		err = errors.Join(r.reconcileAccessRole(ctx, cell), r.reconcileHTTPRoute(ctx, cell))
		if err == nil {
			r.observeHTTPRoute(ctx, cell)
		}
	} else {
		err = r.deleteAccessRole(ctx, cell)
		if r.routeAPIAvailable {
			err = errors.Join(err, r.deleteHTTPRoute(ctx, cell))
		}
	}
	if err != nil {
		if r.Recorder != nil {
			r.Recorder.Eventf(cell, "Warning", reasonForError(err), "public access reconciliation failed: %v", err)
		}
		ctrl.LoggerFrom(ctx).Error(err, "public access reconciliation failed", "cell", client.ObjectKeyFromObject(cell))
		return fmt.Errorf("public access reconciliation: %w", err)
	}
	return nil
}

func reasonForError(err error) string {
	var conflict *ownershipConflictError
	if errors.As(err, &conflict) {
		return reasonOwnershipConflict
	}
	return reasonReconcileFailed
}

func (r *CellReconciler) reconcileAccessRole(ctx context.Context, cell *dshv1alpha1.Cell) error {
	names := cellcontract.ResourceNames(string(cell.UID))
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: names.Base + "-access", Namespace: cell.Namespace}}
	return r.ensureManaged(ctx, cell, role, true, func(_ bool) error {
		role.Rules = []rbacv1.PolicyRule{{
			APIGroups:     []string{dshv1alpha1.GroupVersion.Group},
			Resources:     []string{"cells"},
			ResourceNames: []string{cell.Name},
			Verbs:         []string{"access"},
		}}
		return nil
	})
}

func (r *CellReconciler) reconcileHTTPRoute(ctx context.Context, cell *dshv1alpha1.Cell) error {
	names := cellcontract.ResourceNames(string(cell.UID))
	route := &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}}
	return r.ensureManaged(ctx, cell, route, true, func(_ bool) error {
		annotations := route.GetAnnotations()
		annotations[cellcontract.RouteCellNameAnnotation] = cell.Name
		annotations[cellcontract.RouteCellUIDAnnotation] = string(cell.UID)
		route.SetAnnotations(annotations)

		gatewayNamespace := gatewayv1.Namespace(r.RouteConfig.GatewayNamespace)
		gatewaySection := gatewayv1.SectionName(r.RouteConfig.GatewaySection)
		pathType := gatewayv1.PathMatchPathPrefix
		root := "/"
		backendPort := gatewayv1.PortNumber(cellcontract.ProxyServicePort)
		route.Spec = gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: []gatewayv1.ParentReference{{
				Group:       ptr.To(gatewayv1.Group(gatewayv1.GroupVersion.Group)),
				Kind:        ptr.To(gatewayv1.Kind("Gateway")),
				Namespace:   &gatewayNamespace,
				Name:        gatewayv1.ObjectName(r.RouteConfig.GatewayName),
				SectionName: &gatewaySection,
			}}},
			Hostnames: []gatewayv1.Hostname{gatewayv1.Hostname(r.RouteConfig.Hostname(string(cell.UID)))},
			Rules: []gatewayv1.HTTPRouteRule{{
				Matches: []gatewayv1.HTTPRouteMatch{{Path: &gatewayv1.HTTPPathMatch{Type: &pathType, Value: &root}}},
				BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Group: ptr.To(gatewayv1.Group("")),
						Kind:  ptr.To(gatewayv1.Kind("Service")),
						Name:  gatewayv1.ObjectName(names.Base),
						Port:  &backendPort,
					},
				}}},
			}},
		}
		return nil
	})
}

func (r *CellReconciler) deleteAccessRole(ctx context.Context, cell *dshv1alpha1.Cell) error {
	names := cellcontract.ResourceNames(string(cell.UID))
	return r.deleteManaged(ctx, cell, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: names.Base + "-access", Namespace: cell.Namespace}})
}

func (r *CellReconciler) deleteHTTPRoute(ctx context.Context, cell *dshv1alpha1.Cell) error {
	names := cellcontract.ResourceNames(string(cell.UID))
	return r.deleteManaged(ctx, cell, &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}})
}

func (r *CellReconciler) deleteManaged(ctx context.Context, cell *dshv1alpha1.Cell, object client.Object) error {
	if err := r.Get(ctx, client.ObjectKeyFromObject(object), object); err != nil {
		return client.IgnoreNotFound(err)
	}
	if err := validateManaged(cell, object, true); err != nil {
		return err
	}
	if object.GetDeletionTimestamp() != nil {
		return nil
	}
	return r.Delete(ctx, object)
}

func (r *CellReconciler) observeHTTPRoute(ctx context.Context, cell *dshv1alpha1.Cell) {
	if r.Recorder == nil {
		return
	}
	names := cellcontract.ResourceNames(string(cell.UID))
	var route gatewayv1.HTTPRoute
	if err := r.Get(ctx, client.ObjectKey{Namespace: cell.Namespace, Name: names.Base}, &route); err != nil {
		return
	}
	for _, parent := range route.Status.Parents {
		for _, condition := range parent.Conditions {
			if (condition.Type == string(gatewayv1.RouteConditionAccepted) || condition.Type == string(gatewayv1.RouteConditionResolvedRefs)) && condition.Status == metav1.ConditionFalse {
				r.Recorder.Eventf(cell, "Warning", "RouteRejected", "HTTPRoute %s: %s", condition.Reason, condition.Message)
				ctrl.LoggerFrom(ctx).Info("Cell HTTPRoute is not ready", "cell", client.ObjectKeyFromObject(cell), "reason", condition.Reason)
				return
			}
		}
	}
}
