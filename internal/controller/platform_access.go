package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/accesscontract"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

const accessModeAnnotation = "dsh.isolated.io/access-mode"

type accessModeConflict struct{ detail string }

func (e *accessModeConflict) Error() string {
	return "access mode conflict: " + e.detail + "; use a fresh Cell and inspect existing resources; no automatic migration"
}

func (r *CellReconciler) accessPodLabel() string {
	if r.RouteConfig.Mode == accesscontract.ModePlatform {
		return "platform"
	}
	return cellcontract.AccessValue
}

// Check before any resource mutation. Live reads avoid accepting cached absence.
// Cluster administrators still own arbitrary routes and NetworkPolicy grants.
func (r *CellReconciler) checkAccessMode(ctx context.Context, cell *dshv1alpha1.Cell) error {
	reader := r.APIReader
	if reader == nil {
		reader = r.Client
	}
	names := cellcontract.ResourceNames(string(cell.UID))
	key := client.ObjectKey{Namespace: cell.Namespace, Name: names.Base}
	var workload appsv1.StatefulSet
	err := reader.Get(ctx, key, &workload)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	platform := r.RouteConfig.Mode == accesscontract.ModePlatform
	if err == nil && (workload.Annotations[accessModeAnnotation] == string(accesscontract.ModePlatform)) != platform {
		return &accessModeConflict{detail: "existing workload belongs to another access mode"}
	}
	if !platform {
		return nil
	}
	if !r.routeAPIAvailable {
		return fmt.Errorf("platform access requires HTTPRoute discovery to detect direct routes")
	}
	var role rbacv1.Role
	err = reader.Get(ctx, client.ObjectKey{Namespace: cell.Namespace, Name: names.Base + "-access"}, &role)
	if err == nil {
		return &accessModeConflict{detail: "standalone access Role exists"}
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	var routes gatewayv1.HTTPRouteList
	if err := reader.List(ctx, &routes, client.InNamespace(cell.Namespace)); err != nil {
		return err
	}
	for _, route := range routes.Items {
		if route.Name == names.Base {
			return &accessModeConflict{detail: "standalone HTTPRoute exists"}
		}
		for _, rule := range route.Spec.Rules {
			for _, backend := range rule.BackendRefs {
				ref := backend.BackendObjectReference
				if targetsCellService(ref, cell.Namespace, names.Base) {
					return &accessModeConflict{detail: "HTTPRoute targets the Cell Service"}
				}
			}
		}
	}
	return nil
}

func targetsCellService(ref gatewayv1.BackendObjectReference, namespace, name string) bool {
	return string(ref.Name) == name && (ref.Namespace == nil || string(*ref.Namespace) == namespace) && (ref.Kind == nil || string(*ref.Kind) == "Service") && (ref.Group == nil || string(*ref.Group) == "")
}
