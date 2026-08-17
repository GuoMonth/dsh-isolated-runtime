package v1alpha1

// Kind is a resource kind name.
type Kind = string

// Well-known resource kinds.
const (
	KindRuntime        Kind = "Runtime"
	KindRuntimeImage   Kind = "RuntimeImage"
	KindRuntimeProfile Kind = "RuntimeProfile"
	KindRuntimeSession Kind = "RuntimeSession"
	KindCheckpoint     Kind = "Checkpoint"
)

// resourcePlurals maps a kind to its CRD resource plural.
var resourcePlurals = map[Kind]string{
	KindRuntime:        "runtimes",
	KindRuntimeImage:   "runtimeimages",
	KindRuntimeProfile: "runtimeprofiles",
	KindRuntimeSession: "runtimesessions",
	KindCheckpoint:     "checkpoints",
}

// CRDName returns the canonical CRD name for a kind, e.g.
// "runtimes.runtime.dsh.io".
func CRDName(k Kind) string {
	return resourcePlurals[k] + "." + GroupName
}
