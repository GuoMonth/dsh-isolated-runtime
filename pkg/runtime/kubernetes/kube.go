// Package kubernetes is the first implementation of the runtime seam (①): a
// Kubernetes-backed boundary (one tenant, one Pod).
//
// It is currently an in-memory stub: it records boundaries and their
// checkpoints in memory. The real pod lifecycle and CRI integration land at
// M1–M3; this stub exists to lock the contract and let the control plane
// develop against the seam without a cluster.
package kubernetes

import (
	"context"
	"sync"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

// Backend implements runtime.Runtime with an in-memory Kubernetes stand-in.
type Backend struct {
	mu          sync.Mutex
	boundaries  map[string]runtime.Info
	checkpoints map[string]runtime.Info
}

// New returns an empty in-memory backend.
func New() *Backend {
	return &Backend{
		boundaries:  make(map[string]runtime.Info),
		checkpoints: make(map[string]runtime.Info),
	}
}

// Compile-time contract assertion: Backend must satisfy Runtime.
var _ runtime.Runtime = (*Backend)(nil)

// Create records a boundary and marks it Running.
func (b *Backend) Create(ctx context.Context, spec runtime.Spec) (*runtime.Info, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.boundaries[spec.Name]; ok {
		return nil, runtime.ErrConflict
	}
	info := runtime.Info{
		Name:    spec.Name,
		Phase:   "Running",
		Address: "pod://" + spec.Name,
	}
	b.boundaries[spec.Name] = info
	return &info, nil
}

// Get returns the observed state of a boundary.
func (b *Backend) Get(ctx context.Context, name string) (*runtime.Info, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	info, ok := b.boundaries[name]
	if !ok {
		return nil, runtime.ErrNotFound
	}
	return &info, nil
}

// Delete tears down a boundary.
func (b *Backend) Delete(ctx context.Context, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.boundaries[name]; !ok {
		return runtime.ErrNotFound
	}
	delete(b.boundaries, name)
	return nil
}

// Checkpoint snapshots the boundary into the in-memory checkpoint store.
// TODO(M3): replace with CRI / CRIU-based checkpoint.
func (b *Backend) Checkpoint(ctx context.Context, name string) (runtime.CheckpointRef, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	info, ok := b.boundaries[name]
	if !ok {
		return runtime.CheckpointRef{}, runtime.ErrNotFound
	}
	info.Phase = "Checkpointed"
	b.checkpoints[name] = info
	return runtime.CheckpointRef{Name: name}, nil
}

// Restore resumes a boundary from a captured state.
func (b *Backend) Restore(ctx context.Context, name string, ref runtime.CheckpointRef) (*runtime.Info, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp, ok := b.checkpoints[ref.Name]
	if !ok {
		return nil, runtime.ErrNotFound
	}
	cp.Phase = "Running"
	b.boundaries[name] = cp
	return &cp, nil
}
