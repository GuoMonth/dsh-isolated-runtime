package controller

import (
	"context"
	"errors"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

const (
	reasonRestoreSourcePending = "RestoreSourcePending"
	reasonRestoreSourceInvalid = "RestoreSourceInvalid"
	reasonRestoreImageMismatch = "RestoreImageMismatch"
)

type restoreSource struct {
	VolumeSnapshotName string
	StorageClassName   string
	SnapshotUID        string
	ImageDigest        string
	DSHVersion         string
	Initialized        bool
}

func (r *CellReconciler) resolveRestore(ctx context.Context, cell *dshv1alpha1.Cell) (*restoreSource, *componentState, error) {
	if cell.Spec.Storage.RestoreFrom == nil {
		return nil, nil, nil
	}
	if existing, state, err := r.existingRestoreSource(ctx, cell); err != nil || state != nil {
		return existing, state, err
	} else if existing != nil {
		if existing.Initialized && (controllerutil.ContainsFinalizer(cell, cellcontract.RestoreInitializationFinalizer) || r.restoreProtectionExists(ctx, cell, existing.SnapshotUID)) {
			if err := r.releaseRestoreProtection(ctx, cell, existing.SnapshotUID); err != nil {
				return nil, nil, err
			}
			if err := r.removeCellRestoreFinalizer(ctx, cell); err != nil {
				return nil, nil, err
			}
			state := falseCondition(reasonRestoreSourcePending, "finishing restore protection cleanup")
			return nil, &state, nil
		}
		return existing, nil, nil
	}
	if !r.SnapshotEnabled {
		state := falseCondition(reasonSnapshotSupportDisabled, "snapshot support is disabled")
		return nil, &state, nil
	}

	snapshot, _, state, err := r.validRestoreObjects(ctx, cell, false)
	if err != nil || state != nil {
		if state != nil && snapshot != nil && controllerutil.ContainsFinalizer(cell, cellcontract.RestoreInitializationFinalizer) {
			if cleanupErr := r.releaseRestoreProtection(ctx, cell, string(snapshot.UID)); cleanupErr != nil {
				return nil, nil, cleanupErr
			}
			if cleanupErr := r.removeCellRestoreFinalizer(ctx, cell); cleanupErr != nil {
				return nil, nil, cleanupErr
			}
		}
		return nil, state, err
	}
	if !controllerutil.ContainsFinalizer(cell, cellcontract.RestoreInitializationFinalizer) {
		original := cell.DeepCopy()
		copy := cell.DeepCopy()
		controllerutil.AddFinalizer(copy, cellcontract.RestoreInitializationFinalizer)
		if err := r.Patch(ctx, copy, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})); err != nil {
			return nil, nil, err
		}
		state := falseCondition(reasonRestoreSourcePending, "protecting restore initialization")
		return nil, &state, nil
	}
	protection := cellcontract.RestoreProtectionFinalizer(string(cell.UID))
	if !controllerutil.ContainsFinalizer(snapshot, protection) {
		original := snapshot.DeepCopy()
		copy := snapshot.DeepCopy()
		controllerutil.AddFinalizer(copy, protection)
		if err := r.Patch(ctx, copy, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})); err != nil {
			if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
				state := falseCondition(reasonRestoreSourcePending, "restore CellSnapshot changed while protection was acquired")
				return nil, &state, nil
			}
			return nil, nil, err
		}
		state := falseCondition(reasonRestoreSourcePending, "protecting restore source until the first Ready reader")
		return nil, &state, nil
	}

	// Re-read both immutable inputs after protection is visible. This is the
	// final read-to-create barrier for the PVC's immutable dataSource.
	snapshot, volumeSnapshot, state, err := r.validRestoreObjects(ctx, cell, false)
	if err != nil || state != nil {
		if state != nil && snapshot != nil {
			if cleanupErr := r.releaseRestoreProtection(ctx, cell, string(snapshot.UID)); cleanupErr != nil {
				return nil, nil, cleanupErr
			}
			if cleanupErr := r.removeCellRestoreFinalizer(ctx, cell); cleanupErr != nil {
				return nil, nil, cleanupErr
			}
		}
		return nil, state, err
	}
	if !controllerutil.ContainsFinalizer(snapshot, protection) {
		state := falseCondition(reasonRestoreSourcePending, "restore protection is not yet observable")
		return nil, &state, nil
	}
	return &restoreSource{
		VolumeSnapshotName: volumeSnapshot.Name,
		StorageClassName:   snapshot.Status.StorageClassName,
		SnapshotUID:        string(snapshot.UID),
		ImageDigest:        snapshot.Status.ImageDigest,
		DSHVersion:         snapshot.Status.DSHVersion,
	}, nil, nil
}

func (r *CellReconciler) validRestoreObjects(
	ctx context.Context,
	cell *dshv1alpha1.Cell,
	allowProtectedDeletingSnapshot bool,
) (*dshv1alpha1.CellSnapshot, *volumesnapshotv1.VolumeSnapshot, *componentState, error) {
	var snapshot dshv1alpha1.CellSnapshot
	key := types.NamespacedName{Namespace: cell.Namespace, Name: cell.Spec.Storage.RestoreFrom.Name}
	if err := r.cellRead(ctx, key, &snapshot); err != nil {
		if apierrors.IsNotFound(err) {
			state := falseCondition(reasonRestoreSourcePending, "restore CellSnapshot is unavailable")
			return nil, nil, &state, nil
		}
		return nil, nil, nil, err
	}
	if snapshot.DeletionTimestamp != nil && (!allowProtectedDeletingSnapshot ||
		!controllerutil.ContainsFinalizer(&snapshot, cellcontract.RestoreProtectionFinalizer(string(cell.UID)))) {
		state := falseCondition(reasonRestoreSourceInvalid, "restore CellSnapshot is deleting")
		return &snapshot, nil, &state, nil
	}
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotFailed) {
		state := falseCondition(reasonRestoreSourceInvalid, "restore CellSnapshot failed")
		return &snapshot, nil, &state, nil
	}
	ready := meta.FindStatusCondition(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotReady)
	if ready == nil || ready.Status != metav1.ConditionTrue || snapshot.Status.ObservedGeneration != snapshot.Generation {
		state := falseCondition(reasonRestoreSourcePending, "restore CellSnapshot is not ready")
		return &snapshot, nil, &state, nil
	}
	if snapshot.Status.DSHVersion != cellcontract.DSHVersion || snapshot.Status.SourceCellUID == "" || snapshot.Status.SourcePVCUID == "" ||
		snapshot.Status.SourceGeneration <= 0 || snapshot.Status.StorageClassName == "" ||
		snapshot.Status.RestoreSize == nil || snapshot.Status.RestoreSize.Sign() <= 0 {
		state := falseCondition(reasonRestoreSourceInvalid, "restore CellSnapshot status is incomplete or incompatible")
		return &snapshot, nil, &state, nil
	}
	if imageDigest(cell.Spec.Image) != snapshot.Status.ImageDigest {
		state := falseCondition(reasonRestoreImageMismatch, "Cell image digest does not match the restore snapshot")
		return &snapshot, nil, &state, nil
	}
	if cell.Spec.Storage.Size.Cmp(*snapshot.Status.RestoreSize) < 0 {
		state := falseCondition(reasonRestoreSourceInvalid, "Cell storage is smaller than the restore snapshot")
		return &snapshot, nil, &state, nil
	}
	if cell.Spec.Storage.StorageClassName != nil && *cell.Spec.Storage.StorageClassName != snapshot.Status.StorageClassName {
		state := falseCondition(reasonRestoreSourceInvalid, "Cell and restore snapshot use different StorageClasses")
		return &snapshot, nil, &state, nil
	}

	var volumeSnapshot volumesnapshotv1.VolumeSnapshot
	volumeSnapshotName := cellcontract.SnapshotName(string(snapshot.UID))
	if err := r.cellRead(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: volumeSnapshotName}, &volumeSnapshot); err != nil {
		if apierrors.IsNotFound(err) {
			state := falseCondition(reasonRestoreSourcePending, "CSI VolumeSnapshot is unavailable")
			return &snapshot, nil, &state, nil
		}
		return nil, nil, nil, err
	}
	if volumeSnapshot.DeletionTimestamp != nil {
		state := falseCondition(reasonRestoreSourceInvalid, "CSI VolumeSnapshot is deleting")
		return &snapshot, &volumeSnapshot, &state, nil
	}
	if !snapshotOwnsVolumeSnapshot(&snapshot, &volumeSnapshot) {
		state := falseCondition(reasonRestoreSourceInvalid, "restore VolumeSnapshot ownership is invalid")
		return &snapshot, &volumeSnapshot, &state, nil
	}
	if volumeSnapshot.Status != nil && volumeSnapshot.Status.Error != nil {
		state := falseCondition(reasonRestoreSourceInvalid, "CSI VolumeSnapshot reports an error")
		return &snapshot, &volumeSnapshot, &state, nil
	}
	if volumeSnapshot.Status == nil || volumeSnapshot.Status.ReadyToUse == nil || !*volumeSnapshot.Status.ReadyToUse {
		state := falseCondition(reasonRestoreSourcePending, "CSI VolumeSnapshot is not ready")
		return &snapshot, &volumeSnapshot, &state, nil
	}
	return &snapshot, &volumeSnapshot, nil, nil
}

func (r *CellReconciler) existingRestoreSource(ctx context.Context, cell *dshv1alpha1.Cell) (*restoreSource, *componentState, error) {
	names := cellcontract.ResourceNames(string(cell.UID))
	var claim corev1.PersistentVolumeClaim
	if err := r.cellRead(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.DataPVC}, &claim); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	controlled := cell.Spec.Storage.RetentionPolicy == dshv1alpha1.RetentionDelete
	if err := validateManaged(cell, &claim, controlled); err != nil {
		return nil, nil, err
	}
	annotations := claim.Annotations
	source := &restoreSource{
		SnapshotUID: annotations[cellcontract.RestoreSnapshotUIDAnnotation],
		ImageDigest: annotations[cellcontract.RestoreImageDigestAnnotation],
		DSHVersion:  annotations[cellcontract.RestoreDSHVersionAnnotation],
		Initialized: annotations[cellcontract.RestoreInitializedAnnotation] == "true",
	}
	if claim.Spec.DataSource != nil && claim.Spec.DataSource.APIGroup != nil &&
		*claim.Spec.DataSource.APIGroup == volumesnapshotv1.GroupName && claim.Spec.DataSource.Kind == "VolumeSnapshot" {
		source.VolumeSnapshotName = claim.Spec.DataSource.Name
	}
	if claim.Spec.StorageClassName != nil {
		source.StorageClassName = *claim.Spec.StorageClassName
	}
	if source.VolumeSnapshotName == "" || source.StorageClassName == "" || source.SnapshotUID == "" ||
		source.ImageDigest == "" || source.DSHVersion != cellcontract.DSHVersion ||
		source.VolumeSnapshotName != cellcontract.SnapshotName(source.SnapshotUID) {
		state := falseCondition(reasonRestoreSourceInvalid, "managed restore PVC provenance is incomplete or invalid")
		return nil, &state, nil
	}
	if source.Initialized {
		return source, nil, nil
	}
	if imageDigest(cell.Spec.Image) != source.ImageDigest {
		state := falseCondition(reasonRestoreImageMismatch, "recorded restore image must become Ready before an image change")
		return nil, &state, nil
	}
	if !r.SnapshotEnabled {
		state := falseCondition(reasonSnapshotSupportDisabled, "snapshot support is disabled during restore initialization")
		return nil, &state, nil
	}
	var snapshot *dshv1alpha1.CellSnapshot
	var volumeSnapshot *volumesnapshotv1.VolumeSnapshot
	var state *componentState
	var err error
	if claim.Status.Phase == corev1.ClaimBound {
		snapshot, volumeSnapshot, state, err = r.validBoundRestoreProtection(ctx, cell, source)
	} else {
		snapshot, volumeSnapshot, state, err = r.validRestoreObjects(ctx, cell, true)
	}
	if err != nil || state != nil {
		return nil, state, err
	}
	if string(snapshot.UID) != source.SnapshotUID || volumeSnapshot.Name != source.VolumeSnapshotName ||
		snapshot.Status.ImageDigest != source.ImageDigest || snapshot.Status.DSHVersion != source.DSHVersion ||
		snapshot.Status.StorageClassName != source.StorageClassName ||
		!controllerutil.ContainsFinalizer(snapshot, cellcontract.RestoreProtectionFinalizer(string(cell.UID))) ||
		!controllerutil.ContainsFinalizer(cell, cellcontract.RestoreInitializationFinalizer) {
		state := falseCondition(reasonRestoreSourceInvalid, "restore initialization protection or provenance drifted")
		return nil, &state, nil
	}
	return source, nil, nil
}

// validBoundRestoreProtection verifies the durable transaction binding after
// CSI has materialized the immutable dataSource into a Bound PVC. At that
// point a delete request may put the protected CellSnapshot into termination,
// and its live Ready condition is no longer a prerequisite for starting the
// exact recorded image. The UID/provenance and both protection finalizers stay
// mandatory until that image becomes the first Ready reader.
func (r *CellReconciler) validBoundRestoreProtection(
	ctx context.Context,
	cell *dshv1alpha1.Cell,
	source *restoreSource,
) (*dshv1alpha1.CellSnapshot, *volumesnapshotv1.VolumeSnapshot, *componentState, error) {
	var snapshot dshv1alpha1.CellSnapshot
	key := types.NamespacedName{Namespace: cell.Namespace, Name: cell.Spec.Storage.RestoreFrom.Name}
	if err := r.cellRead(ctx, key, &snapshot); err != nil {
		if apierrors.IsNotFound(err) {
			state := falseCondition(reasonRestoreSourceInvalid, "protected restore CellSnapshot disappeared")
			return nil, nil, &state, nil
		}
		return nil, nil, nil, err
	}
	protection := cellcontract.RestoreProtectionFinalizer(string(cell.UID))
	if string(snapshot.UID) != source.SnapshotUID ||
		!controllerutil.ContainsFinalizer(&snapshot, protection) ||
		!controllerutil.ContainsFinalizer(cell, cellcontract.RestoreInitializationFinalizer) ||
		snapshot.Status.ImageDigest != source.ImageDigest ||
		snapshot.Status.DSHVersion != source.DSHVersion ||
		snapshot.Status.StorageClassName != source.StorageClassName {
		state := falseCondition(reasonRestoreSourceInvalid, "bound restore protection or provenance drifted")
		return &snapshot, nil, &state, nil
	}

	var volumeSnapshot volumesnapshotv1.VolumeSnapshot
	volumeKey := types.NamespacedName{Namespace: cell.Namespace, Name: source.VolumeSnapshotName}
	if err := r.cellRead(ctx, volumeKey, &volumeSnapshot); err != nil {
		if apierrors.IsNotFound(err) {
			state := falseCondition(reasonRestoreSourceInvalid, "protected CSI VolumeSnapshot disappeared")
			return &snapshot, nil, &state, nil
		}
		return nil, nil, nil, err
	}
	if volumeSnapshot.DeletionTimestamp != nil || !snapshotOwnsVolumeSnapshot(&snapshot, &volumeSnapshot) {
		state := falseCondition(reasonRestoreSourceInvalid, "protected restore VolumeSnapshot ownership is invalid")
		return &snapshot, &volumeSnapshot, &state, nil
	}
	return &snapshot, &volumeSnapshot, nil, nil
}

func (r *CellReconciler) completeRestoreInitialization(ctx context.Context, cell *dshv1alpha1.Cell, claim *corev1.PersistentVolumeClaim, restore *restoreSource) error {
	if restore == nil || restore.Initialized {
		return nil
	}
	if imageDigest(cell.Spec.Image) != restore.ImageDigest {
		return errors.New("restore image changed before the first Ready reader")
	}
	originalClaim := claim.DeepCopy()
	copyClaim := claim.DeepCopy()
	if copyClaim.Annotations == nil {
		copyClaim.Annotations = map[string]string{}
	}
	copyClaim.Annotations[cellcontract.RestoreInitializedAnnotation] = "true"
	if err := r.Patch(ctx, copyClaim, client.MergeFromWithOptions(originalClaim, client.MergeFromWithOptimisticLock{})); err != nil {
		return err
	}
	if err := r.releaseRestoreProtection(ctx, cell, restore.SnapshotUID); err != nil {
		return err
	}
	return r.removeCellRestoreFinalizer(ctx, cell)
}

func (r *CellReconciler) releaseRestoreProtection(ctx context.Context, cell *dshv1alpha1.Cell, snapshotUID string) error {
	if cell.Spec.Storage.RestoreFrom == nil || snapshotUID == "" {
		return nil
	}
	var snapshot dshv1alpha1.CellSnapshot
	key := types.NamespacedName{Namespace: cell.Namespace, Name: cell.Spec.Storage.RestoreFrom.Name}
	if err := r.cellRead(ctx, key, &snapshot); err != nil {
		return client.IgnoreNotFound(err)
	}
	if string(snapshot.UID) != snapshotUID {
		return nil
	}
	protection := cellcontract.RestoreProtectionFinalizer(string(cell.UID))
	if !controllerutil.ContainsFinalizer(&snapshot, protection) {
		return nil
	}
	original := snapshot.DeepCopy()
	copy := snapshot.DeepCopy()
	controllerutil.RemoveFinalizer(copy, protection)
	return r.Patch(ctx, copy, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{}))
}

func (r *CellReconciler) restoreProtectionExists(ctx context.Context, cell *dshv1alpha1.Cell, snapshotUID string) bool {
	if cell.Spec.Storage.RestoreFrom == nil || snapshotUID == "" {
		return false
	}
	var snapshot dshv1alpha1.CellSnapshot
	if err := r.cellRead(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: cell.Spec.Storage.RestoreFrom.Name}, &snapshot); err != nil || string(snapshot.UID) != snapshotUID {
		return false
	}
	return controllerutil.ContainsFinalizer(&snapshot, cellcontract.RestoreProtectionFinalizer(string(cell.UID)))
}

func (r *CellReconciler) removeCellRestoreFinalizer(ctx context.Context, cell *dshv1alpha1.Cell) error {
	if !controllerutil.ContainsFinalizer(cell, cellcontract.RestoreInitializationFinalizer) {
		return nil
	}
	var latest dshv1alpha1.Cell
	if err := r.cellRead(ctx, client.ObjectKeyFromObject(cell), &latest); err != nil {
		return client.IgnoreNotFound(err)
	}
	if latest.UID != cell.UID || !controllerutil.ContainsFinalizer(&latest, cellcontract.RestoreInitializationFinalizer) {
		return nil
	}
	original := latest.DeepCopy()
	copy := latest.DeepCopy()
	controllerutil.RemoveFinalizer(copy, cellcontract.RestoreInitializationFinalizer)
	return r.Patch(ctx, copy, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{}))
}

func (r *CellReconciler) cleanupDeletingRestore(ctx context.Context, cell *dshv1alpha1.Cell) (bool, error) {
	if !controllerutil.ContainsFinalizer(cell, cellcontract.RestoreInitializationFinalizer) {
		return false, nil
	}
	snapshotUID := ""
	names := cellcontract.ResourceNames(string(cell.UID))
	var claim corev1.PersistentVolumeClaim
	if err := r.cellRead(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.DataPVC}, &claim); err == nil {
		snapshotUID = claim.Annotations[cellcontract.RestoreSnapshotUIDAnnotation]
	} else if !apierrors.IsNotFound(err) {
		return true, err
	}
	if snapshotUID == "" && cell.Spec.Storage.RestoreFrom != nil {
		var snapshot dshv1alpha1.CellSnapshot
		if err := r.cellRead(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: cell.Spec.Storage.RestoreFrom.Name}, &snapshot); err == nil {
			snapshotUID = string(snapshot.UID)
		} else if !apierrors.IsNotFound(err) {
			return true, err
		}
	}
	if err := r.releaseRestoreProtection(ctx, cell, snapshotUID); err != nil {
		return true, err
	}
	return true, r.removeCellRestoreFinalizer(ctx, cell)
}
