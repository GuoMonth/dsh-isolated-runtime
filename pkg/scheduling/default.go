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

// FirstFit reuses the first compatible running runtime owned by the same tenant
// when reuse is allowed; otherwise it allocates the requested new runtime name.
// Production callers should provide Inventory; Runtimes remains as a small test
// seam and compatibility path for the M0 scaffold.
type FirstFit struct {
	Inventory Inventory
	Runtimes  []runtime.Info
}

var _ Allocator = (*FirstFit)(nil)

func (a *FirstFit) Allocate(ctx context.Context, req Request) (Allocation, error) {
	if req.Tenant == "" || req.Session == "" || req.DesiredRuntimeName == "" {
		return Allocation{}, ErrInvalidRequest
	}

	if req.AllowReuse {
		candidates := a.Runtimes
		if a.Inventory != nil {
			live, err := a.Inventory.List(ctx, req.Tenant)
			if err != nil {
				return Allocation{}, err
			}
			candidates = live
		}
		for _, candidate := range candidates {
			if candidate.Tenant != req.Tenant || candidate.Phase != "Running" {
				continue
			}
			if candidate.RuntimeClass != req.RuntimeClass || candidate.SecurityClass != req.SecurityClass || candidate.Image != req.Image {
				continue
			}
			return Allocation{RuntimeName: candidate.Name, Reuse: true}, nil
		}
	}

	return Allocation{RuntimeName: req.DesiredRuntimeName, Reuse: false}, nil
}
