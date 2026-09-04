package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	reasonSnapshotSupportDisabled    = "SnapshotSupportDisabled"
	reasonSnapshotPrerequisites      = "PrerequisitesSatisfied"
	reasonSnapshotSourceNotFound     = "SourceNotFound"
	reasonSnapshotSourceNotReady     = "SourceNotReady"
	reasonSnapshotClassMissing       = "SnapshotClassUnavailable"
	reasonSnapshotDriverMismatch     = "SnapshotClassDriverMismatch"
	reasonSnapshotOperationQueued    = "OperationQueued"
	reasonSnapshotAcquiringLock      = "AcquiringOperation"
	reasonSnapshotWriterStopPending  = "WriterStopPending"
	reasonSnapshotWriterStopped      = "WriterStopped"
	reasonSnapshotWriterStopTimedOut = "WriterStopTimedOut"
	reasonSnapshotPending            = "SnapshotPending"
	reasonSnapshotReady              = "SnapshotReady"
	reasonSnapshotFailed             = "SnapshotFailed"
	reasonSnapshotSourceChanged      = "SourceChanged"
	reasonSnapshotTimedOut           = "SnapshotTimedOut"
	reasonSnapshotCleanupPending     = "CleanupPending"
	reasonSnapshotCleanupBlocked     = "CleanupBlocked"
)

var errSnapshotSourceChanged = errors.New("snapshot source changed")

// SnapshotConfig controls the optional CSI snapshot capability. It is cluster
// policy rather than Cell intent.
type SnapshotConfig struct {
	Enabled           bool
	WriterStopTimeout time.Duration
	SnapshotTimeout   time.Duration
}

func (c SnapshotConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.WriterStopTimeout < 30*time.Second {
		return errors.New("writer stop timeout must be at least 30 seconds")
	}
	if c.SnapshotTimeout < time.Minute {
		return errors.New("snapshot timeout must be at least one minute")
	}
	return nil
}

// CellSnapshotReconciler translates one operation-UID-bound CellSnapshot into
// a writer-stopped, crash-consistent CSI VolumeSnapshot. Kubernetes scales the
// StatefulSet to zero; DSH process exit is deliberately not treated as a flush
// acknowledgement because the exact supported release cannot provide one.
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
		Watches(&appsv1.StatefulSet{}, handler.EnqueueRequestsFromMapFunc(r.mapManagedToSnapshots)).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.mapManagedToSnapshots))
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
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotReady) ||
		conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotFailed) {
		return ctrl.Result{}, r.releaseCell(ctx, &snapshot)
	}
	if !conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) {
		return r.accept(ctx, &snapshot)
	}

	cell, err := r.acceptedSource(ctx, &snapshot)
	if err != nil {
		if errors.Is(err, errSnapshotSourceChanged) {
			return r.fail(ctx, &snapshot, reasonSnapshotSourceChanged, "source Cell or managed resources changed during snapshot")
		}
		return ctrl.Result{}, err
	}
	if !conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotWriterStopped) {
		return r.waitForWriterStop(ctx, &snapshot, cell)
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
	if cell.DeletionTimestamp != nil {
		return r.setPending(ctx, snapshot, reasonSnapshotSourceNotFound, "source Cell is deleting")
	}
	if active := cell.Annotations[cellcontract.ActiveSnapshotAnnotation]; active != "" && active != string(snapshot.UID) {
		return r.setPending(ctx, snapshot, reasonSnapshotOperationQueued, "another CellSnapshot is active")
	}
	ready := meta.FindStatusCondition(cell.Status.Conditions, dshv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue || cell.Status.ObservedGeneration != cell.Generation ||
		cell.Status.DSHVersion != cellcontract.DSHVersion || cell.Status.ImageDigest == "" {
		return r.setPending(ctx, snapshot, reasonSnapshotSourceNotReady, "source Cell current generation is not ready")
	}

	data, storageClass, _, result, err := r.snapshotPrerequisites(ctx, snapshot, &cell)
	if err != nil || result != nil {
		if result != nil {
			return *result, err
		}
		return ctrl.Result{}, err
	}
	if !controllerutil.ContainsFinalizer(snapshot, cellcontract.SnapshotFinalizer) {
		copy := snapshot.DeepCopy()
		controllerutil.AddFinalizer(copy, cellcontract.SnapshotFinalizer)
		if err := r.Update(ctx, copy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}

	bound := snapshot.Status.SourceCellUID == string(cell.UID) &&
		snapshot.Status.SourcePVCUID == string(data.UID) &&
		snapshot.Status.SourceGeneration == cell.Generation &&
		snapshot.Status.DSHVersion == cell.Status.DSHVersion &&
		snapshot.Status.ImageDigest == cell.Status.ImageDigest &&
		snapshot.Status.StorageClassName == storageClass.Name
	if !bound {
		err := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
			status.SourceCellUID = string(cell.UID)
			status.SourcePVCUID = string(data.UID)
			status.SourceGeneration = cell.Generation
			status.DSHVersion = cell.Status.DSHVersion
			status.ImageDigest = cell.Status.ImageDigest
			status.StorageClassName = storageClass.Name
			setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotAccepted, metav1.ConditionFalse, reasonSnapshotAcquiringLock, "source identity is bound; waiting for operation lock")
			setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotWriterStopped, metav1.ConditionFalse, reasonSnapshotWriterStopPending, "source writer is still running")
			setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotReady, metav1.ConditionFalse, reasonSnapshotPending, "waiting for CSI snapshot")
			setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reasonSnapshotPending, "snapshot has not failed")
		})
		return ctrl.Result{RequeueAfter: pendingRequeue}, err
	}

	// Re-read after status persistence. The lock is acquired only against the
	// exact source UID and generation recorded above.
	if err := r.read(ctx, key, &cell); err != nil {
		return ctrl.Result{}, err
	}
	if string(cell.UID) != snapshot.Status.SourceCellUID || cell.Generation != snapshot.Status.SourceGeneration ||
		imageDigest(cell.Spec.Image) != snapshot.Status.ImageDigest || cell.DeletionTimestamp != nil {
		return r.fail(ctx, snapshot, reasonSnapshotSourceChanged, "source Cell changed before operation lock")
	}
	if _, _, _, validationErr := r.acceptedSnapshotInputs(ctx, snapshot, &cell); validationErr != nil {
		return r.fail(ctx, snapshot, reasonSnapshotSourceChanged, "source storage identity changed before operation lock")
	}
	if active := cell.Annotations[cellcontract.ActiveSnapshotAnnotation]; active == "" {
		if err := r.patchCellOperation(ctx, &cell, string(snapshot.UID)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	} else if active != string(snapshot.UID) {
		return r.setPending(ctx, snapshot, reasonSnapshotOperationQueued, "another CellSnapshot is active")
	}

	err = r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotAccepted, metav1.ConditionTrue, reasonSnapshotPrerequisites, "snapshot prerequisites are satisfied and the source operation is locked")
	})
	return ctrl.Result{RequeueAfter: pendingRequeue}, err
}

func (r *CellSnapshotReconciler) snapshotPrerequisites(
	ctx context.Context,
	snapshot *dshv1alpha1.CellSnapshot,
	cell *dshv1alpha1.Cell,
) (*corev1.PersistentVolumeClaim, *storagev1.StorageClass, *volumesnapshotv1.VolumeSnapshotClass, *ctrl.Result, error) {
	names := cellcontract.ResourceNames(string(cell.UID))
	var data corev1.PersistentVolumeClaim
	if err := r.read(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.DataPVC}, &data); err != nil {
		result, statusErr := r.setPending(ctx, snapshot, reasonSnapshotSourceNotReady, "source data PVC is not ready")
		return nil, nil, nil, &result, errors.Join(client.IgnoreNotFound(err), statusErr)
	}
	controlled := cell.Spec.Storage.RetentionPolicy == dshv1alpha1.RetentionDelete
	if data.UID == "" || data.DeletionTimestamp != nil || validateManaged(cell, &data, controlled) != nil || data.Status.Phase != corev1.ClaimBound ||
		data.Spec.StorageClassName == nil || *data.Spec.StorageClassName == "" {
		result, err := r.setPending(ctx, snapshot, reasonSnapshotSourceNotReady, "source data PVC is not a bound resource owned by this Cell")
		return nil, nil, nil, &result, err
	}
	var storageClass storagev1.StorageClass
	if err := r.read(ctx, types.NamespacedName{Name: *data.Spec.StorageClassName}, &storageClass); err != nil {
		result, statusErr := r.setPending(ctx, snapshot, reasonSnapshotSourceNotReady, "source StorageClass is unavailable")
		return nil, nil, nil, &result, errors.Join(client.IgnoreNotFound(err), statusErr)
	}
	var snapshotClass volumesnapshotv1.VolumeSnapshotClass
	if err := r.read(ctx, types.NamespacedName{Name: snapshot.Spec.VolumeSnapshotClassName}, &snapshotClass); err != nil {
		if apierrors.IsNotFound(err) {
			result, statusErr := r.setPending(ctx, snapshot, reasonSnapshotClassMissing, "VolumeSnapshotClass is unavailable")
			return nil, nil, nil, &result, statusErr
		}
		return nil, nil, nil, nil, err
	}
	if snapshotClass.DeletionTimestamp != nil || snapshotClass.Driver != storageClass.Provisioner {
		result, err := r.setPending(ctx, snapshot, reasonSnapshotDriverMismatch, "snapshot and storage classes use different active CSI drivers")
		return nil, nil, nil, &result, err
	}
	return &data, &storageClass, &snapshotClass, nil, nil
}

func (r *CellSnapshotReconciler) acceptedSource(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (*dshv1alpha1.Cell, error) {
	cell, err := r.operationSource(ctx, snapshot)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errSnapshotSourceChanged
		}
		return nil, err
	}
	if cell.DeletionTimestamp != nil || cell.Generation != snapshot.Status.SourceGeneration || imageDigest(cell.Spec.Image) != snapshot.Status.ImageDigest {
		return nil, errSnapshotSourceChanged
	}
	if _, _, _, err := r.acceptedSnapshotInputs(ctx, snapshot, cell); err != nil {
		return nil, err
	}
	return cell, nil
}

func (r *CellSnapshotReconciler) acceptedSnapshotInputs(
	ctx context.Context,
	snapshot *dshv1alpha1.CellSnapshot,
	cell *dshv1alpha1.Cell,
) (*corev1.PersistentVolumeClaim, *storagev1.StorageClass, *volumesnapshotv1.VolumeSnapshotClass, error) {
	names := cellcontract.ResourceNames(string(cell.UID))
	var data corev1.PersistentVolumeClaim
	if err := r.read(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.DataPVC}, &data); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, nil, errSnapshotSourceChanged
		}
		return nil, nil, nil, err
	}
	controlled := cell.Spec.Storage.RetentionPolicy == dshv1alpha1.RetentionDelete
	if data.UID == "" || string(data.UID) != snapshot.Status.SourcePVCUID || data.DeletionTimestamp != nil ||
		validateManaged(cell, &data, controlled) != nil || data.Status.Phase != corev1.ClaimBound ||
		data.Spec.StorageClassName == nil || *data.Spec.StorageClassName != snapshot.Status.StorageClassName {
		return nil, nil, nil, errSnapshotSourceChanged
	}
	var storageClass storagev1.StorageClass
	if err := r.read(ctx, types.NamespacedName{Name: snapshot.Status.StorageClassName}, &storageClass); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, nil, errSnapshotSourceChanged
		}
		return nil, nil, nil, err
	}
	var snapshotClass volumesnapshotv1.VolumeSnapshotClass
	if err := r.read(ctx, types.NamespacedName{Name: snapshot.Spec.VolumeSnapshotClassName}, &snapshotClass); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, nil, errSnapshotSourceChanged
		}
		return nil, nil, nil, err
	}
	if snapshotClass.DeletionTimestamp != nil || snapshotClass.Driver != storageClass.Provisioner {
		return nil, nil, nil, errSnapshotSourceChanged
	}
	return &data, &storageClass, &snapshotClass, nil
}

func (r *CellSnapshotReconciler) operationSource(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (*dshv1alpha1.Cell, error) {
	var cell dshv1alpha1.Cell
	if err := r.read(ctx, types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.Spec.CellRef.Name}, &cell); err != nil {
		return nil, err
	}
	if snapshot.Status.SourceCellUID == "" || string(cell.UID) != snapshot.Status.SourceCellUID ||
		cell.Annotations[cellcontract.ActiveSnapshotAnnotation] != string(snapshot.UID) {
		return nil, errSnapshotSourceChanged
	}
	return &cell, nil
}

func (r *CellSnapshotReconciler) waitForWriterStop(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, cell *dshv1alpha1.Cell) (ctrl.Result, error) {
	if conditionAge(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) > r.Config.WriterStopTimeout {
		return r.fail(ctx, snapshot, reasonSnapshotWriterStopTimedOut, "managed writer did not stop before the deadline")
	}
	stopped, err := r.managedWriterStopped(ctx, cell)
	if err != nil {
		if errors.Is(err, errSnapshotSourceChanged) {
			return r.fail(ctx, snapshot, reasonSnapshotSourceChanged, "managed workload identity changed during writer stop")
		}
		return ctrl.Result{}, err
	}
	if !stopped {
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}
	err = r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotWriterStopped, metav1.ConditionTrue, reasonSnapshotWriterStopped, "StatefulSet is observed at zero and no managed Pod remains; no application flush is claimed")
	})
	return ctrl.Result{RequeueAfter: pendingRequeue}, err
}

func (r *CellSnapshotReconciler) managedWriterStopped(ctx context.Context, cell *dshv1alpha1.Cell) (bool, error) {
	names := cellcontract.ResourceNames(string(cell.UID))
	var workload appsv1.StatefulSet
	if err := r.read(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.Base}, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return false, errSnapshotSourceChanged
		}
		return false, err
	}
	if workload.DeletionTimestamp != nil || validateManaged(cell, &workload, true) != nil {
		return false, errSnapshotSourceChanged
	}
	if !statefulSetStopped(&workload) {
		return false, nil
	}
	var pods corev1.PodList
	if err := r.list(ctx, &pods, client.InNamespace(cell.Namespace)); err != nil {
		return false, err
	}
	for index := range pods.Items {
		pod := &pods.Items[index]
		owner := metav1.GetControllerOf(pod)
		ownedByCurrentWorkload := owner != nil && owner.APIVersion == appsv1.SchemeGroupVersion.String() &&
			owner.Kind == "StatefulSet" && owner.Name == workload.Name && owner.UID == workload.UID
		possibleCellWriter := pod.Annotations[cellcontract.CellUIDAnnotation] == string(cell.UID) &&
			pod.Annotations[cellcontract.CellNameAnnotation] == cell.Name
		if ownedByCurrentWorkload || possibleCellWriter {
			return false, nil
		}
	}
	return true, nil
}

func (r *CellSnapshotReconciler) snapshot(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, cell *dshv1alpha1.Cell) (ctrl.Result, error) {
	stopped, err := r.managedWriterStopped(ctx, cell)
	if err != nil {
		if errors.Is(err, errSnapshotSourceChanged) {
			return r.fail(ctx, snapshot, reasonSnapshotSourceChanged, "managed workload identity changed before CSI snapshot")
		}
		return ctrl.Result{}, err
	}
	if !stopped {
		err := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
			setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotWriterStopped, metav1.ConditionFalse, reasonSnapshotWriterStopPending, "writer-stop barrier was lost before CSI snapshot")
		})
		return ctrl.Result{RequeueAfter: pendingRequeue}, err
	}
	data, _, _, err := r.acceptedSnapshotInputs(ctx, snapshot, cell)
	if err != nil {
		if errors.Is(err, errSnapshotSourceChanged) {
			return r.fail(ctx, snapshot, reasonSnapshotSourceChanged, "source data PVC identity changed before CSI snapshot")
		}
		return ctrl.Result{}, err
	}

	volumeSnapshot, err := r.ensureVolumeSnapshot(ctx, snapshot, cell, data.Name)
	if err != nil {
		return r.fail(ctx, snapshot, reasonSnapshotFailed, "CSI snapshot reconciliation failed")
	}
	if volumeSnapshot.DeletionTimestamp != nil {
		return r.fail(ctx, snapshot, reasonSnapshotFailed, "CSI VolumeSnapshot is deleting")
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
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotReady, metav1.ConditionTrue, reasonSnapshotReady, "writer-stopped crash-consistent CSI snapshot is ready to restore")
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, r.releaseCell(ctx, snapshot)
}

func (r *CellSnapshotReconciler) fail(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, reason, message string) (ctrl.Result, error) {
	err := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotReady, metav1.ConditionFalse, reason, message)
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reasonSnapshotCleanupPending, "confirming owned Kubernetes snapshot object cleanup")
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	return r.completeFailure(ctx, snapshot)
}

func (r *CellSnapshotReconciler) completeFailure(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (ctrl.Result, error) {
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) &&
		!conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotWriterStopped) {
		cell, err := r.operationSource(ctx, snapshot)
		if err == nil {
			stopped, stopErr := r.managedWriterStopped(ctx, cell)
			if stopErr != nil || !stopped {
				statusErr := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
					setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reasonSnapshotCleanupBlocked, "source writer is not yet fenced; the operation lock remains held")
				})
				return ctrl.Result{RequeueAfter: pendingRequeue}, errors.Join(stopErr, statusErr)
			}
			if err := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
				setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotWriterStopped, metav1.ConditionTrue, reasonSnapshotWriterStopped, "writer-stop barrier completed while reconciling failure")
			}); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: pendingRequeue}, nil
		}
		if !apierrors.IsNotFound(err) && !errors.Is(err, errSnapshotSourceChanged) {
			return ctrl.Result{}, err
		}
	}
	gone, cleanupErr := r.deleteVolumeSnapshot(ctx, snapshot)
	if cleanupErr != nil {
		statusErr := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
			setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reasonSnapshotCleanupBlocked, "owned Kubernetes VolumeSnapshot deletion is blocked; backend state remains CSI-owned")
		})
		return ctrl.Result{RequeueAfter: pendingRequeue}, errors.Join(cleanupErr, statusErr)
	}
	if !gone {
		reason := reasonSnapshotCleanupPending
		message := "waiting for owned Kubernetes VolumeSnapshot deletion"
		if blocked, err := r.volumeSnapshotDeletionStarted(ctx, snapshot); err != nil {
			return ctrl.Result{RequeueAfter: pendingRequeue}, err
		} else if blocked {
			reason = reasonSnapshotCleanupBlocked
			message = "owned Kubernetes VolumeSnapshot remains after deletion was requested; backend state remains CSI-owned"
		}
		statusErr := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
			setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reason, message)
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
	return ctrl.Result{}, r.releaseCell(ctx, snapshot)
}

func (r *CellSnapshotReconciler) volumeSnapshotDeletionStarted(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (bool, error) {
	var object volumesnapshotv1.VolumeSnapshot
	err := r.read(ctx, types.NamespacedName{Namespace: snapshot.Namespace, Name: cellcontract.SnapshotName(string(snapshot.UID))}, &object)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return snapshotOwnsVolumeSnapshot(snapshot, &object) && object.DeletionTimestamp != nil, nil
}

func (r *CellSnapshotReconciler) reconcileDeleting(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(snapshot, cellcontract.SnapshotFinalizer) {
		return ctrl.Result{}, nil
	}
	active, changed, err := r.pruneRestoreProtections(ctx, snapshot)
	if err != nil {
		return ctrl.Result{RequeueAfter: pendingRequeue}, err
	}
	if active || changed {
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) {
		if cell, sourceErr := r.operationSource(ctx, snapshot); sourceErr == nil {
			stopped, stopErr := r.managedWriterStopped(ctx, cell)
			if stopErr != nil {
				return ctrl.Result{RequeueAfter: pendingRequeue}, stopErr
			}
			if !stopped {
				return ctrl.Result{RequeueAfter: pendingRequeue}, nil
			}
		} else if !apierrors.IsNotFound(sourceErr) && !errors.Is(sourceErr, errSnapshotSourceChanged) {
			return ctrl.Result{RequeueAfter: pendingRequeue}, sourceErr
		}
	}
	gone, err := r.deleteVolumeSnapshot(ctx, snapshot)
	if err != nil {
		return ctrl.Result{RequeueAfter: pendingRequeue}, err
	}
	if !gone {
		return ctrl.Result{RequeueAfter: pendingRequeue}, nil
	}
	if err := r.releaseCell(ctx, snapshot); err != nil {
		return ctrl.Result{}, err
	}
	copy := snapshot.DeepCopy()
	controllerutil.RemoveFinalizer(copy, cellcontract.SnapshotFinalizer)
	if err := r.Update(ctx, copy); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *CellSnapshotReconciler) pruneRestoreProtections(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (active bool, changed bool, err error) {
	var cells dshv1alpha1.CellList
	if err := r.list(ctx, &cells, client.InNamespace(snapshot.Namespace)); err != nil {
		return false, false, err
	}
	activeUIDs := make(map[string]struct{})
	for index := range cells.Items {
		cell := &cells.Items[index]
		if cell.Spec.Storage.RestoreFrom != nil && cell.Spec.Storage.RestoreFrom.Name == snapshot.Name &&
			controllerutil.ContainsFinalizer(cell, cellcontract.RestoreInitializationFinalizer) {
			activeUIDs[string(cell.UID)] = struct{}{}
		}
	}
	copy := snapshot.DeepCopy()
	for _, finalizer := range snapshot.Finalizers {
		if !strings.HasPrefix(finalizer, cellcontract.RestoreProtectionFinalizerPrefix) {
			continue
		}
		uid := strings.TrimPrefix(finalizer, cellcontract.RestoreProtectionFinalizerPrefix)
		if _, ok := activeUIDs[uid]; ok {
			active = true
			continue
		}
		controllerutil.RemoveFinalizer(copy, finalizer)
		changed = true
	}
	if !changed {
		return active, false, nil
	}
	patch := client.MergeFromWithOptions(snapshot.DeepCopy(), client.MergeFromWithOptimisticLock{})
	if err := r.Patch(ctx, copy, patch); err != nil {
		return active, false, err
	}
	return active, true, nil
}

func (r *CellSnapshotReconciler) setPending(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, reason, message string) (ctrl.Result, error) {
	err := r.patchStatus(ctx, snapshot, func(status *dshv1alpha1.CellSnapshotStatus) {
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotAccepted, metav1.ConditionFalse, reason, message)
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotWriterStopped, metav1.ConditionFalse, reasonSnapshotWriterStopPending, "source writer has not been stopped")
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotReady, metav1.ConditionFalse, reasonSnapshotPending, "snapshot is not ready")
		setSnapshotCondition(status, snapshot.Generation, dshv1alpha1.ConditionSnapshotFailed, metav1.ConditionFalse, reasonSnapshotPending, "snapshot has not failed")
	})
	return ctrl.Result{RequeueAfter: pendingRequeue}, err
}

func (r *CellSnapshotReconciler) ensureVolumeSnapshot(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, cell *dshv1alpha1.Cell, claimName string) (*volumesnapshotv1.VolumeSnapshot, error) {
	name := cellcontract.SnapshotName(string(snapshot.UID))
	key := types.NamespacedName{Namespace: snapshot.Namespace, Name: name}
	var object volumesnapshotv1.VolumeSnapshot
	err := r.read(ctx, key, &object)
	if apierrors.IsNotFound(err) {
		object = volumesnapshotv1.VolumeSnapshot{
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
		if err := controllerutil.SetControllerReference(snapshot, &object, r.Scheme); err != nil {
			return nil, err
		}
		if err := r.Create(ctx, &object); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
			if err := r.read(ctx, key, &object); err != nil {
				return nil, err
			}
		}
	} else if err != nil {
		return nil, err
	}
	if !snapshotOwnsVolumeSnapshot(snapshot, &object) || object.Annotations[cellcontract.CellUIDAnnotation] != string(cell.UID) {
		return nil, &ownershipConflictError{resource: "VolumeSnapshot"}
	}
	if object.Spec.Source.PersistentVolumeClaimName == nil || *object.Spec.Source.PersistentVolumeClaimName != claimName ||
		object.Spec.VolumeSnapshotClassName == nil || *object.Spec.VolumeSnapshotClassName != snapshot.Spec.VolumeSnapshotClassName {
		return nil, errors.New("managed VolumeSnapshot spec drifted")
	}
	return &object, nil
}

func (r *CellSnapshotReconciler) deleteVolumeSnapshot(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) (bool, error) {
	var object volumesnapshotv1.VolumeSnapshot
	err := r.read(ctx, types.NamespacedName{Namespace: snapshot.Namespace, Name: cellcontract.SnapshotName(string(snapshot.UID))}, &object)
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !snapshotOwnsVolumeSnapshot(snapshot, &object) {
		return true, nil
	}
	if object.DeletionTimestamp == nil {
		if err := r.Delete(ctx, &object); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}
	return false, nil
}

func snapshotOwnsVolumeSnapshot(snapshot *dshv1alpha1.CellSnapshot, object *volumesnapshotv1.VolumeSnapshot) bool {
	owner := metav1.GetControllerOf(object)
	return owner != nil && owner.UID == snapshot.UID && owner.Kind == "CellSnapshot" &&
		owner.APIVersion == dshv1alpha1.GroupVersion.String() &&
		object.Annotations[cellcontract.SnapshotUIDAnnotation] == string(snapshot.UID)
}

func (r *CellSnapshotReconciler) releaseCell(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot) error {
	if snapshot.Status.SourceCellUID == "" {
		return nil
	}
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

func (r *CellSnapshotReconciler) read(ctx context.Context, key client.ObjectKey, object client.Object) error {
	if r.APIReader != nil {
		return r.APIReader.Get(ctx, key, object)
	}
	return r.Get(ctx, key, object)
}

func (r *CellSnapshotReconciler) list(ctx context.Context, list client.ObjectList, options ...client.ListOption) error {
	if r.APIReader != nil {
		return r.APIReader.List(ctx, list, options...)
	}
	return r.List(ctx, list, options...)
}

func (r *CellSnapshotReconciler) patchStatus(ctx context.Context, snapshot *dshv1alpha1.CellSnapshot, mutate func(*dshv1alpha1.CellSnapshotStatus)) error {
	original := snapshot.DeepCopy()
	mutate(&snapshot.Status)
	snapshot.Status.ObservedGeneration = snapshot.Generation
	return r.Status().Patch(ctx, snapshot, client.MergeFrom(original))
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
