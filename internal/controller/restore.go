package controller

import (
	"context"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

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
}

func (r *CellReconciler) resolveRestore(ctx context.Context, cell *dshv1alpha1.Cell) (*restoreSource, *componentState, error) {
	if cell.Spec.Storage.RestoreFrom == nil {
		return nil, nil, nil
	}
	// The exact image/version/class/size checks protect creation of the
	// immutable PVC dataSource. Once that managed claim exists, it is the
	// durable restore binding: later image rollouts must not be rejected by the
	// creation-time digest check, and deleting the source snapshot must not
	// break an already restored Cell.
	if existing, err := r.existingRestoreSource(ctx, cell); err != nil {
		return nil, nil, err
	} else if existing != nil {
		return existing, nil, nil
	}
	if !r.SnapshotEnabled {
		state := falseCondition(reasonSnapshotSupportDisabled, "snapshot support is disabled")
		return nil, &state, nil
	}
	var snapshot dshv1alpha1.CellSnapshot
	key := types.NamespacedName{Namespace: cell.Namespace, Name: cell.Spec.Storage.RestoreFrom.Name}
	if err := r.Get(ctx, key, &snapshot); err != nil {
		if apierrors.IsNotFound(err) {
			state := falseCondition(reasonRestoreSourcePending, "restore CellSnapshot is unavailable")
			return nil, &state, nil
		}
		return nil, nil, err
	}
	if conditionTrue(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotFailed) {
		state := falseCondition(reasonRestoreSourceInvalid, "restore CellSnapshot failed")
		return nil, &state, nil
	}
	ready := meta.FindStatusCondition(snapshot.Status.Conditions, dshv1alpha1.ConditionSnapshotReady)
	if ready == nil || ready.Status != metav1.ConditionTrue || snapshot.Status.ObservedGeneration != snapshot.Generation {
		state := falseCondition(reasonRestoreSourcePending, "restore CellSnapshot is not ready")
		return nil, &state, nil
	}
	if snapshot.Status.DSHVersion != cellcontract.DSHVersion || snapshot.Status.SourceCellUID == "" || snapshot.Status.SourceGeneration <= 0 || snapshot.Status.StorageClassName == "" || snapshot.Status.RestoreSize == nil || snapshot.Status.RestoreSize.Sign() <= 0 {
		state := falseCondition(reasonRestoreSourceInvalid, "restore CellSnapshot status is incomplete or incompatible")
		return nil, &state, nil
	}
	if imageDigest(cell.Spec.Image) != snapshot.Status.ImageDigest {
		state := falseCondition(reasonRestoreImageMismatch, "Cell image digest does not match the restore snapshot")
		return nil, &state, nil
	}
	if cell.Spec.Storage.Size.Cmp(*snapshot.Status.RestoreSize) < 0 {
		state := falseCondition(reasonRestoreSourceInvalid, "Cell storage is smaller than the restore snapshot")
		return nil, &state, nil
	}
	if cell.Spec.Storage.StorageClassName != nil && *cell.Spec.Storage.StorageClassName != snapshot.Status.StorageClassName {
		state := falseCondition(reasonRestoreSourceInvalid, "Cell and restore snapshot use different StorageClasses")
		return nil, &state, nil
	}

	var volumeSnapshot volumesnapshotv1.VolumeSnapshot
	volumeSnapshotName := cellcontract.SnapshotName(string(snapshot.UID))
	if err := r.Get(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: volumeSnapshotName}, &volumeSnapshot); err != nil {
		if apierrors.IsNotFound(err) {
			state := falseCondition(reasonRestoreSourcePending, "CSI VolumeSnapshot is unavailable")
			return nil, &state, nil
		}
		return nil, nil, err
	}
	if !snapshotOwnsVolumeSnapshot(&snapshot, &volumeSnapshot) {
		state := falseCondition(reasonRestoreSourceInvalid, "restore VolumeSnapshot ownership is invalid")
		return nil, &state, nil
	}
	if volumeSnapshot.Status != nil && volumeSnapshot.Status.Error != nil {
		state := falseCondition(reasonRestoreSourceInvalid, "CSI VolumeSnapshot reports an error")
		return nil, &state, nil
	}
	if volumeSnapshot.Status == nil || volumeSnapshot.Status.ReadyToUse == nil || !*volumeSnapshot.Status.ReadyToUse {
		state := falseCondition(reasonRestoreSourcePending, "CSI VolumeSnapshot is not ready")
		return nil, &state, nil
	}
	return &restoreSource{
		VolumeSnapshotName: volumeSnapshotName,
		StorageClassName:   snapshot.Status.StorageClassName,
	}, nil, nil
}

func (r *CellReconciler) existingRestoreSource(ctx context.Context, cell *dshv1alpha1.Cell) (*restoreSource, error) {
	names := cellcontract.ResourceNames(string(cell.UID))
	var claim corev1.PersistentVolumeClaim
	if err := r.Get(ctx, types.NamespacedName{Namespace: cell.Namespace, Name: names.DataPVC}, &claim); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	sourceName, storageClassName := "", ""
	if claim.Spec.DataSource != nil {
		sourceName = claim.Spec.DataSource.Name
	}
	if claim.Spec.StorageClassName != nil {
		storageClassName = *claim.Spec.StorageClassName
	}
	return &restoreSource{VolumeSnapshotName: sourceName, StorageClassName: storageClassName}, nil
}
