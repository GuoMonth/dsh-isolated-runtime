package scheduling

import (
	"context"
	"testing"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime/kubernetes"
)

func TestFirstFitUsesLiveInventory(t *testing.T) {
	inventory := kubernetes.NewMemory()
	_, _ = inventory.Create(context.Background(), runtime.Spec{Name: "existing", Tenant: "tenant-a", Image: "image", SecurityClass: runtime.SecurityStandard})
	allocator := &FirstFit{Inventory: inventory}
	got, err := allocator.Allocate(context.Background(), Request{Tenant: "tenant-a", Session: "s1", DesiredRuntimeName: "new", AllowReuse: true, Image: "image", SecurityClass: runtime.SecurityStandard})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Reuse || got.RuntimeName != "existing" {
		t.Fatalf("allocation=%#v", got)
	}
}
