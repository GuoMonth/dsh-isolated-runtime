// Package checkpoint owns logical session/workspace persistence and restore (⑤).
//
// M0.1 deliberately does not promise CRIU/container-memory checkpointing. The
// first implementation should persist portable logical state (DSH/session
// state, workspace, artifacts, and a versioned manifest) to object storage and
// restore it into a fresh runtime.
package checkpoint

import "context"

// Snapshot is an opaque handle to a persisted logical state capture.
type Snapshot struct {
	ID string
}

// Manager is the single authority for logical state capture and restore.
type Manager interface {
	Snapshot(ctx context.Context, runtimeRef string) (Snapshot, error)
	Restore(ctx context.Context, runtimeRef string, s Snapshot) error
}
