package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/accesscontract"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

func TestPlatformAccessCreatesOnlyPrivateResources(t *testing.T) {
	t.Parallel()
	cell := testCell("main", "Retain")
	r, kube := testReconciler(t, cell)
	r.RouteConfig = accesscontract.Config{Mode: accesscontract.ModePlatform, BaseDomain: "cells.test"}
	r.routeAPIAvailable = true
	reconcileCell(t, r, cell)
	reconcileCell(t, r, cell)
	names := cellcontract.ResourceNames(string(cell.UID))
	workload := get[*appsv1.StatefulSet](t, kube, cell.Namespace, names.Base)
	if environmentValue(workload, "CELL_AUTHORITY") != r.RouteConfig.Authority(string(cell.UID)) {
		t.Fatal("wrong public authority")
	}
	if workload.Annotations[accessModeAnnotation] != "platform" {
		t.Fatal("missing mode ownership")
	}
	policy := get[*networkingv1.NetworkPolicy](t, kube, cell.Namespace, names.Base)
	peer := policy.Spec.Ingress[0].From[0]
	if peer.PodSelector.MatchLabels[cellcontract.AccessLabel] != "platform" || peer.NamespaceSelector.MatchLabels[corev1.LabelMetadataName] != r.SystemNamespace {
		t.Fatal("platform ingress was not isolated")
	}
	for _, obj := range []client.Object{&rbacv1.Role{}, &gatewayv1.HTTPRoute{}} {
		name := names.Base
		if _, ok := obj.(*rbacv1.Role); ok {
			name += "-access"
		}
		if err := kube.Get(context.Background(), client.ObjectKey{Namespace: cell.Namespace, Name: name}, obj); !apierrors.IsNotFound(err) {
			t.Fatalf("unexpected public resource: %T: %v", obj, err)
		}
	}
	// Mode changes must not silently rewrite a running Cell.
	r.RouteConfig = accesscontract.Config{}
	reconcileCell(t, r, cell)
	unchanged := get[*appsv1.StatefulSet](t, kube, cell.Namespace, names.Base)
	if unchanged.ResourceVersion != workload.ResourceVersion {
		t.Fatal("mode switch changed workload")
	}
	updated := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	found := false
	for _, c := range updated.Status.Conditions {
		if c.Reason == "AccessModeConflict" {
			found = true
		}
	}
	if !found {
		t.Fatal("mode conflict was not diagnosed")
	}
}

func TestPlatformConflictDoesNotMutateResources(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"role", "route", "aliased-route", "workload"} {
		t.Run(kind, func(t *testing.T) {
			cell := testCell("main", "Retain")
			names := cellcontract.ResourceNames(string(cell.UID))
			var object client.Object
			switch kind {
			case "role":
				object = &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: names.Base + "-access", Namespace: cell.Namespace}}
			case "workload":
				object = &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}}
			default:
				route := &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}}
				if kind == "aliased-route" {
					route.Name = "custom-route"
					route.Spec.Rules = []gatewayv1.HTTPRouteRule{{BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: gatewayv1.ObjectName(names.Base)}}}}}}
				}
				object = route
			}
			r, kube := testReconciler(t, cell, object)
			before := object.DeepCopyObject().(client.Object)
			if err := kube.Get(context.Background(), client.ObjectKeyFromObject(object), before); err != nil {
				t.Fatal(err)
			}
			r.RouteConfig = accesscontract.Config{Mode: accesscontract.ModePlatform, BaseDomain: "cells.test"}
			r.routeAPIAvailable = true
			reconcileCell(t, r, cell)
			after := object.DeepCopyObject().(client.Object)
			if err := kube.Get(context.Background(), client.ObjectKeyFromObject(object), after); err != nil {
				t.Fatal(err)
			}
			if before.GetResourceVersion() != after.GetResourceVersion() {
				t.Fatal("conflicting resource changed")
			}
			var claims corev1.PersistentVolumeClaimList
			if err := kube.List(context.Background(), &claims); err != nil {
				t.Fatal(err)
			}
			if len(claims.Items) != 0 {
				t.Fatal("conflict was detected after storage mutation")
			}
			updated := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
			found := false
			for _, c := range updated.Status.Conditions {
				if c.Reason == "AccessModeConflict" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing conflict diagnostic")
			}
		})
	}
}

func TestPlatformRequiresRouteInspection(t *testing.T) {
	t.Parallel()
	cell := testCell("main", "Retain")
	r, kube := testReconciler(t, cell)
	r.RouteConfig = accesscontract.Config{Mode: accesscontract.ModePlatform, BaseDomain: "cells.test"}
	if _, err := r.Reconcile(context.Background(), requestFor(cell)); err == nil {
		t.Fatal("missing route discovery accepted")
	}
	var claims corev1.PersistentVolumeClaimList
	if err := kube.List(context.Background(), &claims); err != nil {
		t.Fatal(err)
	}
	if len(claims.Items) != 0 {
		t.Fatal("storage mutated without route inspection")
	}
}

func TestAliasedRouteChangesWakeCell(t *testing.T) {
	t.Parallel()
	cell := testCell("main", "Retain")
	r, _ := testReconciler(t, cell)
	names := cellcontract.ResourceNames(string(cell.UID))
	route := &gatewayv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: "custom-route", Namespace: cell.Namespace}}
	route.Spec.Rules = []gatewayv1.HTTPRouteRule{{BackendRefs: []gatewayv1.HTTPBackendRef{{BackendRef: gatewayv1.BackendRef{BackendObjectReference: gatewayv1.BackendObjectReference{Name: gatewayv1.ObjectName(names.Base)}}}}}}
	requests := r.mapDerivedAccessObject(context.Background(), route)
	if len(requests) != 1 || requests[0].NamespacedName != client.ObjectKeyFromObject(cell) {
		t.Fatal("aliased route conflict cannot wake Cell")
	}
}
