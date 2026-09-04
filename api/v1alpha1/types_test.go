package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestSchemeRegistersCellResources(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"Cell", "CellList", "CellSnapshot", "CellSnapshotList"} {
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

func TestCellSnapshotDeepCopyDoesNotAliasMutableState(t *testing.T) {
	size := resource.MustParse("1Gi")
	snapshot := &CellSnapshot{
		Status: CellSnapshotStatus{
			Conditions:  []metav1.Condition{{Type: ConditionSnapshotReady}},
			RestoreSize: &size,
		},
	}
	copy := snapshot.DeepCopy()
	copy.Status.Conditions[0].Type = ConditionSnapshotFailed
	copy.Status.RestoreSize.Set(2)
	if snapshot.Status.Conditions[0].Type != ConditionSnapshotReady || snapshot.Status.RestoreSize.Value() == 2 {
		t.Fatal("CellSnapshot DeepCopy aliased mutable status")
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
