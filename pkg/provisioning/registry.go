package provisioning

import (
	"sync"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

// InMemory is the default in-memory profile registry. Image names are local
// placeholders until M4 publishes the standard DSH runtime images.
type InMemory struct {
	mu       sync.Mutex
	profiles map[string]Profile
}

// NewInMemory returns a registry seeded with standard DSH runtime profiles.
func NewInMemory() *InMemory {
	return &InMemory{profiles: map[string]Profile{
		"base": {
			Workload:      "base",
			Image:         "dsh-runtime:base",
			SecurityClass: runtime.SecurityStandard,
			Seccomp:       "runtime/default",
		},
		"data": {
			Workload:      "data",
			Image:         "dsh-runtime:data",
			SecurityClass: runtime.SecuritySandboxed,
			Seccomp:       "runtime/default",
		},
		"dev": {
			Workload:      "dev",
			Image:         "dsh-runtime:dev",
			SecurityClass: runtime.SecuritySandboxed,
			Seccomp:       "runtime/default",
		},
	}}
}

var _ Registry = (*InMemory)(nil)

// Resolve returns the profile for a workload.
func (r *InMemory) Resolve(workload string) (Profile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.profiles[workload]
	if !ok {
		return Profile{}, ErrUnknownWorkload
	}
	return p, nil
}

// Register installs (or replaces) a profile.
func (r *InMemory) Register(p Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profiles[p.Workload] = p
	return nil
}
