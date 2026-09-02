package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestSchemeRegistersOnlyCellResources(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"Cell", "CellList"} {
		if _, err := scheme.New(GroupVersion.WithKind(kind)); err != nil {
			t.Fatalf("scheme.New(%s): %v", kind, err)
		}
	}
	for _, removed := range []string{"Runtime", "RuntimeSession", "Checkpoint", "CellRevision"} {
		if _, err := scheme.New(GroupVersion.WithKind(removed)); err == nil {
			t.Fatalf("removed kind %s remains registered", removed)
		}
	}
}

func TestCellDeepCopyDoesNotAliasMutableState(t *testing.T) {
	cell := &Cell{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "tenant-a"},
		Status:     CellStatus{Conditions: []metav1.Condition{{Type: ConditionReady}}},
	}
	copy := cell.DeepCopy()
	copy.Status.Conditions[0].Type = ConditionAccessReady
	if cell.Status.Conditions[0].Type != ConditionReady {
		t.Fatal("DeepCopy aliased conditions")
	}
}
