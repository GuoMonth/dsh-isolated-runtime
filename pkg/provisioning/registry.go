package provisioning

import "sync"

// InMemory is the default in-memory profile registry, seeded with the standard
// workloads (Terminal, Python, Node).
type InMemory struct {
	mu       sync.Mutex
	profiles map[string]Profile
}

// NewInMemory returns a registry seeded with standard defaults.
func NewInMemory() *InMemory {
	return &InMemory{profiles: map[string]Profile{
		"terminal": {Workload: "terminal", Image: "alpine:3.20", Seccomp: "runtime/default"},
		"python":   {Workload: "python", Image: "python:3.12-slim", Seccomp: "runtime/default"},
		"node":     {Workload: "node", Image: "node:20-slim", Seccomp: "runtime/default"},
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
