package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecurityClass selects an operator-defined isolation posture. It deliberately
// does not expose RuntimeClass or arbitrary Pod security knobs.
// +kubebuilder:validation:Enum=standard;sandboxed
type SecurityClass string

const (
	SecurityStandard  SecurityClass = "standard"
	SecuritySandboxed SecurityClass = "sandboxed"
)

// RetentionPolicy controls what happens to Cell data after Cell deletion.
// +kubebuilder:validation:Enum=Retain;Delete
type RetentionPolicy string

const (
	RetentionRetain RetentionPolicy = "Retain"
	RetentionDelete RetentionPolicy = "Delete"
)

const (
	ConditionReady         = "Ready"
	ConditionStorageReady  = "StorageReady"
	ConditionWorkloadReady = "WorkloadReady"
	ConditionAccessReady   = "AccessReady"
)

const (
	ConditionSnapshotAccepted      = "Accepted"
	ConditionSnapshotWriterStopped = "WriterStopped"
	ConditionSnapshotReady         = ConditionReady
	ConditionSnapshotFailed        = "Failed"
)

// LocalCellSnapshotReference names a CellSnapshot in the Cell namespace.
type LocalCellSnapshotReference struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// CellStorageSpec describes the single tenant-data volume. DSH home state and
// the workspace live on this volume; credentials use a separate owner.
// +kubebuilder:validation:XValidation:rule="quantity(self.size).isGreaterThan(quantity('0'))",message="storage size must be positive"
// +kubebuilder:validation:XValidation:rule="quantity(self.size).compareTo(quantity(oldSelf.size)) >= 0",message="storage size cannot decrease"
// +kubebuilder:validation:XValidation:rule="has(self.storageClassName) == has(oldSelf.storageClassName) && (!has(self.storageClassName) || self.storageClassName == oldSelf.storageClassName)",message="storageClassName is immutable"
// +kubebuilder:validation:XValidation:rule="self.retentionPolicy == oldSelf.retentionPolicy",message="retentionPolicy is immutable"
// +kubebuilder:validation:XValidation:rule="has(self.restoreFrom) == has(oldSelf.restoreFrom) && (!has(self.restoreFrom) || self.restoreFrom == oldSelf.restoreFrom)",message="restoreFrom is immutable"
type CellStorageSpec struct {
	// Size is the requested data-volume capacity and may only grow.
	Size resource.Quantity `json:"size"`
	// StorageClassName selects a CSI storage class. Once set, it is immutable.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
	// RetentionPolicy defaults to Retain so deleting a Cell cannot silently
	// destroy tenant data. It is immutable after creation so deletion ownership
	// is decided once without a finalizer or deletion-time race.
	// +kubebuilder:default=Retain
	RetentionPolicy RetentionPolicy `json:"retentionPolicy,omitempty"`
	// RestoreFrom creates the initial data PVC from a Ready CellSnapshot in
	// this namespace. It is creation-only because PVC dataSource is immutable.
	// +optional
	RestoreFrom *LocalCellSnapshotReference `json:"restoreFrom,omitempty"`
}

// CellResources exposes only ordinary compute requests and limits. Dynamic
// resource claims and other Pod-level mechanisms stay outside the Cell API.
type CellResources struct {
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

// LocalSecretReference names a Secret in the same namespace as the Cell.
type LocalSecretReference struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// CellSpec is intentionally smaller than PodSpec. Every field represents Cell
// intent; Kubernetes retains ownership of placement and workload mechanics.
type CellSpec struct {
	// Image is an OCI content reference pinned by digest, never a floating tag.
	// +kubebuilder:validation:Pattern=`^[^\s@]+@sha256:[a-f0-9]{64}$`
	Image string `json:"image"`
	// SecurityClass selects standard containers or an operator-configured
	// sandboxed RuntimeClass.
	// +kubebuilder:default=standard
	SecurityClass SecurityClass `json:"securityClass,omitempty"`
	// Resources are the standard Kubernetes requests and limits for DSH.
	// +optional
	Resources CellResources `json:"resources,omitempty"`
	// Storage is durable DSH and workspace data.
	Storage CellStorageSpec `json:"storage"`
	// CredentialsRef names a Secret in the Cell namespace whose keys are
	// injected as DSH provider environment variables. Secret material never
	// appears in the Cell or its data snapshots.
	// +optional
	CredentialsRef *LocalSecretReference `json:"credentialsRef,omitempty"`
}

// CellStatus is topology-free observed state. It never exposes Pod, Service,
// Node, revision, or internal network addresses.
type CellStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:XValidation:rule="self.all(c, c.type in ['Ready', 'StorageReady', 'WorkloadReady', 'AccessReady'])",message="unsupported Cell condition type"
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	DSHVersion string             `json:"dshVersion,omitempty"`
	// +kubebuilder:validation:Pattern=`^sha256:[a-f0-9]{64}$`
	ImageDigest string `json:"imageDigest,omitempty"`
}

// Cell is the stable tenant-owned trust and state boundary. Its namespace is
// its tenant identity; no second tenant field exists to drift or be spoofed.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=dshcell
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="DSH",type="string",JSONPath=".status.dshVersion"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Cell struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CellSpec   `json:"spec"`
	Status CellStatus `json:"status,omitempty"`
}

// CellList contains Cells in one or more tenant namespaces.
// +kubebuilder:object:root=true
type CellList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cell `json:"items"`
}

// LocalCellReference names a Cell in the same namespace.
type LocalCellReference struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// CellSnapshotSpec requests one writer-stopped, crash-consistent snapshot of a
// Cell's tenant-data PVC. The complete request is immutable.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="CellSnapshot spec is immutable"
type CellSnapshotSpec struct {
	CellRef LocalCellReference `json:"cellRef"`
	// VolumeSnapshotClassName is cluster-owned and must use the same CSI driver
	// as the source data PVC's StorageClass.
	// +kubebuilder:validation:MinLength=1
	VolumeSnapshotClassName string `json:"volumeSnapshotClassName"`
}

// CellSnapshotStatus is durable, topology-free evidence of one snapshot
// operation. It never exposes Pod identity, IP addresses, Nodes, or secrets.
type CellSnapshotStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:XValidation:rule="self.all(c, c.type in ['Accepted', 'WriterStopped', 'Ready', 'Failed'])",message="unsupported CellSnapshot condition type"
	Conditions       []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	SourceCellUID    string             `json:"sourceCellUID,omitempty"`
	SourcePVCUID     string             `json:"sourcePVCUID,omitempty"`
	SourceGeneration int64              `json:"sourceGeneration,omitempty"`
	DSHVersion       string             `json:"dshVersion,omitempty"`
	// +kubebuilder:validation:Pattern=`^sha256:[a-f0-9]{64}$`
	ImageDigest      string `json:"imageDigest,omitempty"`
	StorageClassName string `json:"storageClassName,omitempty"`
	// +optional
	RestoreSize *resource.Quantity `json:"restoreSize,omitempty"`
}

// CellSnapshot is the single public trigger for stopping a Cell writer and
// creating a crash-consistent CSI snapshot of its tenant-data PVC.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=dshsnap
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Source",type="string",JSONPath=".spec.cellRef.name"
// +kubebuilder:printcolumn:name="DSH",type="string",JSONPath=".status.dshVersion"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type CellSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CellSnapshotSpec   `json:"spec"`
	Status CellSnapshotStatus `json:"status,omitempty"`
}

// CellSnapshotList contains CellSnapshots in one or more tenant namespaces.
// +kubebuilder:object:root=true
type CellSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CellSnapshot `json:"items"`
}
