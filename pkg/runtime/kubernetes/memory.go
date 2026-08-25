package kubernetes

import (
	"context"
	"sync"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

// MemoryBackend is the deterministic in-memory backend used by contract and
// control-plane unit tests. Production code uses Backend/NewInCluster.
type MemoryBackend struct {
	mu         sync.Mutex
	boundaries map[string]runtime.Info
}

// New preserves the M0 test constructor. It returns an in-memory backend.
func New() *MemoryBackend { return NewMemory() }

// NewMemory returns an empty in-memory backend.
func NewMemory() *MemoryBackend {
	return &MemoryBackend{boundaries: make(map[string]runtime.Info)}
}

var _ runtime.Runtime = (*MemoryBackend)(nil)

func (b *MemoryBackend) Create(_ context.Context, spec runtime.Spec) (*runtime.Info, error) {
	if spec.Name == "" || spec.Tenant == "" {
		return nil, runtime.ErrInvalidSpec
	}
	if spec.SecurityClass == "" {
		spec.SecurityClass = runtime.SecurityStandard
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

func (b *MemoryBackend) Get(_ context.Context, tenant, name string) (*runtime.Info, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	info, ok := b.boundaries[name]
	if !ok || info.Tenant != tenant {
		return nil, runtime.ErrNotFound
	}
	copy := info
	return &copy, nil
}

func (b *MemoryBackend) List(_ context.Context, tenant string) ([]runtime.Info, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]runtime.Info, 0, len(b.boundaries))
	for _, info := range b.boundaries {
		if tenant != "" && info.Tenant != tenant {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

func (b *MemoryBackend) Delete(_ context.Context, tenant, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	info, ok := b.boundaries[name]
	if !ok || info.Tenant != tenant {
		return runtime.ErrNotFound
	}
	delete(b.boundaries, name)
	return nil
}
