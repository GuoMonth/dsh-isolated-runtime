package scheduling

import (
	"context"
	"errors"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

var (
	// ErrUnschedulable is returned when no valid runtime allocation can be made.
	ErrUnschedulable = errors.New("scheduling: no runtime allocation available")
	// ErrInvalidRequest is returned when tenant/session/runtime identity is absent.
	ErrInvalidRequest = errors.New("scheduling: invalid allocation request")
)

// FirstFit reuses the first running runtime owned by the same tenant when reuse
// is allowed; otherwise it allocates the caller's desired new runtime name.
//
// It deliberately never chooses a Kubernetes Node.
type FirstFit struct {
	Runtimes []runtime.Info
}

var _ Allocator = (*FirstFit)(nil)

// Allocate returns a tenant-safe runtime allocation.
func (a *FirstFit) Allocate(ctx context.Context, req Request) (Allocation, error) {
	if req.Tenant == "" || req.Session == "" || req.DesiredRuntimeName == "" {
		return Allocation{}, ErrInvalidRequest
	}

	if req.AllowReuse {
		for _, candidate := range a.Runtimes {
			if candidate.Tenant != req.Tenant || candidate.Phase != "Running" {
				continue
			}
			if candidate.RuntimeClass != req.RuntimeClass ||
				candidate.SecurityClass != req.SecurityClass ||
				candidate.Image != req.Image {
				continue
			}
			return Allocation{RuntimeName: candidate.Name, Reuse: true}, nil
		}
	}

	return Allocation{RuntimeName: req.DesiredRuntimeName, Reuse: false}, nil
}
