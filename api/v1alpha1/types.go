package v1alpha1

// TypeMeta describes an individual API object's type. It is a self-contained
// stand-in for metav1.TypeMeta; the M1 Kubernetes adapter intentionally uses a
// narrow REST boundary rather than importing Kubernetes Go types here.
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

// Runtime is the isolation boundary (①). Every Runtime belongs to exactly one
// tenant; a tenant may own multiple Runtimes, but a Runtime is never shared
// across tenants.
type Runtime struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       RuntimeSpec   `json:"spec,omitempty"`
	Status     RuntimeStatus `json:"status,omitempty"`
}

// RuntimeSpec describes the boundary to provision.
type RuntimeSpec struct {
	// Tenant is the immutable owner of the runtime.
	Tenant string `json:"tenant"`
	// RuntimeClass selects the concrete container/sandbox runtime.
	RuntimeClass string `json:"runtimeClass,omitempty"`
	// SecurityClass is a platform-defined posture such as standard or sandboxed.
	SecurityClass string `json:"securityClass,omitempty"`
	// Image is the OCI image realized by the M1 runtime Pod.
	Image string `json:"image"`
	// NetworkIsolation requests a dedicated network namespace with no
	// cross-tenant traffic by default.
	NetworkIsolation bool `json:"networkIsolation,omitempty"`
	// ResourceLimits caps CPU, memory, and optionally ephemeral storage.
	ResourceLimits map[string]string `json:"resourceLimits,omitempty"`
}

// RuntimeStatus is the observed state of a boundary.
type RuntimeStatus struct {
	// Phase is Pending, Running, Failed, or Terminating.
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
	// Workload is a platform profile family such as base, data, or dev.
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
	SecurityClass  string   `json:"securityClass,omitempty"`
	SeccompProfile string   `json:"seccompProfile,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty"`
}

// RuntimeSession binds a tenant and DSH session to a runtime (④ admission,
// ③ runtime allocation).
type RuntimeSession struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       RuntimeSessionSpec   `json:"spec,omitempty"`
	Status     RuntimeSessionStatus `json:"status,omitempty"`
}

// RuntimeSessionSpec is the desired state of a session binding.
type RuntimeSessionSpec struct {
	// Tenant is the owning tenant. Any bound Runtime must have the same owner.
	Tenant string `json:"tenant"`
	// Session is the DSH session identifier.
	Session string `json:"session"`
	// ImageRef and ProfileRef select the workload (②).
	ImageRef   string `json:"imageRef,omitempty"`
	ProfileRef string `json:"profileRef,omitempty"`
	// ReuseRuntime allows reuse only among runtimes owned by the same tenant.
	ReuseRuntime bool `json:"reuseRuntime,omitempty"`
	// Constraints are inputs to runtime allocation and the generated backend
	// specification. They are not a request for this project to choose a
	// Kubernetes Node directly.
	Constraints RuntimeConstraints `json:"constraints,omitempty"`
}

// RuntimeSessionStatus is the observed state of a session binding.
type RuntimeSessionStatus struct {
	// Phase is Pending, Allocated, Running, Persisted, or Terminated.
	Phase string `json:"phase,omitempty"`
	// RuntimeRef is the runtime this session is bound to.
	RuntimeRef string `json:"runtimeRef,omitempty"`
	// CheckpointRef is the last logical state snapshot (⑤), if any.
	CheckpointRef string `json:"checkpointRef,omitempty"`
}

// RuntimeConstraints are allocation inputs. Kubernetes-specific adapters may
// translate these into node selectors, affinity, tolerations, resource requests,
// RuntimeClass, and related Pod policy without exposing node placement as a
// control-plane decision.
type RuntimeConstraints struct {
	NodeSelector    map[string]string `json:"nodeSelector,omitempty"`
	Tolerations     []string          `json:"tolerations,omitempty"`
	ResourceRequest map[string]string `json:"resourceRequest,omitempty"`
}

// Checkpoint is a logical persisted session/workspace state (⑤), not a promise
// of kernel/process memory checkpointing.
type Checkpoint struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       CheckpointSpec   `json:"spec,omitempty"`
	Status     CheckpointStatus `json:"status,omitempty"`
}

// CheckpointSpec describes a logical state capture.
type CheckpointSpec struct {
	TenantRef  string `json:"tenantRef"`
	SessionRef string `json:"sessionRef"`
	StorageRef string `json:"storageRef,omitempty"`
	ImageRef   string `json:"imageRef,omitempty"`
	ProfileRef string `json:"profileRef,omitempty"`
}

// CheckpointStatus is the observed state of a logical capture.
type CheckpointStatus struct {
	// Phase is Pending, Capturing, Ready, or Restoring.
	Phase         string `json:"phase,omitempty"`
	RestoreTarget string `json:"restoreTarget,omitempty"`
}
