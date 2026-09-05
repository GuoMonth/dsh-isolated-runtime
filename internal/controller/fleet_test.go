package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

func TestFleetConcurrencyDefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	for configured, want := range map[int]int{-1: 1, 0: 1, 1: 1, 7: 7} {
		if got := normalizedConcurrency(configured); got != want {
			t.Fatalf("normalizedConcurrency(%d) = %d, want %d", configured, got, want)
		}
	}
}

func TestReadyProgressIsWatchDriven(t *testing.T) {
	t.Parallel()
	cell := testCell("watch-driven", dshv1alpha1.RetentionRetain)
	reconciler, _ := testReconciler(t, cell)
	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cell)})
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("initial non-Ready reconcile = %#v, %v; want watch-driven zero result", result, err)
	}
}

func TestForeignManagedObjectDeletionMapsBackToCell(t *testing.T) {
	t.Parallel()
	cell := testCell("foreign-watch", dshv1alpha1.RetentionRetain)
	reconciler, _ := testReconciler(t, cell)
	names := cellcontract.ResourceNames(string(cell.UID))
	foreign := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace}}
	requests := reconciler.mapManagedObject(context.Background(), foreign)
	if len(requests) != 1 || requests[0].NamespacedName != client.ObjectKeyFromObject(cell) {
		t.Fatalf("foreign collision mapped to %#v", requests)
	}
}

func TestSnapshotWakeupsAreReasonSpecific(t *testing.T) {
	t.Parallel()
	for reason, want := range map[string]ctrl.Result{
		reasonSnapshotSourceNotFound:  {},
		reasonSnapshotSourceNotReady:  {},
		reasonSnapshotOperationQueued: {},
		reasonSnapshotSupportDisabled: {},
		reasonSnapshotClassMissing:    {RequeueAfter: externalPrerequisiteRequeue},
		reasonSnapshotDriverMismatch:  {RequeueAfter: externalPrerequisiteRequeue},
	} {
		if got := pendingSnapshotResult(reason); got != want {
			t.Fatalf("pendingSnapshotResult(%q) = %#v, want %#v", reason, got, want)
		}
	}

	if got := timeoutResult(25*time.Second, time.Minute); got.RequeueAfter != 35*time.Second {
		t.Fatalf("timeoutResult before deadline = %#v", got)
	}
	if got := timeoutResult(time.Minute, time.Minute); got.RequeueAfter != time.Nanosecond {
		t.Fatalf("timeoutResult at deadline = %#v", got)
	}
}

func TestControllerRuntimeDefaultFailureBackoff(t *testing.T) {
	t.Parallel()
	limiter := workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]()
	request := reconcile.Request{NamespacedName: client.ObjectKey{Namespace: "tenant", Name: "cell"}}
	for attempt, want := range []time.Duration{5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond} {
		if got := limiter.When(request); got != want {
			t.Fatalf("attempt %d delay = %s, want %s", attempt+1, got, want)
		}
	}
	var capped time.Duration
	for range 30 {
		capped = limiter.When(request)
	}
	if capped != 1000*time.Second {
		t.Fatalf("capped delay = %s, want 1000s", capped)
	}
	limiter.Forget(request)
	if got := limiter.When(request); got != 5*time.Millisecond {
		t.Fatalf("delay after Forget = %s, want 5ms", got)
	}
}

func TestTransientAPIErrorsAreReturnedForWorkqueueBackoff(t *testing.T) {
	t.Parallel()
	tests := map[string]error{
		"429": apierrors.NewTooManyRequests("injected", 1),
		"5xx": apierrors.NewServiceUnavailable("injected"),
	}
	for name, injected := range tests {
		t.Run(name, func(t *testing.T) {
			cell := testCell("api-pressure", dshv1alpha1.RetentionRetain)
			reconciler, kube := testReconciler(t, cell)
			reconciler.Client = interceptor.NewClient(kube.(client.WithWatch), interceptor.Funcs{
				Get: func(ctx context.Context, inner client.WithWatch, key client.ObjectKey, object client.Object, options ...client.GetOption) error {
					if _, ok := object.(*dshv1alpha1.Cell); ok {
						return injected
					}
					return inner.Get(ctx, key, object, options...)
				},
			})
			result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cell)})
			if err == nil || result != (ctrl.Result{}) {
				t.Fatalf("Reconcile() = %#v, %v; want zero result and API error", result, err)
			}
			if apierrors.ReasonForError(err) != apierrors.ReasonForError(injected) {
				t.Fatalf("error reason = %s, want %s", apierrors.ReasonForError(err), apierrors.ReasonForError(injected))
			}
		})
	}
}

func TestRestoreCellWatchWakesSnapshotFinalizer(t *testing.T) {
	t.Parallel()
	source := testCell("source", dshv1alpha1.RetentionRetain)
	snapshot := &dshv1alpha1.CellSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: source.Namespace, UID: "snapshot-uid"},
		Spec:       dshv1alpha1.CellSnapshotSpec{CellRef: dshv1alpha1.LocalCellReference{Name: source.Name}, VolumeSnapshotClassName: "snapclass"},
	}
	restore := testCell("restore", dshv1alpha1.RetentionRetain)
	restore.Spec.Storage.RestoreFrom = &dshv1alpha1.LocalCellSnapshotReference{Name: snapshot.Name}
	reconciler, _ := testReconciler(t, source, snapshot, restore)
	requests := (&CellSnapshotReconciler{Client: reconciler.Client}).mapCellToSnapshots(context.Background(), restore)
	if len(requests) != 1 || requests[0].NamespacedName != client.ObjectKeyFromObject(snapshot) {
		t.Fatalf("restore Cell mapped to %#v", requests)
	}
}
