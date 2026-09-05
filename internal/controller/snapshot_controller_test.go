package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dshv1alpha1 "github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/internal/cellcontract"
)

const testSnapshotUID = types.UID("87654321-4321-4321-4321-cba987654321")

func TestSnapshotLifecycleAndFreshCellRestore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	storageClassName := "snapshot-storage"
	cell := testCell("snapshot-source", dshv1alpha1.RetentionRetain)
	cell.Spec.Storage.StorageClassName = &storageClassName
	cellReconciler, kube := testReconciler(t,
		cell,
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: storageClassName}, Provisioner: "snapshot.test/csi"},
		&volumesnapshotv1.VolumeSnapshotClass{ObjectMeta: metav1.ObjectMeta{Name: "snapshot-class"}, Driver: "snapshot.test/csi", DeletionPolicy: volumesnapshotv1.VolumeSnapshotContentDelete},
	)
	cellReconciler.SnapshotEnabled = true
	reconcileCell(t, cellReconciler, cell)
	names := cellcontract.ResourceNames(string(cell.UID))
	data := get[*corev1.PersistentVolumeClaim](t, kube, cell.Namespace, names.DataPVC)
	data.UID = types.UID("source-data-pvc-uid")
	if err := kube.Update(ctx, data); err != nil {
		t.Fatal(err)
	}
	private := get[*corev1.PersistentVolumeClaim](t, kube, cell.Namespace, names.PrivatePVC)
	workload := get[*appsv1.StatefulSet](t, kube, cell.Namespace, names.Base)
	workload.UID = types.UID("statefulset-snapshot-source")
	if err := kube.Update(ctx, workload); err != nil {
		t.Fatal(err)
	}
	access := get[*corev1.Service](t, kube, cell.Namespace, names.Base)
	markReady(t, kube, data, private, workload, access)
	reconcileCell(t, cellReconciler, cell)

	snapshot := &dshv1alpha1.CellSnapshot{
		TypeMeta:   metav1.TypeMeta{APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "CellSnapshot"},
		ObjectMeta: metav1.ObjectMeta{Name: "source-backup", Namespace: cell.Namespace, UID: testSnapshotUID, Generation: 1},
		Spec: dshv1alpha1.CellSnapshotSpec{
			CellRef: dshv1alpha1.LocalCellReference{Name: cell.Name}, VolumeSnapshotClassName: "snapshot-class",
		},
	}
	if err := kube.Create(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	snapshotReconciler := &CellSnapshotReconciler{
		Client: kube, Scheme: cellReconciler.Scheme,
		Config: SnapshotConfig{Enabled: true, WriterStopTimeout: 2 * time.Minute, SnapshotTimeout: 30 * time.Minute},
	}
	reconcileSnapshot(t, snapshotReconciler, snapshot, 3)
	locked := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	if locked.Annotations[cellcontract.ActiveSnapshotAnnotation] != string(snapshot.UID) {
		t.Fatal("snapshot did not acquire the operation lock")
	}
	// The Cell worker may observe the lock before the snapshot worker records
	// Accepted. Its writer fence makes Ready false; that must not deadlock the
	// very operation which owns the fence, including after a controller restart.
	reconcileCell(t, cellReconciler, cell)
	locked = get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	if conditionTrue(locked.Status.Conditions, dshv1alpha1.ConditionReady) {
		t.Fatal("writer fence did not withdraw Cell readiness")
	}
	reconcileSnapshot(t, snapshotReconciler, snapshot, 1)
	accepted := get[*dshv1alpha1.CellSnapshot](t, kube, snapshot.Namespace, snapshot.Name)
	if !conditionTrue(accepted.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) || conditionTrue(accepted.Status.Conditions, dshv1alpha1.ConditionSnapshotWriterStopped) {
		t.Fatalf("snapshot lock did not precede writer stop: status=%#v", accepted.Status)
	}

	reconcileCell(t, cellReconciler, cell)
	workload = get[*appsv1.StatefulSet](t, kube, cell.Namespace, names.Base)
	if workload.Spec.Replicas == nil || *workload.Spec.Replicas != 0 {
		t.Fatalf("writer-stop replicas = %#v", workload.Spec.Replicas)
	}
	if err := kube.Delete(ctx, get[*corev1.Pod](t, kube, cell.Namespace, workload.Name+"-0")); err != nil {
		t.Fatal(err)
	}
	workload.Status = appsv1.StatefulSetStatus{ObservedGeneration: workload.Generation}
	if err := kube.Status().Update(ctx, workload); err != nil {
		t.Fatal(err)
	}
	reconcileSnapshot(t, snapshotReconciler, snapshot, 2)
	volumeSnapshot := get[*volumesnapshotv1.VolumeSnapshot](t, kube, snapshot.Namespace, cellcontract.SnapshotName(string(snapshot.UID)))
	if volumeSnapshot.Spec.Source.PersistentVolumeClaimName == nil || *volumeSnapshot.Spec.Source.PersistentVolumeClaimName != names.DataPVC {
		t.Fatalf("snapshot source = %#v", volumeSnapshot.Spec.Source)
	}
	ready := true
	restoreSize := resource.MustParse("1Gi")
	volumeSnapshot.Status = &volumesnapshotv1.VolumeSnapshotStatus{ReadyToUse: &ready, RestoreSize: &restoreSize}
	if err := kube.Status().Update(ctx, volumeSnapshot); err != nil {
		t.Fatal(err)
	}
	reconcileSnapshot(t, snapshotReconciler, snapshot, 1)
	completed := get[*dshv1alpha1.CellSnapshot](t, kube, snapshot.Namespace, snapshot.Name)
	if !conditionTrue(completed.Status.Conditions, dshv1alpha1.ConditionSnapshotReady) || completed.Status.RestoreSize == nil {
		t.Fatalf("snapshot did not become Ready: %#v", completed.Status)
	}

	restored := testCell("restored", dshv1alpha1.RetentionRetain)
	restored.UID = types.UID("11111111-2222-3333-4444-555555555555")
	restored.Spec.Storage.StorageClassName = nil
	restored.Spec.Storage.RestoreFrom = &dshv1alpha1.LocalCellSnapshotReference{Name: snapshot.Name}
	if err := kube.Create(ctx, restored); err != nil {
		t.Fatal(err)
	}
	reconcileCell(t, cellReconciler, restored)
	reconcileCell(t, cellReconciler, restored)
	reconcileCell(t, cellReconciler, restored)
	restoredNames := cellcontract.ResourceNames(string(restored.UID))
	restoredData := get[*corev1.PersistentVolumeClaim](t, kube, restored.Namespace, restoredNames.DataPVC)
	if restoredData.Spec.DataSource == nil || restoredData.Spec.DataSource.Name != volumeSnapshot.Name || restoredData.Spec.StorageClassName == nil || *restoredData.Spec.StorageClassName != storageClassName {
		t.Fatalf("restored data PVC = %#v", restoredData.Spec)
	}
	restoredPrivate := get[*corev1.PersistentVolumeClaim](t, kube, restored.Namespace, restoredNames.PrivatePVC)
	if restoredPrivate.Spec.DataSource != nil {
		t.Fatalf("private PVC inherited snapshot source: %#v", restoredPrivate.Spec.DataSource)
	}
	prematureUpgrade := get[*dshv1alpha1.Cell](t, kube, restored.Namespace, restored.Name)
	prematureUpgrade.Spec.Image = "example.test/cell@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := kube.Update(ctx, prematureUpgrade); err != nil {
		t.Fatal(err)
	}
	reconcileCell(t, cellReconciler, prematureUpgrade)
	blocked := get[*dshv1alpha1.Cell](t, kube, restored.Namespace, restored.Name)
	storageCondition := meta.FindStatusCondition(blocked.Status.Conditions, dshv1alpha1.ConditionStorageReady)
	if storageCondition == nil || storageCondition.Reason != reasonRestoreImageMismatch {
		t.Fatalf("premature restore upgrade was not blocked: %#v", storageCondition)
	}
	if got := get[*appsv1.StatefulSet](t, kube, restored.Namespace, restoredNames.Base).Spec.Template.Spec.Containers[0].Image; got != restored.Spec.Image {
		t.Fatalf("alternate digest became the first restore reader: %q", got)
	}
	blocked.Spec.Image = restored.Spec.Image
	if err := kube.Update(ctx, blocked); err != nil {
		t.Fatal(err)
	}
	reconcileCell(t, cellReconciler, blocked)
	restoredData = get[*corev1.PersistentVolumeClaim](t, kube, restored.Namespace, restoredNames.DataPVC)
	restoredPrivate = get[*corev1.PersistentVolumeClaim](t, kube, restored.Namespace, restoredNames.PrivatePVC)
	restoredWorkload := get[*appsv1.StatefulSet](t, kube, restored.Namespace, restoredNames.Base)
	restoredAccess := get[*corev1.Service](t, kube, restored.Namespace, restoredNames.Base)
	markReady(t, kube, restoredData, restoredPrivate, restoredWorkload, restoredAccess)
	reconcileCell(t, cellReconciler, restored)
	reconcileCell(t, cellReconciler, restored)
	initialized := get[*corev1.PersistentVolumeClaim](t, kube, restored.Namespace, restoredNames.DataPVC)
	if initialized.Annotations[cellcontract.RestoreInitializedAnnotation] != "true" {
		t.Fatalf("first Ready reader did not seal restore provenance: %#v", initialized.Annotations)
	}
	protectedSnapshot := get[*dshv1alpha1.CellSnapshot](t, kube, snapshot.Namespace, snapshot.Name)
	if controllerutil.ContainsFinalizer(protectedSnapshot, cellcontract.RestoreProtectionFinalizer(string(restored.UID))) {
		t.Fatal("restore protection remained after the first Ready reader")
	}
	initializedCell := get[*dshv1alpha1.Cell](t, kube, restored.Namespace, restored.Name)
	if controllerutil.ContainsFinalizer(initializedCell, cellcontract.RestoreInitializationFinalizer) {
		t.Fatal("target Cell restore finalizer remained after initialization")
	}

	upgraded := get[*dshv1alpha1.Cell](t, kube, restored.Namespace, restored.Name)
	upgraded.Spec.Image = "example.test/cell@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := kube.Update(ctx, upgraded); err != nil {
		t.Fatal(err)
	}
	reconcileCell(t, cellReconciler, upgraded)
	upgradedWorkload := get[*appsv1.StatefulSet](t, kube, restored.Namespace, restoredNames.Base)
	if got := upgradedWorkload.Spec.Template.Spec.Containers[0].Image; got != upgraded.Spec.Image {
		t.Fatalf("restored Cell upgrade image = %q, want %q", got, upgraded.Spec.Image)
	}

	if err := kube.Delete(ctx, volumeSnapshot); err != nil {
		t.Fatal(err)
	}
	withoutSnapshot := get[*dshv1alpha1.Cell](t, kube, restored.Namespace, restored.Name)
	restore, state, err := cellReconciler.resolveRestore(ctx, withoutSnapshot)
	if err != nil || state != nil || restore == nil || restore.VolumeSnapshotName != volumeSnapshot.Name {
		t.Fatalf("existing restore binding depended on source snapshot: restore=%#v state=%#v err=%v", restore, state, err)
	}
}

func TestRestoreRejectsWrongImageBeforePVC(t *testing.T) {
	t.Parallel()
	size := resource.MustParse("1Gi")
	snapshot := &dshv1alpha1.CellSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "tenant-a", UID: testSnapshotUID, Generation: 1},
		Status: dshv1alpha1.CellSnapshotStatus{
			ObservedGeneration: 1, SourceCellUID: string(testUID), SourcePVCUID: "source-data-pvc-uid", SourceGeneration: 1,
			DSHVersion: cellcontract.DSHVersion, ImageDigest: testDigest,
			StorageClassName: "snapshot-storage", RestoreSize: &size,
			Conditions: []metav1.Condition{{Type: dshv1alpha1.ConditionSnapshotReady, Status: metav1.ConditionTrue, Reason: reasonSnapshotReady, ObservedGeneration: 1}},
		},
	}
	volumeSnapshot := readyVolumeSnapshot(snapshot, "source-data", size)
	cell := testCell("wrong-image", dshv1alpha1.RetentionRetain)
	cell.UID = types.UID("99999999-2222-3333-4444-555555555555")
	cell.Spec.Image = "example.test/cell@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	cell.Spec.Storage.RestoreFrom = &dshv1alpha1.LocalCellSnapshotReference{Name: snapshot.Name}
	reconciler, kube := testReconciler(t, cell, snapshot, volumeSnapshot)
	reconciler.SnapshotEnabled = true
	reconcileCell(t, reconciler, cell)
	observed := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	storage := meta.FindStatusCondition(observed.Status.Conditions, dshv1alpha1.ConditionStorageReady)
	if storage == nil || storage.Reason != reasonRestoreImageMismatch {
		t.Fatalf("wrong image restore status = %#v", storage)
	}
	var claim corev1.PersistentVolumeClaim
	err := kube.Get(context.Background(), types.NamespacedName{Namespace: cell.Namespace, Name: cellcontract.ResourceNames(string(cell.UID)).DataPVC}, &claim)
	if err == nil {
		t.Fatal("wrong-image restore created a data PVC")
	}
}

func TestRestoreRejectsDeletingInputsBeforePVC(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"CellSnapshot", "VolumeSnapshot"} {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			now := metav1.Now()
			size := resource.MustParse("1Gi")
			snapshot := &dshv1alpha1.CellSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "deleting", Namespace: "tenant-a", UID: testSnapshotUID, Generation: 1,
					Finalizers: []string{cellcontract.SnapshotFinalizer},
				},
				Status: dshv1alpha1.CellSnapshotStatus{
					ObservedGeneration: 1, SourceCellUID: string(testUID), SourcePVCUID: "source-data-pvc-uid", SourceGeneration: 1,
					DSHVersion: cellcontract.DSHVersion, ImageDigest: testDigest,
					StorageClassName: "snapshot-storage", RestoreSize: &size,
					Conditions: []metav1.Condition{{
						Type: dshv1alpha1.ConditionSnapshotReady, Status: metav1.ConditionTrue,
						Reason: reasonSnapshotReady, ObservedGeneration: 1,
					}},
				},
			}
			volumeSnapshot := readyVolumeSnapshot(snapshot, "source-data", size)
			if target == "CellSnapshot" {
				snapshot.DeletionTimestamp = &now
			} else {
				volumeSnapshot.Finalizers = append(volumeSnapshot.Finalizers, "snapshot.storage.kubernetes.io/volumesnapshot-bound-protection")
				volumeSnapshot.DeletionTimestamp = &now
			}
			cell := testCell("restore-deleting", dshv1alpha1.RetentionDelete)
			cell.UID = types.UID("restore-deleting-uid")
			cell.Spec.Storage.RestoreFrom = &dshv1alpha1.LocalCellSnapshotReference{Name: snapshot.Name}
			reconciler, kube := testReconciler(t, cell, snapshot, volumeSnapshot)
			reconciler.SnapshotEnabled = true
			reconcileCell(t, reconciler, cell)
			observed := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
			condition := meta.FindStatusCondition(observed.Status.Conditions, dshv1alpha1.ConditionStorageReady)
			if condition == nil || condition.Reason != reasonRestoreSourceInvalid {
				t.Fatalf("deleting %s restore status = %#v", target, condition)
			}
			var claim corev1.PersistentVolumeClaim
			err := kube.Get(context.Background(), types.NamespacedName{
				Namespace: cell.Namespace, Name: cellcontract.ResourceNames(string(cell.UID)).DataPVC,
			}, &claim)
			if !apierrors.IsNotFound(err) {
				t.Fatalf("deleting %s created immutable restore PVC: %v", target, err)
			}
		})
	}
}

func TestBoundRestoreContinuesWhileProtectedSnapshotIsDeleting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := metav1.Now()
	size := resource.MustParse("1Gi")
	cell := testCell("deletion-race", dshv1alpha1.RetentionRetain)
	cell.UID = types.UID("restore-deletion-race-uid")
	cell.Spec.Storage.RestoreFrom = &dshv1alpha1.LocalCellSnapshotReference{Name: "source-backup"}
	cell.Finalizers = []string{cellcontract.RestoreInitializationFinalizer}
	protection := cellcontract.RestoreProtectionFinalizer(string(cell.UID))
	snapshot := &dshv1alpha1.CellSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name: "source-backup", Namespace: cell.Namespace, UID: testSnapshotUID, Generation: 1,
			DeletionTimestamp: &now, Finalizers: []string{cellcontract.SnapshotFinalizer, protection},
		},
		Status: dshv1alpha1.CellSnapshotStatus{
			ObservedGeneration: 1, SourceCellUID: string(testUID), SourcePVCUID: "source-pvc", SourceGeneration: 1,
			DSHVersion: cellcontract.DSHVersion, ImageDigest: testDigest,
			StorageClassName: "snapshot-storage", RestoreSize: &size,
			Conditions: []metav1.Condition{{
				Type: dshv1alpha1.ConditionSnapshotReady, Status: metav1.ConditionFalse,
				Reason: reasonSnapshotPending, ObservedGeneration: 1,
			}},
		},
	}
	volumeSnapshot := readyVolumeSnapshot(snapshot, "source-data", size)
	names := cellcontract.ResourceNames(string(cell.UID))
	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.DataPVC, Namespace: cell.Namespace,
			Annotations: map[string]string{
				cellcontract.RestoreSnapshotUIDAnnotation: string(snapshot.UID),
				cellcontract.RestoreImageDigestAnnotation: testDigest,
				cellcontract.RestoreDSHVersionAnnotation:  cellcontract.DSHVersion,
				cellcontract.RestoreInitializedAnnotation: "false",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: ptr.To("snapshot-storage"),
			DataSource: &corev1.TypedLocalObjectReference{
				APIGroup: ptr.To(volumesnapshotv1.GroupName), Kind: "VolumeSnapshot", Name: volumeSnapshot.Name,
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
	setCellMetadata(claim, cell)
	reconciler, _ := testReconciler(t, cell, snapshot, volumeSnapshot, claim)
	reconciler.SnapshotEnabled = true

	source, state, err := reconciler.existingRestoreSource(ctx, cell)
	if err != nil || state != nil || source == nil {
		t.Fatalf("bound protected restore did not survive snapshot deletion: source=%#v state=%#v err=%v", source, state, err)
	}
	if source.SnapshotUID != string(snapshot.UID) || source.ImageDigest != testDigest {
		t.Fatalf("bound restore provenance changed: %#v", source)
	}
}

func TestSnapshotDisabledIsObservableWithoutQuiesce(t *testing.T) {
	t.Parallel()
	snapshot := &dshv1alpha1.CellSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "disabled", Namespace: "tenant-a", UID: testSnapshotUID, Generation: 1},
		Spec:       dshv1alpha1.CellSnapshotSpec{CellRef: dshv1alpha1.LocalCellReference{Name: "cell"}, VolumeSnapshotClassName: "class"},
	}
	reconciler, kube := testReconciler(t, snapshot)
	snapshotReconciler := &CellSnapshotReconciler{Client: kube, Scheme: reconciler.Scheme, Config: SnapshotConfig{Enabled: false}}
	reconcileSnapshot(t, snapshotReconciler, snapshot, 1)
	observed := get[*dshv1alpha1.CellSnapshot](t, kube, snapshot.Namespace, snapshot.Name)
	accepted := meta.FindStatusCondition(observed.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted)
	if accepted == nil || accepted.Status != metav1.ConditionFalse || accepted.Reason != reasonSnapshotSupportDisabled || len(observed.Finalizers) != 0 {
		t.Fatalf("disabled snapshot status = %#v finalizers=%v", observed.Status, observed.Finalizers)
	}
}

func TestSnapshotOperationsQueueAndRecoverAfterLockRelease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	storageClassName := "snapshot-storage"
	cell := testCell("source", dshv1alpha1.RetentionRetain)
	cell.Spec.Storage.StorageClassName = &storageClassName
	cell.Status = dshv1alpha1.CellStatus{
		ObservedGeneration: cell.Generation,
		DSHVersion:         cellcontract.DSHVersion,
		ImageDigest:        testDigest,
		Conditions: []metav1.Condition{{
			Type: dshv1alpha1.ConditionReady, Status: metav1.ConditionTrue,
			Reason: reasonComponentsReady, ObservedGeneration: cell.Generation,
		}},
	}
	names := cellcontract.ResourceNames(string(cell.UID))
	data := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: names.DataPVC, Namespace: cell.Namespace, UID: types.UID("source-data-pvc-uid")},
		Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClassName},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
	setCellMetadata(data, cell)
	first := &dshv1alpha1.CellSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "first", Namespace: cell.Namespace, UID: testSnapshotUID, Generation: 1},
		Spec:       dshv1alpha1.CellSnapshotSpec{CellRef: dshv1alpha1.LocalCellReference{Name: cell.Name}, VolumeSnapshotClassName: "snapshot-class"},
	}
	second := first.DeepCopy()
	second.Name = "second"
	second.UID = types.UID("99999999-4321-4321-4321-cba987654321")
	cellReconciler, kube := testReconciler(t,
		cell, data, first, second,
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: storageClassName}, Provisioner: "snapshot.test/csi"},
		&volumesnapshotv1.VolumeSnapshotClass{ObjectMeta: metav1.ObjectMeta{Name: "snapshot-class"}, Driver: "snapshot.test/csi"},
	)
	reconciler := &CellSnapshotReconciler{
		Client: kube, Scheme: cellReconciler.Scheme,
		Config: SnapshotConfig{Enabled: true, WriterStopTimeout: 2 * time.Minute, SnapshotTimeout: 30 * time.Minute},
	}

	reconcileSnapshot(t, reconciler, first, 3)
	reconcileSnapshot(t, reconciler, second, 1)
	queued := get[*dshv1alpha1.CellSnapshot](t, kube, second.Namespace, second.Name)
	accepted := meta.FindStatusCondition(queued.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted)
	if accepted == nil || accepted.Status != metav1.ConditionFalse || accepted.Reason != reasonSnapshotOperationQueued {
		t.Fatalf("second operation was not queued: %#v", accepted)
	}
	locked := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	if locked.Annotations[cellcontract.ActiveSnapshotAnnotation] != string(first.UID) {
		t.Fatalf("active operation = %q", locked.Annotations[cellcontract.ActiveSnapshotAnnotation])
	}

	active := get[*dshv1alpha1.CellSnapshot](t, kube, first.Namespace, first.Name)
	meta.SetStatusCondition(&active.Status.Conditions, metav1.Condition{
		Type: dshv1alpha1.ConditionSnapshotReady, Status: metav1.ConditionTrue,
		Reason: reasonSnapshotReady, ObservedGeneration: active.Generation,
	})
	if err := kube.Status().Update(ctx, active); err != nil {
		t.Fatal(err)
	}
	reconcileSnapshot(t, reconciler, first, 1)
	reconcileSnapshot(t, reconciler, second, 4)
	recovered := get[*dshv1alpha1.CellSnapshot](t, kube, second.Namespace, second.Name)
	if !conditionTrue(recovered.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted) {
		t.Fatalf("queued operation did not acquire the released lock: %#v", recovered.Status)
	}
}

func TestSnapshotLockUsesUncachedReader(t *testing.T) {
	t.Parallel()
	storageClassName := "snapshot-storage"
	cell := testCell("source", dshv1alpha1.RetentionRetain)
	cell.Spec.Storage.StorageClassName = &storageClassName
	cell.Status = dshv1alpha1.CellStatus{
		ObservedGeneration: cell.Generation,
		DSHVersion:         cellcontract.DSHVersion,
		ImageDigest:        testDigest,
		Conditions: []metav1.Condition{{
			Type: dshv1alpha1.ConditionReady, Status: metav1.ConditionTrue,
			Reason: reasonComponentsReady, ObservedGeneration: cell.Generation,
		}},
	}
	names := cellcontract.ResourceNames(string(cell.UID))
	data := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: names.DataPVC, Namespace: cell.Namespace, UID: types.UID("source-data-pvc-uid")},
		Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClassName},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
	setCellMetadata(data, cell)
	second := &dshv1alpha1.CellSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "second", Namespace: cell.Namespace, UID: testSnapshotUID, Generation: 1},
		Spec:       dshv1alpha1.CellSnapshotSpec{CellRef: dshv1alpha1.LocalCellReference{Name: cell.Name}, VolumeSnapshotClassName: "snapshot-class"},
	}
	cellReconciler, cached := testReconciler(t,
		cell, data, second,
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: storageClassName}, Provisioner: "snapshot.test/csi"},
		&volumesnapshotv1.VolumeSnapshotClass{ObjectMeta: metav1.ObjectMeta{Name: "snapshot-class"}, Driver: "snapshot.test/csi"},
	)
	directCell := cell.DeepCopy()
	directCell.Annotations = map[string]string{cellcontract.ActiveSnapshotAnnotation: "first-operation"}
	directCell.Status = dshv1alpha1.CellStatus{}
	direct := fake.NewClientBuilder().WithScheme(cellReconciler.Scheme).WithObjects(directCell).Build()
	reconciler := &CellSnapshotReconciler{
		Client: cached, APIReader: direct, Scheme: cellReconciler.Scheme,
		Config: SnapshotConfig{Enabled: true, WriterStopTimeout: 2 * time.Minute, SnapshotTimeout: 30 * time.Minute},
	}

	reconcileSnapshot(t, reconciler, second, 1)
	queued := get[*dshv1alpha1.CellSnapshot](t, cached, second.Namespace, second.Name)
	accepted := meta.FindStatusCondition(queued.Status.Conditions, dshv1alpha1.ConditionSnapshotAccepted)
	if accepted == nil || accepted.Status != metav1.ConditionFalse || accepted.Reason != reasonSnapshotOperationQueued {
		t.Fatalf("stale cache bypass did not preserve active operation: %#v", accepted)
	}
}

func TestWriterStopBarrierUsesUncachedUnfilteredPodList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cell := testCell("source", dshv1alpha1.RetentionRetain)
	names := cellcontract.ResourceNames(string(cell.UID))
	zero := int32(0)
	workload := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace, UID: types.UID("current-workload"), Generation: 1},
		Spec:       appsv1.StatefulSetSpec{Replicas: &zero},
		Status:     appsv1.StatefulSetStatus{ObservedGeneration: 1},
	}
	setCellMetadata(workload, cell)
	workload.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "Cell", Name: cell.Name, UID: cell.UID, Controller: ptr.To(true),
	}}
	possibleWriter := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.Base + "-0", Namespace: cell.Namespace,
			// Labels and owner are deliberately absent: a mutable selector or a
			// replaced controller must never hide a Cell-identified writer.
			Annotations: cellAnnotations(cell),
		},
	}
	base, cached := testReconciler(t)
	direct := fake.NewClientBuilder().WithScheme(base.Scheme).WithObjects(workload, possibleWriter).Build()
	reconciler := &CellSnapshotReconciler{Client: cached, APIReader: direct, Scheme: base.Scheme}

	stopped, err := reconciler.managedWriterStopped(ctx, cell)
	if err != nil {
		t.Fatal(err)
	}
	if stopped {
		t.Fatal("unowned, unlabeled Pod carrying exact Cell identity was missed")
	}
	if err := direct.Delete(ctx, possibleWriter); err != nil {
		t.Fatal(err)
	}
	stopped, err = reconciler.managedWriterStopped(ctx, cell)
	if err != nil {
		t.Fatal(err)
	}
	if !stopped {
		t.Fatal("writer barrier did not complete after the directly observed Pod disappeared")
	}
}

func TestForeignVolumeSnapshotIsNeverAdoptedOrDeleted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	snapshot := &dshv1alpha1.CellSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foreign", Namespace: "tenant-a", UID: testSnapshotUID, Generation: 1,
			Finalizers: []string{cellcontract.SnapshotFinalizer},
		},
		Spec: dshv1alpha1.CellSnapshotSpec{CellRef: dshv1alpha1.LocalCellReference{Name: "source"}, VolumeSnapshotClassName: "snapshot-class"},
		Status: dshv1alpha1.CellSnapshotStatus{
			ObservedGeneration: 1, SourceCellUID: string(testUID), SourceGeneration: 1,
			DSHVersion: cellcontract.DSHVersion, ImageDigest: testDigest, StorageClassName: "snapshot-storage",
			Conditions: []metav1.Condition{
				{Type: dshv1alpha1.ConditionSnapshotAccepted, Status: metav1.ConditionTrue, Reason: reasonSnapshotPrerequisites},
				{Type: dshv1alpha1.ConditionSnapshotWriterStopped, Status: metav1.ConditionTrue, Reason: reasonSnapshotWriterStopped},
			},
		},
	}
	cell := testCell("source", dshv1alpha1.RetentionRetain)
	cell.Annotations = map[string]string{cellcontract.ActiveSnapshotAnnotation: string(snapshot.UID)}
	names := cellcontract.ResourceNames(string(cell.UID))
	zero := int32(0)
	workload := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace, Generation: 1},
		Spec:       appsv1.StatefulSetSpec{Replicas: &zero},
		Status:     appsv1.StatefulSetStatus{ObservedGeneration: 1},
	}
	setCellMetadata(workload, cell)
	workload.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "Cell", Name: cell.Name, UID: cell.UID, Controller: ptr.To(true),
	}}
	claimName := names.DataPVC
	className := snapshot.Spec.VolumeSnapshotClassName
	foreign := &volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: cellcontract.SnapshotName(string(snapshot.UID)), Namespace: snapshot.Namespace},
		Spec: volumesnapshotv1.VolumeSnapshotSpec{
			VolumeSnapshotClassName: &className,
			Source:                  volumesnapshotv1.VolumeSnapshotSource{PersistentVolumeClaimName: &claimName},
		},
	}
	cellReconciler, kube := testReconciler(t, cell, snapshot, workload, foreign)
	reconciler := &CellSnapshotReconciler{
		Client: kube, Scheme: cellReconciler.Scheme,
		Config: SnapshotConfig{Enabled: true, WriterStopTimeout: 2 * time.Minute, SnapshotTimeout: 30 * time.Minute},
	}

	reconcileSnapshot(t, reconciler, snapshot, 1)
	observed := get[*dshv1alpha1.CellSnapshot](t, kube, snapshot.Namespace, snapshot.Name)
	if !conditionTrue(observed.Status.Conditions, dshv1alpha1.ConditionSnapshotFailed) {
		t.Fatalf("foreign collision did not fail the operation: %#v", observed.Status)
	}
	if err := kube.Get(ctx, client.ObjectKeyFromObject(foreign), &volumesnapshotv1.VolumeSnapshot{}); err != nil {
		t.Fatalf("foreign VolumeSnapshot was removed: %v", err)
	}
}

func TestSnapshotDeletionWaitsForFirstReadyRestoreReader(t *testing.T) {
	t.Parallel()
	now := metav1.Now()
	restoreUID := types.UID("restore-reader-uid")
	snapshot := &dshv1alpha1.CellSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name: "protected", Namespace: "tenant-a", UID: testSnapshotUID, Generation: 1,
			DeletionTimestamp: &now,
			Finalizers: []string{
				cellcontract.SnapshotFinalizer,
				cellcontract.RestoreProtectionFinalizer(string(restoreUID)),
			},
		},
		Spec: dshv1alpha1.CellSnapshotSpec{
			CellRef: dshv1alpha1.LocalCellReference{Name: "source"}, VolumeSnapshotClassName: "snapshot-class",
		},
	}
	restoreCell := testCell("restore-reader", dshv1alpha1.RetentionDelete)
	restoreCell.UID = restoreUID
	restoreCell.Spec.Storage.RestoreFrom = &dshv1alpha1.LocalCellSnapshotReference{Name: snapshot.Name}
	restoreCell.Finalizers = []string{cellcontract.RestoreInitializationFinalizer}
	volumeSnapshot := readyVolumeSnapshot(snapshot, "source-data", resource.MustParse("1Gi"))
	cellReconciler, kube := testReconciler(t, snapshot, restoreCell, volumeSnapshot)
	reconciler := &CellSnapshotReconciler{
		Client: kube, Scheme: cellReconciler.Scheme,
		Config: SnapshotConfig{Enabled: true, WriterStopTimeout: 2 * time.Minute, SnapshotTimeout: 30 * time.Minute},
	}

	reconcileSnapshot(t, reconciler, snapshot, 1)
	if err := kube.Get(context.Background(), client.ObjectKeyFromObject(volumeSnapshot), &volumesnapshotv1.VolumeSnapshot{}); err != nil {
		t.Fatalf("protected CSI snapshot was deleted before first Ready reader: %v", err)
	}
	observed := get[*dshv1alpha1.CellSnapshot](t, kube, snapshot.Namespace, snapshot.Name)
	if !controllerutil.ContainsFinalizer(observed, cellcontract.RestoreProtectionFinalizer(string(restoreUID))) {
		t.Fatal("active restore protection was pruned")
	}
}

func TestSnapshotFailureDeletesCSIObjectBeforeResuming(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	snapshot := &dshv1alpha1.CellSnapshot{
		TypeMeta: metav1.TypeMeta{APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "CellSnapshot"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "failed", Namespace: "tenant-a", UID: testSnapshotUID, Generation: 1,
			Finalizers: []string{cellcontract.SnapshotFinalizer},
		},
		Spec: dshv1alpha1.CellSnapshotSpec{CellRef: dshv1alpha1.LocalCellReference{Name: "source"}, VolumeSnapshotClassName: "snapshot-class"},
		Status: dshv1alpha1.CellSnapshotStatus{
			ObservedGeneration: 1, SourceCellUID: string(testUID), SourceGeneration: 1,
			DSHVersion: cellcontract.DSHVersion, ImageDigest: testDigest, StorageClassName: "snapshot-storage",
			Conditions: []metav1.Condition{
				{Type: dshv1alpha1.ConditionSnapshotAccepted, Status: metav1.ConditionTrue, Reason: reasonSnapshotPrerequisites},
				{Type: dshv1alpha1.ConditionSnapshotWriterStopped, Status: metav1.ConditionTrue, Reason: reasonSnapshotWriterStopped},
			},
		},
	}
	cell := testCell("source", dshv1alpha1.RetentionRetain)
	cell.Annotations = map[string]string{cellcontract.ActiveSnapshotAnnotation: string(snapshot.UID)}
	names := cellcontract.ResourceNames(string(cell.UID))
	zero := int32(0)
	workload := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace, Generation: 1},
		Spec:       appsv1.StatefulSetSpec{Replicas: &zero},
		Status:     appsv1.StatefulSetStatus{ObservedGeneration: 1},
	}
	setCellMetadata(workload, cell)
	workload.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "Cell", Name: cell.Name, UID: cell.UID, Controller: ptr.To(true),
	}}
	message := "fixture CSI failure"
	volumeSnapshot := readyVolumeSnapshot(snapshot, names.DataPVC, resource.MustParse("1Gi"))
	volumeSnapshot.Status.Error = &volumesnapshotv1.VolumeSnapshotError{Message: &message}
	cellReconciler, kube := testReconciler(t, cell, snapshot, workload, volumeSnapshot)
	reconciler := &CellSnapshotReconciler{
		Client: kube, Scheme: cellReconciler.Scheme,
		Config: SnapshotConfig{Enabled: true, WriterStopTimeout: 2 * time.Minute, SnapshotTimeout: 30 * time.Minute},
	}

	reconcileSnapshot(t, reconciler, snapshot, 2)
	failed := get[*dshv1alpha1.CellSnapshot](t, kube, snapshot.Namespace, snapshot.Name)
	if !conditionTrue(failed.Status.Conditions, dshv1alpha1.ConditionSnapshotFailed) || !controllerutil.ContainsFinalizer(failed, cellcontract.SnapshotFinalizer) {
		t.Fatalf("failed snapshot did not retain deletion protection after cleanup: %#v", failed)
	}
	var deleted volumesnapshotv1.VolumeSnapshot
	if err := kube.Get(ctx, client.ObjectKeyFromObject(volumeSnapshot), &deleted); err == nil {
		t.Fatal("failed CSI VolumeSnapshot still exists")
	}
	resumed := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	if resumed.Annotations[cellcontract.ActiveSnapshotAnnotation] != "" {
		t.Fatal("source Cell resumed before releasing the operation marker")
	}
}

func TestSnapshotFailureIsNotTerminalUntilCSIDeletionIsObserved(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	snapshot := &dshv1alpha1.CellSnapshot{
		TypeMeta: metav1.TypeMeta{APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "CellSnapshot"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cleanup-blocked", Namespace: "tenant-a", UID: testSnapshotUID, Generation: 1,
			Finalizers: []string{cellcontract.SnapshotFinalizer},
		},
		Spec: dshv1alpha1.CellSnapshotSpec{CellRef: dshv1alpha1.LocalCellReference{Name: "source"}, VolumeSnapshotClassName: "snapshot-class"},
		Status: dshv1alpha1.CellSnapshotStatus{
			ObservedGeneration: 1, SourceCellUID: string(testUID), SourceGeneration: 1,
			DSHVersion: cellcontract.DSHVersion, ImageDigest: testDigest, StorageClassName: "snapshot-storage",
			Conditions: []metav1.Condition{
				{Type: dshv1alpha1.ConditionSnapshotAccepted, Status: metav1.ConditionTrue, Reason: reasonSnapshotPrerequisites},
				{Type: dshv1alpha1.ConditionSnapshotWriterStopped, Status: metav1.ConditionTrue, Reason: reasonSnapshotWriterStopped},
			},
		},
	}
	cell := testCell("source", dshv1alpha1.RetentionRetain)
	cell.Annotations = map[string]string{cellcontract.ActiveSnapshotAnnotation: string(snapshot.UID)}
	names := cellcontract.ResourceNames(string(cell.UID))
	zero := int32(0)
	workload := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: names.Base, Namespace: cell.Namespace, Generation: 1},
		Spec:       appsv1.StatefulSetSpec{Replicas: &zero},
		Status:     appsv1.StatefulSetStatus{ObservedGeneration: 1},
	}
	setCellMetadata(workload, cell)
	workload.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "Cell", Name: cell.Name, UID: cell.UID, Controller: ptr.To(true),
	}}
	message := "fixture CSI failure"
	volumeSnapshot := readyVolumeSnapshot(snapshot, names.DataPVC, resource.MustParse("1Gi"))
	volumeSnapshot.Finalizers = []string{"snapshot.test/hold-deletion"}
	volumeSnapshot.Status.Error = &volumesnapshotv1.VolumeSnapshotError{Message: &message}
	cellReconciler, kube := testReconciler(t, cell, snapshot, workload, volumeSnapshot)
	reconciler := &CellSnapshotReconciler{
		Client: kube, Scheme: cellReconciler.Scheme,
		Config: SnapshotConfig{Enabled: true, WriterStopTimeout: 2 * time.Minute, SnapshotTimeout: 30 * time.Minute},
	}

	reconcileSnapshot(t, reconciler, snapshot, 1)
	pending := get[*dshv1alpha1.CellSnapshot](t, kube, snapshot.Namespace, snapshot.Name)
	failed := meta.FindStatusCondition(pending.Status.Conditions, dshv1alpha1.ConditionSnapshotFailed)
	if failed == nil || failed.Status != metav1.ConditionFalse || failed.Reason != reasonSnapshotCleanupBlocked {
		t.Fatalf("failure became terminal before CSI deletion: %#v", failed)
	}
	stillStopped := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	if stillStopped.Annotations[cellcontract.ActiveSnapshotAnnotation] != string(snapshot.UID) {
		t.Fatal("source operation marker was released before CSI deletion")
	}

	deleting := get[*volumesnapshotv1.VolumeSnapshot](t, kube, volumeSnapshot.Namespace, volumeSnapshot.Name)
	deleting.Finalizers = nil
	if err := kube.Update(ctx, deleting); err != nil && !apierrors.IsNotFound(err) {
		t.Fatal(err)
	}
	reconcileSnapshot(t, reconciler, snapshot, 2)
	terminal := get[*dshv1alpha1.CellSnapshot](t, kube, snapshot.Namespace, snapshot.Name)
	if !conditionTrue(terminal.Status.Conditions, dshv1alpha1.ConditionSnapshotFailed) {
		t.Fatalf("snapshot did not become terminal after cleanup: %#v", terminal.Status)
	}
	resumed := get[*dshv1alpha1.Cell](t, kube, cell.Namespace, cell.Name)
	if resumed.Annotations[cellcontract.ActiveSnapshotAnnotation] != "" {
		t.Fatal("source operation marker remained after confirmed cleanup")
	}
}

func TestSnapshotConfigRequiresBoundedMinimums(t *testing.T) {
	t.Parallel()
	for _, config := range []SnapshotConfig{
		{Enabled: true, WriterStopTimeout: 29 * time.Second, SnapshotTimeout: time.Minute},
		{Enabled: true, WriterStopTimeout: 30 * time.Second, SnapshotTimeout: 59 * time.Second},
	} {
		if err := config.Validate(); err == nil {
			t.Fatalf("invalid snapshot config was accepted: %#v", config)
		}
	}
	if err := (SnapshotConfig{Enabled: false}).Validate(); err != nil {
		t.Fatalf("disabled snapshot config required external settings: %v", err)
	}
	if err := (SnapshotConfig{Enabled: true, WriterStopTimeout: 30 * time.Second, SnapshotTimeout: time.Minute}).Validate(); err != nil {
		t.Fatalf("minimum snapshot config was rejected: %v", err)
	}
}

func readyVolumeSnapshot(snapshot *dshv1alpha1.CellSnapshot, source string, size resource.Quantity) *volumesnapshotv1.VolumeSnapshot {
	ready := true
	className := "snapshot-class"
	return &volumesnapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name: cellcontract.SnapshotName(string(snapshot.UID)), Namespace: snapshot.Namespace,
			Annotations: map[string]string{cellcontract.SnapshotUIDAnnotation: string(snapshot.UID)},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: dshv1alpha1.GroupVersion.String(), Kind: "CellSnapshot", Name: snapshot.Name, UID: snapshot.UID, Controller: ptr.To(true),
			}},
		},
		Spec: volumesnapshotv1.VolumeSnapshotSpec{
			VolumeSnapshotClassName: &className,
			Source:                  volumesnapshotv1.VolumeSnapshotSource{PersistentVolumeClaimName: &source},
		},
		Status: &volumesnapshotv1.VolumeSnapshotStatus{ReadyToUse: &ready, RestoreSize: &size},
	}
}

func reconcileSnapshot(t *testing.T, reconciler *CellSnapshotReconciler, snapshot *dshv1alpha1.CellSnapshot, count int) {
	t.Helper()
	request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(snapshot)}
	for range count {
		if _, err := reconciler.Reconcile(context.Background(), request); err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
}
