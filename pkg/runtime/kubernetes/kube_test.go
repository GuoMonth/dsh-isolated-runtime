package kubernetes

import (
	"context"
	"errors"
	"testing"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

// TestBackendSatisfiesContract is the contract test for the runtime seam (①).
// Any backend must pass it — this is what proves "runtime-agnostic".
func TestBackendSatisfiesContract(t *testing.T) {
	var rt runtime.Runtime = New()
	ctx := context.Background()

	info, err := rt.Create(ctx, runtime.Spec{Name: "tenant-a"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if info.Name != "tenant-a" {
		t.Fatalf("name = %q, want tenant-a", info.Name)
	}

	got, err := rt.Get(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Phase != "Running" {
		t.Fatalf("phase = %q, want Running", got.Phase)
	}

	if _, err := rt.Create(ctx, runtime.Spec{Name: "tenant-a"}); !errors.Is(err, runtime.ErrConflict) {
		t.Fatalf("second create err = %v, want ErrConflict", err)
	}

	ref, err := rt.Checkpoint(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	restored, err := rt.Restore(ctx, "tenant-a", ref)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Phase != "Running" {
		t.Fatalf("restored phase = %q, want Running", restored.Phase)
	}

	if err := rt.Delete(ctx, "tenant-a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := rt.Get(ctx, "tenant-a"); !errors.Is(err, runtime.ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
}
