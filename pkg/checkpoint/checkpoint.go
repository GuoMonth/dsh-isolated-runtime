// Package checkpoint captures and restores a session's runtime state (⑤).
//
// The concrete mechanism (CRIU vs runc/containerd checkpoint vs in-process
// snapshot) is decision-gated; this is the contract that mechanism must
// satisfy.
package checkpoint

import "context"

// Snapshot is an opaque handle to a captured runtime state.
type Snapshot struct {
	ID string
}

// Manager captures and restores runtime state.
type Manager interface {
	Snapshot(ctx context.Context, runtimeRef string) (Snapshot, error)
	Restore(ctx context.Context, runtimeRef string, s Snapshot) error
}
