// Package runtime defines the runtime-agnostic isolation-boundary seam (①).
//
// The control plane depends on this interface and nothing else. Kubernetes is
// the first backend; Docker, Firecracker, and Nomad may implement the same
// interface later. This is what keeps "runtime-agnostic" honest: a backend is
// proven by the contract, not by fiat.
package runtime

import (
	"context"
	"errors"
)

// Spec describes the boundary to provision.
type Spec struct {
	Name             string
	RuntimeClass     string
	Image            string
	NetworkIsolation bool
	ResourceLimits   map[string]string
}

// Info is the observed state of a boundary.
type Info struct {
	Name    string
	Phase   string // Pending, Running, Checkpointed, Terminated
	Address string // backend-local address (e.g. pod://name)
}

// CheckpointRef is an opaque handle to a captured runtime state.
type CheckpointRef struct {
	Name string
}

// Runtime is the isolation-boundary seam (①). Every backend implements it.
type Runtime interface {
	Create(ctx context.Context, spec Spec) (*Info, error)
	Get(ctx context.Context, name string) (*Info, error)
	Delete(ctx context.Context, name string) error
	Checkpoint(ctx context.Context, name string) (CheckpointRef, error)
	Restore(ctx context.Context, name string, ref CheckpointRef) (*Info, error)
}

// Sentinel errors returned by backends.
var (
	ErrNotFound = errors.New("runtime: not found")
	ErrConflict = errors.New("runtime: already exists")
)
