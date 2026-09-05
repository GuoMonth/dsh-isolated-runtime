package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/accesscontract"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

func TestPublicAccessResourcesAndAuthority(t *testing.T) {
	t.Parallel()
	cell := testCell("main", "Retain")
	reconciler, kube := testReconciler(t, cell)
	reconciler.RouteConfig = accesscontract.Config{
		GatewayName:       "dsh",
		GatewayNamespace:  "dsh-system",
		GatewaySection:    "https",
		BaseDomain:        "cells.test",
		ExternalHTTPSPort: 18443,
	}
	reconciler.routeAPIAvailable = true
	reconcileCell(t, reconciler, cell)

	names := cellcontract.ResourceNames(string(cell.UID))
	role := get[*rbacv1.Role](t, kube, cell.Namespace, names.Base+"-access")
	if !metav1.IsControlledBy(role, cell) || len(role.Rules) != 1 || role.Rules[0].Verbs[0] != "access" || role.Rules[0].ResourceNames[0] != cell.Name {
		t.Fatalf("unexpected access Role: %#v", role)
	}
	if len(role.Rules[0].APIGroups) != 1 || role.Rules[0].APIGroups[0] != dshv1alpha1.GroupVersion.Group ||
		len(role.Rules[0].Resources) != 1 || role.Rules[0].Resources[0] != "cells" || len(role.Rules[0].Verbs) != 1 {
		t.Fatalf("access Role is broader than one Cell access verb: %#v", role.Rules)
	}
	route := get[*gatewayv1.HTTPRoute](t, kube, cell.Namespace, names.Base)
	if !metav1.IsControlledBy(route, cell) || string(route.Spec.Hostnames[0]) != reconciler.RouteConfig.Hostname(string(cell.UID)) {
		t.Fatalf("unexpected HTTPRoute: %#v", route)
	}
	if route.Annotations[cellcontract.RouteCellNameAnnotation] != cell.Name || route.Annotations[cellcontract.RouteCellUIDAnnotation] != string(cell.UID) {
		t.Fatalf("route metadata = %#v", route.Annotations)
	}
	parent := route.Spec.ParentRefs[0]
	if parent.Namespace == nil || string(*parent.Namespace) != "dsh-system" || string(parent.Name) != "dsh" || parent.SectionName == nil || string(*parent.SectionName) != "https" {
		t.Fatalf("route parent = %#v", parent)
	}
	if len(route.Spec.Rules) != 1 || len(route.Spec.Rules[0].Matches) != 1 ||
		route.Spec.Rules[0].Matches[0].Path == nil || *route.Spec.Rules[0].Matches[0].Path.Value != "/" ||
		len(route.Spec.Rules[0].BackendRefs) != 1 || string(route.Spec.Rules[0].BackendRefs[0].Name) != names.Base ||
		route.Spec.Rules[0].BackendRefs[0].Port == nil || int32(*route.Spec.Rules[0].BackendRefs[0].Port) != cellcontract.ProxyServicePort {
		t.Fatalf("route rule = %#v", route.Spec.Rules)
	}
	workload := get[*appsv1.StatefulSet](t, kube, cell.Namespace, names.Base)
	if got := environmentValue(workload, "CELL_AUTHORITY"); got != reconciler.RouteConfig.Authority(string(cell.UID)) {
		t.Fatalf("CELL_AUTHORITY = %q", got)
	}
}

func TestPublicAccessDriftIsRestored(t *testing.T) {
	t.Parallel()
	cell := testCell("main", "Retain")
	reconciler, kube := testReconciler(t, cell)
	reconciler.RouteConfig = accesscontract.Config{GatewayName: "dsh", GatewayNamespace: "dsh-system", GatewaySection: "https", BaseDomain: "cells.test"}
	reconciler.routeAPIAvailable = true
	reconcileCell(t, reconciler, cell)

	names := cellcontract.ResourceNames(string(cell.UID))
	role := get[*rbacv1.Role](t, kube, cell.Namespace, names.Base+"-access")
	role.Rules[0].Verbs = []string{"get"}
	if err := kube.Update(context.Background(), role); err != nil {
		t.Fatal(err)
	}
	route := get[*gatewayv1.HTTPRoute](t, kube, cell.Namespace, names.Base)
	route.Spec.ParentRefs[0].Name = "wrong-gateway"
	route.Spec.Rules[0].BackendRefs[0].Name = "wrong-backend"
	if err := kube.Update(context.Background(), route); err != nil {
		t.Fatal(err)
	}

	reconcileCell(t, reconciler, cell)
	role = get[*rbacv1.Role](t, kube, cell.Namespace, names.Base+"-access")
	route = get[*gatewayv1.HTTPRoute](t, kube, cell.Namespace, names.Base)
	if len(role.Rules) != 1 || len(role.Rules[0].Verbs) != 1 || role.Rules[0].Verbs[0] != "access" {
		t.Fatalf("Role drift survived reconcile: %#v", role.Rules)
	}
	if string(route.Spec.ParentRefs[0].Name) != "dsh" || string(route.Spec.Rules[0].BackendRefs[0].Name) != names.Base {
		t.Fatalf("HTTPRoute drift survived reconcile: %#v", route.Spec)
	}
}

func TestDisablingPublicAccessDeletesOwnedResources(t *testing.T) {
	t.Parallel()
	cell := testCell("main", "Retain")
	reconciler, kube := testReconciler(t, cell)
	reconciler.RouteConfig = accesscontract.Config{GatewayName: "dsh", GatewayNamespace: "dsh-system", GatewaySection: "https", BaseDomain: "cells.test"}
	reconciler.routeAPIAvailable = true
	reconcileCell(t, reconciler, cell)
	reconciler.RouteConfig = accesscontract.Config{}
	reconcileCell(t, reconciler, cell)

	names := cellcontract.ResourceNames(string(cell.UID))
	if err := kube.Get(context.Background(), client.ObjectKey{Namespace: cell.Namespace, Name: names.Base + "-access"}, &rbacv1.Role{}); !apierrors.IsNotFound(err) {
		t.Fatalf("Role survived disabled mode: %v", err)
	}
	if err := kube.Get(context.Background(), client.ObjectKey{Namespace: cell.Namespace, Name: names.Base}, &gatewayv1.HTTPRoute{}); !apierrors.IsNotFound(err) {
		t.Fatalf("HTTPRoute survived disabled mode: %v", err)
	}
	workload := get[*appsv1.StatefulSet](t, kube, cell.Namespace, names.Base)
	if got, want := environmentValue(workload, "CELL_AUTHORITY"), cellcontract.Authority(cell.Namespace, string(cell.UID)); got != want {
		t.Fatalf("internal authority = %q, want %q", got, want)
	}
}

func TestPublicAccessForeignRoleFailsClosedWithoutChangingStatusContract(t *testing.T) {
	t.Parallel()
	cell := testCell("main", "Retain")
	names := cellcontract.ResourceNames(string(cell.UID))
	foreign := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: names.Base + "-access", Namespace: cell.Namespace}}
	reconciler, kube := testReconciler(t, cell, foreign)
	reconciler.RouteConfig = accesscontract.Config{GatewayName: "dsh", GatewayNamespace: "dsh-system", GatewaySection: "https", BaseDomain: "cells.test"}
	reconciler.routeAPIAvailable = true
	result, err := reconciler.Reconcile(context.Background(), requestFor(cell))
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("foreign access Role result = %#v, %v", result, err)
	}
	stored := get[*rbacv1.Role](t, kube, cell.Namespace, names.Base+"-access")
	if len(stored.OwnerReferences) != 0 || len(stored.Rules) != 0 {
		t.Fatalf("foreign Role was mutated: %#v", stored)
	}
	updated := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	if len(updated.Status.Conditions) != 4 {
		t.Fatalf("Cell status shape changed: %#v", updated.Status)
	}
}

func TestPublicAccessRecoversOneResourceWhileAnotherStillConflicts(t *testing.T) {
	t.Parallel()
	cell := testCell("main", "Retain")
	names := cellcontract.ResourceNames(string(cell.UID))
	foreignRole := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: names.Base + "-access", Namespace: cell.Namespace}}
	foreignRoute := &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}}
	reconciler, kube := testReconciler(t, cell, foreignRole, foreignRoute)
	reconciler.RouteConfig = accesscontract.Config{GatewayName: "dsh", GatewayNamespace: "dsh-system", GatewaySection: "https", BaseDomain: "cells.test"}
	reconciler.routeAPIAvailable = true

	result, err := reconciler.Reconcile(context.Background(), requestFor(cell))
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("initial foreign resources result = %#v, %v", result, err)
	}
	if err := kube.Delete(context.Background(), foreignRole); err != nil {
		t.Fatalf("delete foreign Role: %v", err)
	}
	result, err = reconciler.Reconcile(context.Background(), requestFor(cell))
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("partial recovery result = %#v, %v", result, err)
	}
	role := get[*rbacv1.Role](t, kube, cell.Namespace, names.Base+"-access")
	if !metav1.IsControlledBy(role, cell) || len(role.Rules) != 1 || role.Rules[0].Verbs[0] != "access" {
		t.Fatalf("Role did not recover while Route remained foreign: %#v", role)
	}
	storedRoute := get[*gatewayv1.HTTPRoute](t, kube, cell.Namespace, names.Base)
	if len(storedRoute.OwnerReferences) != 0 {
		t.Fatalf("foreign HTTPRoute was adopted: %#v", storedRoute)
	}
}

func TestDerivedAccessWatchMapsForeignObjectsToCell(t *testing.T) {
	t.Parallel()
	cell := testCell("main", "Retain")
	names := cellcontract.ResourceNames(string(cell.UID))
	reconciler, _ := testReconciler(t, cell)

	objects := []client.Object{
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: names.Base + "-access", Namespace: cell.Namespace}},
		&gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}},
	}
	for _, object := range objects {
		requests := reconciler.mapDerivedAccessObject(context.Background(), object)
		if len(requests) != 1 || requests[0].NamespacedName != client.ObjectKeyFromObject(cell) {
			t.Fatalf("mapping %T returned %#v", object, requests)
		}
	}
	unrelated := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: cell.Namespace}}
	if requests := reconciler.mapDerivedAccessObject(context.Background(), unrelated); len(requests) != 0 {
		t.Fatalf("unrelated Role mapped to %#v", requests)
	}
}

func requestFor(cell client.Object) ctrl.Request {
	return ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cell)}
}

func environmentValue(workload *appsv1.StatefulSet, name string) string {
	for _, variable := range workload.Spec.Template.Spec.Containers[0].Env {
		if variable.Name == name {
			return variable.Value
		}
	}
	return ""
}
