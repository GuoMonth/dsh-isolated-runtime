// Package provisioning resolves a workload to its runtime image and profile (②).
package provisioning

import "errors"

// Profile is the versioned profile contract: what runs for a workload and how.
type Profile struct {
	Workload     string   // terminal | python | node | …
	Image        string   // OCI image reference
	Seccomp      string   // seccomp profile (e.g. runtime/default)
	Capabilities []string // permitted capabilities
}

// Registry resolves workloads to profiles.
type Registry interface {
	Resolve(workload string) (Profile, error)
	Register(p Profile) error
}

// ErrUnknownWorkload is returned when no profile is registered for a workload.
var ErrUnknownWorkload = errors.New("provisioning: unknown workload")
