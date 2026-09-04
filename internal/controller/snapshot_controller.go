package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

const (
	reasonSnapshotSupportDisabled = "SnapshotSupportDisabled"
	reasonSnapshotPrerequisites   = "PrerequisitesSatisfied"
	reasonSnapshotSourceNotFound  = "SourceNotFound"
	reasonSnapshotSourceNotReady  = "SourceNotReady"
	reasonSnapshotClassMissing    = "SnapshotClassUnavailable"
	reasonSnapshotDriverMismatch  = "SnapshotClassDriverMismatch"
	reasonSnapshotOperationQueued = "OperationQueued"
	reasonSnapshotQuiescePending  = "QuiescePending"
	reasonSnapshotQuiesced        = "Quiesced"
	reasonSnapshotPending         = "SnapshotPending"
	reasonSnapshotReady           = "SnapshotReady"
	reasonSnapshotFailed          = "SnapshotFailed"
	reasonSnapshotSourceChanged   = "SourceChanged"
	reasonSnapshotTimedOut        = "SnapshotTimedOut"
	reasonSnapshotCleanupPending  = "CleanupPending"
	reasonSnapshotCleanupBlocked  = "CleanupBlocked"
)

var errSnapshotSourceChanged = errors.New("snapshot source changed")

// SnapshotConfig controls the optional CSI snapshot capability. It is cluster
// policy rather than Cell intent.
type SnapshotConfig struct {
	Enabled         bool
	QuiesceTimeout  time.Duration
	SnapshotTimeout time.Duration
	HTTPClient      *http.Client
}

type quiesceRejectedError struct {
	status int
}

func (e *quiesceRejectedError) Error() string {
	return fmt.Sprintf("launcher rejected quiesce with status %d", e.status)
}

func (c SnapshotConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.QuiesceTimeout < 30*time.Second {
		return errors.New("quiesce timeout must be at least 30 seconds")
	}
	if c.SnapshotTimeout < time.Minute {
		return errors.New("snapshot timeout must be at least one minute")
	}
	return nil
}

// CellSnapshotReconciler translates the narrow CellSnapshot contract into one
// CSI VolumeSnapshot after exact launcher quiesce acknowledgement.
type CellSnapshotReconciler struct {
	client.Client
	APIReader client.Reader
	Scheme    *runtime.Scheme
	Config    SnapshotConfig
}

// +kubebuilder:rbac:groups=dsh.isolated.io,resources=cellsnapshots,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=dsh.isolated.io,resources=cellsnapshots/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dsh.isolated.io,resources=cells,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshotclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;delete

func (r *CellSnapshotReconciler) SetupWithManager(manager ctrl.Manager) error {
	if err := r.Config.Validate(); err != nil {
		return err
	}
	builder := ctrl.NewControllerManagedBy(manager).
		For(&dshv1alpha1.CellSnapshot{}).
		Watches(&dshv1alpha1.Cell{}, handler.EnqueueRequestsFromMapFunc(r.mapCellToSnapshots)).
		Watches(&appsv1.StatefulSet{}, handler.EnqueueRequestsFromMapFunc(r.mapManagedToSnapshots))
	if r.Config.Enabled {
		for _, kind := range []schema.GroupKind{
			{Group: volumesnapshotv1.GroupName, Kind: "VolumeSnapshot"},
			{Group: volumesnapshotv1.GroupName, Kind: "VolumeSnapshotClass"},
			{Group: volumesnapshotv1.GroupName, Kind: "VolumeSnapshotContent"},
		} {
			if _, err := manager.GetRESTMapper().RESTMapping(kind, volumesnapshotv1.SchemeGroupVersion.Version); err != nil {
				return fmt.Errorf("snapshot support requires the %s CRD: %w", kind.Kind, err)
			}
		}
		builder = builder.Owns(&volumesnapshotv1.VolumeSnapshot{})
	}
	return builder.Complete(r)
}

func (r *CellSnapshotReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var snapshot dshv1alpha1.CellSnapshot
	if err := r.Get(ctx, request.NamespacedName, &snapshot); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if snapshot.UID == "" {
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}

	if snapshot.DeletionTimestamp != nil {
		return r.reconcileDeleting(ctx, &snapshot)
	}
	if !r.Config.Enabled {
		return r.setPending(ctx, &snapshot, reasonSnapshotSupportDisabled, "snapshot support is disabled")
	}
	if failureCleanupRequired(snapshot.Status.Conditions) {
		return r.completeFailure(ctx, &snapshot)
	}
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotReady) {
		return r.completeOperation(ctx, &snapshot, false)
	}
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotFailed) {
		return r.completeOperation(ctx, &snapshot, false)
	}
	if !conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) {
		return r.accept(ctx, &snapshot)
	}

	cell, err := r.acceptedSource(ctx, &snapshot)
	if err != nil {
		if errors.Is(err, errSnapshotSourceChanged) {
			return r.fail(ctx, &snapshot, reasonSnapshotSourceChanged, "source Cell changed during snapshot")
		}
		return ctrl.Result{}, err
	}
	if !conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotQuiesced) {
		return r.quiesce(ctx, &snapshot, cell)
	}
	return r.snapshot(ctx, &snapshot, cell)
}

func (r *CellSnapshotReconciler) accept(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (ctrl.Result, error) {
	var cell dshv1alpha1.Cell
	key := types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.Spec.CellRef.Name}
	if err := r.read(ctx, key, &cell); err != nil {
		if apierrors.IsNotFound(err) {
			return r.setPending(ctx, snapshot, reasonSnapshotSourceNotFound, "source Cell does not exist")
		}
		return ctrl.Result{}, err
	}
	if active := cell.Annotations[cellcontract.ActiveSnapshotAnnotation]; active != "" && active != string(snapshot.UID) {
		return r.setPending(ctx, snapshot, reasonSnapshotOperationQueued, "another CellSnapshot is active")
	}
	ready := meta.FindStatusCondition(cell.Status.Conditions, dshv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue || cell.Status.ObservedGeneration != cell.Generation || cell.Status.DSHVersion != cellcontract.DSHVersion || cell.Status.ImageDigest == "" {
		return r.setPending(ctx, snapshot, reasonSnapshotSourceNotReady, "source Cell current generation is not ready")
	}

	names := cellcontract.ResourceNames(string(cell.UID))
	var data corev1.PersistentVolumeClaim
	if err := r.Get(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.DataPVC}, &data); err != nil {
		return r.setPending(ctx, snapshot, reasonSnapshotSourceNotReady, "source data PVC is not ready")
	}
	if data.Status.Phase != corev1.ClaimBound || data.Spec.StorageClassName == nil || *data.Spec.StorageClassName == "" {
		return r.setPending(ctx, snapshot, reasonSnapshotSourceNotReady, "source data PVC is not bound to a StorageClass")
	}
	var storageClass storagev1.StorageClass
	if err := r.Get(ctx, types.NamespacedName{Name: *data.Spec.StorageClassName}, &storageClass); err != nil {
		return r.setPending(ctx, snapshot, reasonSnapshotSourceNotReady, "source StorageClass is unavailable")
	}
	var snapshotClass volumesnapshotv1.VolumeSnapshotClass
	if err := r.Get(ctx, types.NamespacedName{Name: snapshot.Spec.VolumeSnapshotClassName}, &snapshotClass); err != nil {
		if apierrors.IsNotFound(err) {
			return r.setPending(ctx, snapshot, reasonSnapshotClassMissing, "VolumeSnapshotClass is unavailable")
		}
		return ctrl.Result{}, err
	}
	if snapshotClass.Driver != storageClass.Provisioner {
		return r.setPending(ctx, snapshot, reasonSnapshotDriverMismatch, "snapshot and storage classes use different CSI drivers")
	}

	if !controllerutil.ContainsFinalizer(snapshot, cellcontract.SnapshotFinalizer) {
		copy := snapshot.DeepCopy()
		controllerutil.AddFinalizer(copy, cellcontract.SnapshotFinalizer)
		if err := r.Update(ctx, copy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}
	if active := cell.Annotations[cellcontract.ActiveSnapshotAnnotation]; active == "" {
		if err := r.patchCellOperation(ctx, &cell, string(snapshot.UID)); err != nil {
			return ctrl.Result{}, err
		}
	}

	statusErr := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
		status.SourceCellUID = string(cell.UID)
		status.SourceGeneration = cell.Generation
		status.DSHVersion = cell.Status.DSHVersion
		status.ImageDigest = cell.Status.ImageDigest
		status.StorageClassName = *data.Spec.StorageClassName
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotAccepted, metav1.ConditionTrue, reasonSnapshotPrerequisites, "snapshot prerequisites are satisfied")
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotQuiesced, metav1.ConditionFalse, reasonSnapshotQuiescePending, "waiting for launcher quiesce")
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotReady, metav1.ConditionFalse, reasonSnapshotPending, "waiting for CSI snapshot")
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reasonSnapshotPending, "snapshot has not failed")
	})
	return ctrl.Result{RequeueAfter: pendingRequeue}, statusErr
}

func (r *CellSnapshotReconciler) acceptedSource(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (*dshv1alpha1.Cell, error) {
	cell, err := r.operationSource(ctx, snapshot)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errSnapshotSourceChanged
		}
		return nil, err
	}
	if cell.Generation != snapshot.Status.SourceGeneration || imageDigest(cell.Spec.Image) != snapshot.Status.ImageDigest {
		return nil, errSnapshotSourceChanged
	}
	return cell, nil
}

func (r *CellSnapshotReconciler) operationSource(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (*dshv1alpha1.Cell, error) {
	var cell dshv1alpha1.Cell
	if err := r.read(ctx, types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.Spec.CellRef.Name}, &cell); err != nil {
		return nil, err
	}
	if string(cell.UID) != snapshot.Status.SourceCellUID {
		return nil, errSnapshotSourceChanged
	}
	if cell.Annotations[cellcontract.ActiveSnapshotAnnotation] != string(snapshot.UID) {
		return nil, errSnapshotSourceChanged
	}
	return &cell, nil
}

func (r *CellSnapshotReconciler) quiesce(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, cell *dshv1alpha1.Cell) (ctrl.Result, error) {
	if conditionAge(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) > r.Config.QuiesceTimeout {
		return r.fail(ctx, snapshot, reasonSnapshotTimedOut, "Cell quiesce timed out")
	}
	pod, err := r.workloadPod(ctx, cell)
	if err != nil {
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}
	if err := r.requestQuiesce(ctx, pod, string(snapshot.UID)); err != nil {
		var rejected *quiesceRejectedError
		if errors.As(err, &rejected) {
			return r.fail(ctx, snapshot, reasonSnapshotFailed, "launcher rejected the quiesce operation")
		}
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}
	err = r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotQuiesced, metav1.ConditionTrue, reasonSnapshotQuiesced, "launcher drained traffic and DSH exited normally")
	})
	return ctrl.Result{RequeueAfter: pendingRequeue}, err
}

func (r *CellSnapshotReconciler) snapshot(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, cell *dshv1alpha1.Cell) (ctrl.Result, error) {
	var workload appsv1.StatefulSet
	names := cellcontract.ResourceNames(string(cell.UID))
	if err := r.Get(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.Base}, &workload); err != nil {
		return ctrl.Result{}, err
	}
	if !statefulSetStopped(&workload) {
		if conditionAge(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) > r.Config.QuiesceTimeout {
			return r.fail(ctx, snapshot, reasonSnapshotTimedOut, "StatefulSet did not stop before the quiesce deadline")
		}
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}

	volumeSnapshot, err := r.ensureVolumeSnapshot(ctx, snapshot, cell, names.DataPVC)
	if err != nil {
		return r.fail(ctx, snapshot, reasonSnapshotFailed, "CSI snapshot reconciliation failed")
	}
	if volumeSnapshot.Status != nil && volumeSnapshot.Status.Error != nil {
		return r.fail(ctx, snapshot, reasonSnapshotFailed, "CSI reported a snapshot error")
	}
	if !volumeSnapshot.CreationTimestamp.IsZero() && time.Since(volumeSnapshot.CreationTimestamp.Time) > r.Config.SnapshotTimeout {
		return r.fail(ctx, snapshot, reasonSnapshotTimedOut, "CSI snapshot timed out")
	}
	if volumeSnapshot.Status == nil || volumeSnapshot.Status.ReadyToUse == nil || !*volumeSnapshot.Status.ReadyToUse || volumeSnapshot.Status.RestoreSize == nil {
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}
	restoreSize := volumeSnapshot.Status.RestoreSize.DeepCopy()
	err = r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
		status.RestoreSize = &restoreSize
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotReady, metav1.ConditionTrue, reasonSnapshotReady, "CSI snapshot is ready to restore")
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	return r.completeOperation(ctx, snapshot, false)
}

func (r *CellSnapshotReconciler) fail(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, reason, message string) (ctrl.Result, error) {
	err := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
		// Ready retains the terminal cause while Failed remains false until an
		// incomplete CSI object is proven absent. This makes Failed=True durable
		// evidence that resuming the source cannot race backend snapshot work.
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotReady, metav1.ConditionFalse, reason, message)
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reasonSnapshotCleanupPending, "confirming incomplete CSI snapshot cleanup")
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	return r.completeFailure(ctx, snapshot)
}

func (r *CellSnapshotReconciler) completeFailure(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (ctrl.Result, error) {
	gone, cleanupErr := r.deleteVolumeSnapshot(ctx, snapshot)
	if cleanupErr != nil {
		statusErr := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
			setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reasonSnapshotCleanupBlocked, "incomplete CSI snapshot cleanup is blocked")
		})
		return ctrl.Result{RequeueAfter: pendingRequeue}, errors.Join(cleanupErr, statusErr)
	}
	if !gone {
		statusErr := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
			setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reasonSnapshotCleanupPending, "waiting for incomplete CSI snapshot deletion")
		})
		return ctrl.Result{RequeueAfter: pendingRequeue}, statusErr
	}

	cause := meta.FindStatusCondition(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotReady)
	reason, message := reasonSnapshotFailed, "snapshot failed"
	if cause != nil && cause.Reason != "" {
		reason, message = cause.Reason, cause.Message
	}
	if err := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionTrue, reason, message)
	}); err != nil {
		return ctrl.Result{}, err
	}
	return r.completeOperation(ctx, snapshot, false)
}

func (r *CellSnapshotReconciler) completeOperation(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, cleanup bool) (ctrl.Result, error) {
	if cleanup {
		gone, err := r.deleteVolumeSnapshot(ctx, snapshot)
		if err != nil {
			return ctrl.Result{RequeueAfter: pendingRequeue}, err
		}
		if !gone {
			return ctrl.Result{RequeueAfter: pendingRequeue}, nil
		}
	}
	if err := r.releaseCell(ctx, snapshot); err != nil {
		return ctrl.Result{}, err
	}
	if controllerutil.ContainsFinalizer(snapshot, cellcontract.SnapshotFinalizer) {
		copy := snapshot.DeepCopy()
		controllerutil.RemoveFinalizer(copy, cellcontract.SnapshotFinalizer)
		if err := r.Update(ctx, copy); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *CellSnapshotReconciler) reconcileDeleting(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(snapshot, cellcontract.SnapshotFinalizer) {
		return ctrl.Result{}, nil
	}
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotReady) {
		return r.completeOperation(ctx, snapshot, false)
	}
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotFailed) {
		return r.completeOperation(ctx, snapshot, false)
	}
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) &&
		!conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotQuiesced) {
		cell, err := r.operationSource(ctx, snapshot)
		if apierrors.IsNotFound(err) {
			return r.completeOperation(ctx, snapshot, true)
		}
		if err != nil {
			return ctrl.Result{RequeueAfter: pendingRequeue}, err
		}
		pod, err := r.workloadPod(ctx, cell)
		if err != nil {
			return ctrl.Result{RequeueAfter: pendingRequeue}, nil
		}
		if err := r.requestQuiesce(ctx, pod, string(snapshot.UID)); err != nil {
			return ctrl.Result{RequeueAfter: pendingRequeue}, nil
		}
		err = r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
			setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotQuiesced, metav1.ConditionTrue, reasonSnapshotQuiesced, "launcher drained traffic and DSH exited normally")
		})
		return ctrl.Result{RequeueAfter: pendingRequeue}, err
	}
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotQuiesced) {
		cell, err := r.operationSource(ctx, snapshot)
		if err == nil {
			var workload appsv1.StatefulSet
			names := cellcontract.ResourceNames(string(cell.UID))
			if err := r.Get(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.Base}, &workload); err != nil {
				return ctrl.Result{RequeueAfter: pendingRequeue}, client.IgnoreNotFound(err)
			}
			if !statefulSetStopped(&workload) {
				return ctrl.Result{RequeueAfter: pendingRequeue}, nil
			}
		} else if !apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: pendingRequeue}, err
		}
	}
	return r.completeOperation(ctx, snapshot, true)
}

func (r *CellSnapshotReconciler) setPending(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, reason, message string) (ctrl.Result, error) {
	err := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotAccepted, metav1.ConditionFalse, reason, message)
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotQuiesced, metav1.ConditionFalse, reasonSnapshotQuiescePending, "snapshot has not quiesced the source")
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotReady, metav1.ConditionFalse, reasonSnapshotPending, "snapshot is not ready")
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reasonSnapshotPending, "snapshot has not failed")
	})
	return ctrl.Result{RequeueAfter: pendingRequeue}, err
}

func (r *CellSnapshotReconciler) ensureVolumeSnapshot(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, cell *dshv1alpha1.Cell, claimName string) (*volumesnapshotv1.VolumeSnapshot, error) {
	name := cellcontract.SnapshotName(string(snapshot.UID))
	object := &volumesnapshotv1.VolumeSnapshot{}
	key := types.NamespacedName{Namespace: snapshot.Namespace, Name: name}
	err := r.Get(ctx, key, object)
	if apierrors.IsNotFound(err) {
		object = &volumesnapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: snapshot.Namespace,
				Labels: map[string]string{cellcontract.ManagedByLabel: cellcontract.ManagedByValue},
				Annotations: map[string]string{
					cellcontract.SnapshotUIDAnnotation: string(snapshot.UID),
					cellcontract.CellUIDAnnotation:     string(cell.UID),
				},
			},
			Spec: volumesnapshotv1.VolumeSnapshotSpec{
				VolumeSnapshotClassName: &snapshot.Spec.VolumeSnapshotClassName,
				Source:                  volumesnapshotv1.VolumeSnapshotSource{PersistentVolumeClaimName: &claimName},
			},
		}
		if err := controllerutil.SetControllerReference(snapshot, object, r.Scheme); err != nil {
			return nil, err
		}
		if err := r.Create(ctx, object); err != nil {
			return nil, err
		}
		return object, nil
	}
	if err != nil {
		return nil, err
	}
	if !snapshotOwnsVolumeSnapshot(snapshot, object) {
		return nil, &ownershipConflictError{resource: "VolumeSnapshot"}
	}
	if object.Spec.Source.PersistentVolumeClaimName == nil || *object.Spec.Source.PersistentVolumeClaimName != claimName || object.Spec.VolumeSnapshotClassName == nil || *object.Spec.VolumeSnapshotClassName != snapshot.Spec.VolumeSnapshotClassName {
		return nil, errors.New("managed VolumeSnapshot spec drifted")
	}
	return object, nil
}

func (r *CellSnapshotReconciler) deleteVolumeSnapshot(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (bool, error) {
	var object volumesnapshotv1.VolumeSnapshot
	err := r.Get(ctx, types.NamespacedName{Namespace: snapshot.Namespace, Name: cellcontract.SnapshotName(string(snapshot.UID))}, &object)
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !snapshotOwnsVolumeSnapshot(snapshot, &object) {
		// A foreign collision was never started by this controller and must not
		// be adopted or deleted.
		return true, nil
	}
	if object.DeletionTimestamp == nil {
		if err := r.Delete(ctx, &object); err != nil {
			return false, err
		}
	}
	return false, nil
}

func snapshotOwnsVolumeSnapshot(snapshot *dshv1alpha1.CellSnapshot, object *volumesnapshotv1.VolumeSnapshot) bool {
	owner := metav1.GetControllerOf(object)
	return owner != nil && owner.UID == snapshot.UID && owner.Kind == "CellSnapshot" &&
		object.Annotations[cellcontract.SnapshotUIDAnnotation] == string(snapshot.UID)
}

func (r *CellSnapshotReconciler) workloadPod(ctx context.Context, cell *dshv1alpha1.Cell) (*corev1.Pod, error) {
	var workload appsv1.StatefulSet
	names := cellcontract.ResourceNames(string(cell.UID))
	if err := r.Get(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.Base}, &workload); err != nil {
		return nil, err
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(cell.Namespace), client.MatchingLabels(workloadSelector(cell))); err != nil {
		return nil, err
	}
	var selected *corev1.Pod
	for index := range pods.Items {
		pod := &pods.Items[index]
		owner := metav1.GetControllerOf(pod)
		if owner == nil || owner.UID != workload.UID || pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" {
			continue
		}
		if selected != nil {
			return nil, errors.New("managed workload has more than one running Pod")
		}
		selected = pod
	}
	if selected == nil {
		return nil, errors.New("managed workload Pod is unavailable")
	}
	return selected, nil
}

func (r *CellSnapshotReconciler) requestQuiesce(ctx context.Context, pod *corev1.Pod, operationUID string) error {
	body, err := json.Marshal(map[string]string{"operationUID": operationUID})
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	endpoint := "http://" + net.JoinHostPort(pod.Status.PodIP, strconv.Itoa(cellcontract.ManagementPort)) + cellcontract.QuiescePath
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("construct launcher quiesce request")
	}
	request.Header.Set("Content-Type", "application/json")
	client := r.Config.HTTPClient
	if client == nil {
		client = &http.Client{Transport: &http.Transport{Proxy: nil}}
	}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("launcher quiesce request failed")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusNoContent {
		if response.StatusCode == http.StatusServiceUnavailable {
			return errors.New("launcher is temporarily unavailable for quiesce")
		}
		return &quiesceRejectedError{status: response.StatusCode}
	}
	return nil
}

func (r *CellSnapshotReconciler) releaseCell(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) error {
	var cell dshv1alpha1.Cell
	key := types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.Spec.CellRef.Name}
	if err := r.read(ctx, key, &cell); err != nil {
		return client.IgnoreNotFound(err)
	}
	if string(cell.UID) != snapshot.Status.SourceCellUID || cell.Annotations[cellcontract.ActiveSnapshotAnnotation] != string(snapshot.UID) {
		return nil
	}
	return r.patchCellOperation(ctx, &cell, "")
}

func (r *CellSnapshotReconciler) patchCellOperation(ctx context.Context, cell *dshv1alpha1.Cell, operationUID string) error {
	original := cell.DeepCopy()
	copy := cell.DeepCopy()
	if operationUID == "" {
		delete(copy.Annotations, cellcontract.ActiveSnapshotAnnotation)
	} else {
		if copy.Annotations == nil {
			copy.Annotations = map[string]string{}
		}
		copy.Annotations[cellcontract.ActiveSnapshotAnnotation] = operationUID
	}
	patch := client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})
	return r.Patch(ctx, copy, patch)
}

// read bypasses the controller-runtime cache when a direct API reader is
// available. The active-snapshot annotation is a CAS lock: a cached read
// immediately after Update is allowed to be stale and must not be interpreted
// as lost ownership or as permission for a second operation to acquire it.
func (r *CellSnapshotReconciler) read(ctx context.Context, key client.ObjectKey, object client.Object) error {
	if r.APIReader != nil {
		return r.APIReader.Get(ctx, key, object)
	}
	return r.Get(ctx, key, object)
}

func (r *CellSnapshotReconciler) patchStatus(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, mutate func(*dshv1alpha1.CellSnapshotStatus)) error {
	original := snapshot.DeepCopy()
	mutate(&snapshot.Status)
	snapshot.Status.ObservedGeneration = snapshot.Generation
	if err := r.Status().Patch(ctx, snapshot, client.MergeFrom(original)); err != nil {
		return err
	}
	return nil
}

func setSnapshotCondition(status *dshv1alpha1.CellSnapshotStatus, generation int64, conditionType string, value metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type: conditionType, Status: value, ObservedGeneration: generation,
		Reason: reason, Message: message,
	})
}

func conditionTrue(conditions []metav1.Condition, conditionType string) bool {
	condition := meta.FindStatusCondition(conditions, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func failureCleanupRequired(conditions []metav1.Condition) bool {
	condition := meta.FindStatusCondition(conditions, dshv1alpha1.ConditionSnapshotFailed)
	return condition != nil && condition.Status == metav1.ConditionFalse &&
		(condition.Reason == reasonSnapshotCleanupPending || condition.Reason == reasonSnapshotCleanupBlocked)
}

func conditionAge(conditions []metav1.Condition, conditionType string) time.Duration {
	condition := meta.FindStatusCondition(conditions, conditionType)
	if condition == nil || condition.LastTransitionTime.IsZero() {
		return 0
	}
	return time.Since(condition.LastTransitionTime.Time)
}

func statefulSetStopped(workload *appsv1.StatefulSet) bool {
	return workload.Spec.Replicas != nil && *workload.Spec.Replicas == 0 &&
		workload.Status.ObservedGeneration >= workload.Generation &&
		workload.Status.Replicas == 0 && workload.Status.ReadyReplicas == 0 &&
		workload.Status.CurrentReplicas == 0 && workload.Status.UpdatedReplicas == 0
}

func (r *CellSnapshotReconciler) mapCellToSnapshots(ctx context.Context, object client.Object) []reconcile.Request {
	var snapshots dshv1alpha1.CellSnapshotList
	if err := r.List(ctx, &snapshots, client.InNamespace(object.GetNamespace())); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0)
	for _, snapshot := range snapshots.Items {
		if snapshot.Spec.CellRef.Name == object.GetName() {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&snapshot)})
		}
	}
	return requests
}

func (r *CellSnapshotReconciler) mapManagedToSnapshots(ctx context.Context, object client.Object) []reconcile.Request {
	cellName := object.GetAnnotations()[cellcontract.CellNameAnnotation]
	if cellName == "" {
		return nil
	}
	return r.mapCellToSnapshots(ctx, &dshv1alpha1.Cell{ObjectMeta: metav1.ObjectMeta{Name: cellName, Namespace: object.GetNamespace()}})
}
