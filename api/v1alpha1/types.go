package v1alpha1

// TypeMeta describes an individual API object's type. It is a self-contained
// stand-in for metav1.TypeMeta; the real Kubernetes types are adopted at M1.
type TypeMeta struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
}

// ObjectMeta is a minimal subset of Kubernetes ObjectMeta, sufficient for the
// control-plane contract.
type ObjectMeta struct {
	Name        string            `json:"name,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	UID         string            `json:"uid,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Generation  int64             `json:"generation,omitempty"`
}

// Runtime is the isolation boundary (①): one tenant, one runtime. It is the
// root object of the control plane — nothing above it carries the isolation
// invariant as a shared-code convention.
type Runtime struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       RuntimeSpec   `json:"spec,omitempty"`
	Status     RuntimeStatus `json:"status,omitempty"`
}

// RuntimeSpec describes the boundary to provision.
type RuntimeSpec struct {
	// RuntimeClass names the container-runtime class that backs this boundary
	// (a Kubernetes RuntimeClass, or a backend-specific equivalent).
	RuntimeClass string `json:"runtimeClass,omitempty"`
	// NetworkIsolation, when true, requests a dedicated network namespace with
	// no cross-tenant egress by default.
	NetworkIsolation bool `json:"networkIsolation,omitempty"`
	// ResourceLimits caps CPU and memory for the boundary
	// (e.g. cpu=1, memory=512Mi).
	ResourceLimits map[string]string `json:"resourceLimits,omitempty"`
}

// RuntimeStatus is the observed state of a boundary.
type RuntimeStatus struct {
	// Phase is Pending, Running, Checkpointed, or Terminated.
	Phase string `json:"phase,omitempty"`
	// RuntimeRef is the backend's identifier for this boundary.
	RuntimeRef string `json:"runtimeRef,omitempty"`
	// Message carries a human-readable status detail.
	Message string `json:"message,omitempty"`
}

// RuntimeImage is a workload's base image (②).
type RuntimeImage struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       RuntimeImageSpec `json:"spec,omitempty"`
}

// RuntimeImageSpec describes a workload's base image.
type RuntimeImageSpec struct {
	// Workload is one of terminal, python, node, ….
	Workload string `json:"workload"`
	// Image is the OCI image reference.
	Image string `json:"image"`
	// Digest pins the image content (optional).
	Digest string `json:"digest,omitempty"`
}

// RuntimeProfile is the versioned profile contract (②): how a workload runs,
// including its security posture.
type RuntimeProfile struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       RuntimeProfileSpec `json:"spec,omitempty"`
}

// RuntimeProfileSpec describes how a workload runs.
type RuntimeProfileSpec struct {
	Workload       string   `json:"workload"`
	ImageRef       string   `json:"imageRef"`
	SeccompProfile string   `json:"seccompProfile,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty"`
}

// RuntimeSession binds a tenant and session to a runtime (④ admission,
// ③ scheduling).
type RuntimeSession struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       RuntimeSessionSpec   `json:"spec,omitempty"`
	Status     RuntimeSessionStatus `json:"status,omitempty"`
}

// RuntimeSessionSpec is the desired state of a session binding.
type RuntimeSessionSpec struct {
	// Tenant is the owning tenant; two sessions of different tenants never
	// share a runtime.
	Tenant string `json:"tenant"`
	// Session is the DSH session identifier.
	Session string `json:"session"`
	// ImageRef and ProfileRef select the workload (②).
	ImageRef   string `json:"imageRef,omitempty"`
	ProfileRef string `json:"profileRef,omitempty"`
	// Constraints is the scheduling input (③).
	Constraints SchedulingConstraints `json:"constraints,omitempty"`
}

// RuntimeSessionStatus is the observed state of a session binding.
type RuntimeSessionStatus struct {
	// Phase is Pending, Scheduled, Running, Checkpointed, or Terminated.
	Phase string `json:"phase,omitempty"`
	// RuntimeRef is the runtime this session is bound to.
	RuntimeRef string `json:"runtimeRef,omitempty"`
	// CheckpointRef is the last captured state (⑤), if any.
	CheckpointRef string `json:"checkpointRef,omitempty"`
}

// SchedulingConstraints is the placement input for the Scheduler (③).
type SchedulingConstraints struct {
	NodeSelector    map[string]string `json:"nodeSelector,omitempty"`
	Tolerations     []string          `json:"tolerations,omitempty"`
	ResourceRequest map[string]string `json:"resourceRequest,omitempty"`
}

// Checkpoint is a captured runtime state (⑤).
type Checkpoint struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       CheckpointSpec   `json:"spec,omitempty"`
	Status     CheckpointStatus `json:"status,omitempty"`
}

// CheckpointSpec describes a capture.
type CheckpointSpec struct {
	SessionRef string `json:"sessionRef"`
	StorageRef string `json:"storageRef,omitempty"`
}

// CheckpointStatus is the observed state of a capture.
type CheckpointStatus struct {
	// Phase is Pending, Capturing, Ready, or Restoring.
	Phase         string `json:"phase,omitempty"`
	RestoreTarget string `json:"restoreTarget,omitempty"`
}
