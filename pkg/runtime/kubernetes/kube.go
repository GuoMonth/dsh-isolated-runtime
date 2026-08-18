// Package kubernetes is the first implementation of the runtime seam (①).
//
// It is currently an in-memory stand-in. Real Pod lifecycle integration lands
// at M1; Kubernetes-specific behavior stays behind this adapter boundary.
package kubernetes

import (
	"context"
	"sync"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

// Backend implements runtime.Runtime with an in-memory Kubernetes stand-in.
type Backend struct {
	mu         sync.Mutex
	boundaries map[string]runtime.Info
}

// New returns an empty in-memory backend.
func New() *Backend {
	return &Backend{boundaries: make(map[string]runtime.Info)}
}

// Compile-time contract assertion: Backend must satisfy Runtime.
var _ runtime.Runtime = (*Backend)(nil)

// Create records a tenant-owned boundary and marks it Running.
func (b *Backend) Create(ctx context.Context, spec runtime.Spec) (*runtime.Info, error) {
	if spec.Name == "" || spec.Tenant == "" {
		return nil, runtime.ErrInvalidSpec
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.boundaries[spec.Name]; ok {
		return nil, runtime.ErrConflict
	}
	info := runtime.Info{
		Name:          spec.Name,
		Tenant:        spec.Tenant,
		RuntimeClass:  spec.RuntimeClass,
		SecurityClass: spec.SecurityClass,
		Image:         spec.Image,
		Phase:         "Running",
		Address:       "pod://" + spec.Name,
	}
	b.boundaries[spec.Name] = info
	copy := info
	return &copy, nil
}

// Get returns the observed state only when the requested tenant owns it.
func (b *Backend) Get(ctx context.Context, tenant, name string) (*runtime.Info, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	info, ok := b.boundaries[name]
	if !ok || info.Tenant != tenant {
		return nil, runtime.ErrNotFound
	}
	copy := info
	return &copy, nil
}

// Delete tears down a boundary only for its owning tenant.
func (b *Backend) Delete(ctx context.Context, tenant, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	info, ok := b.boundaries[name]
	if !ok || info.Tenant != tenant {
		return runtime.ErrNotFound
	}
	delete(b.boundaries, name)
	return nil
}
