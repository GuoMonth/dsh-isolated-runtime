package checkpoint

import (
	"context"
	"errors"
	"sync"
)

// Memory is an in-memory Manager for development. TODO(M3): replace with a
// real mechanism (CRIU / runc/containerd checkpoint).
type Memory struct {
	mu        sync.Mutex
	snapshots map[string]Snapshot
}

// NewMemory returns an empty in-memory Manager.
func NewMemory() *Memory {
	return &Memory{snapshots: make(map[string]Snapshot)}
}

var _ Manager = (*Memory)(nil)

// Snapshot records a snapshot keyed by the runtime ref.
func (m *Memory) Snapshot(ctx context.Context, runtimeRef string) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Snapshot{ID: runtimeRef}
	m.snapshots[runtimeRef] = s
	return s, nil
}

// Restore verifies the snapshot exists; a real restore resumes the state.
func (m *Memory) Restore(ctx context.Context, runtimeRef string, s Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.snapshots[s.ID]; !ok {
		return errors.New("checkpoint: snapshot not found")
	}
	return nil
}
