// Package provisioning resolves a workload profile to a DSH runtime image and
// security posture (②).
package provisioning

import (
	"errors"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

// Profile is the versioned runtime profile contract.
//
// Every standard profile is a DSH runtime image; profiles add workload tooling
// rather than replacing DSH with a bare language image.
type Profile struct {
	Workload      string
	Image         string
	SecurityClass runtime.SecurityClass
	Seccomp       string
	Capabilities  []string
}

// Registry resolves workloads to profiles.
type Registry interface {
	Resolve(workload string) (Profile, error)
	Register(p Profile) error
}

// ErrUnknownWorkload is returned when no profile is registered for a workload.
var ErrUnknownWorkload = errors.New("provisioning: unknown workload")
