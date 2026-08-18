// Package runtimetest provides the shared contract suite every runtime backend
// must pass.
package runtimetest

import (
	"context"
	"errors"
	"testing"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

// Factory returns a fresh backend for each contract run.
type Factory func() runtime.Runtime

// AssertRuntimeContract verifies the tenant-ownership and lifecycle invariants
// shared by all runtime backends.
func AssertRuntimeContract(t *testing.T, factory Factory) {
	t.Helper()

	t.Run("requires owner and name", func(t *testing.T) {
		rt := factory()
		ctx := context.Background()

		if _, err := rt.Create(ctx, runtime.Spec{Name: "r1"}); !errors.Is(err, runtime.ErrInvalidSpec) {
			t.Fatalf("missing tenant err = %v, want ErrInvalidSpec", err)
		}
		if _, err := rt.Create(ctx, runtime.Spec{Tenant: "tenant-a"}); !errors.Is(err, runtime.ErrInvalidSpec) {
			t.Fatalf("missing name err = %v, want ErrInvalidSpec", err)
		}
	})

	t.Run("owner is immutable and foreign tenants cannot enumerate", func(t *testing.T) {
		rt := factory()
		ctx := context.Background()

		created, err := rt.Create(ctx, runtime.Spec{Name: "runtime-1", Tenant: "tenant-a"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if created.Tenant != "tenant-a" {
			t.Fatalf("tenant = %q, want tenant-a", created.Tenant)
		}

		got, err := rt.Get(ctx, "tenant-a", "runtime-1")
		if err != nil {
			t.Fatalf("owner get: %v", err)
		}
		if got.Name != "runtime-1" || got.Tenant != "tenant-a" {
			t.Fatalf("got = %#v", got)
		}

		if _, err := rt.Get(ctx, "tenant-b", "runtime-1"); !errors.Is(err, runtime.ErrNotFound) {
			t.Fatalf("foreign get err = %v, want ErrNotFound", err)
		}
		if err := rt.Delete(ctx, "tenant-b", "runtime-1"); !errors.Is(err, runtime.ErrNotFound) {
			t.Fatalf("foreign delete err = %v, want ErrNotFound", err)
		}

		if _, err := rt.Create(ctx, runtime.Spec{Name: "runtime-1", Tenant: "tenant-b"}); !errors.Is(err, runtime.ErrConflict) {
			t.Fatalf("cross-tenant recreate err = %v, want ErrConflict", err)
		}

		if _, err := rt.Get(ctx, "tenant-a", "runtime-1"); err != nil {
			t.Fatalf("foreign operations changed owner runtime: %v", err)
		}
	})

	t.Run("owner can delete", func(t *testing.T) {
		rt := factory()
		ctx := context.Background()

		if _, err := rt.Create(ctx, runtime.Spec{Name: "runtime-1", Tenant: "tenant-a"}); err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := rt.Delete(ctx, "tenant-a", "runtime-1"); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := rt.Get(ctx, "tenant-a", "runtime-1"); !errors.Is(err, runtime.ErrNotFound) {
			t.Fatalf("get after delete err = %v, want ErrNotFound", err)
		}
	})
}
