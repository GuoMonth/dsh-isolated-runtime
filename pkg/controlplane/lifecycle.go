package controlplane

import (
	"context"
	"errors"

	"github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/scheduling"
)

var ErrTenantOwnershipMismatch = errors.New("controlplane: runtime tenant ownership mismatch")

// Lifecycle orchestrates create/delete across ③ runtime allocation and ① the
// isolation boundary (⑥).
type Lifecycle struct {
	rt        runtime.Runtime
	allocator scheduling.Allocator
}

// New builds a Lifecycle.
func New(rt runtime.Runtime, allocator scheduling.Allocator) *Lifecycle {
	return &Lifecycle{rt: rt, allocator: allocator}
}

// CreateSessionRequest is the control-plane input for creating or reusing a
// tenant-owned runtime for one DSH session.
type CreateSessionRequest struct {
	Tenant      string
	Session     string
	AllowReuse  bool
	Constraints v1alpha1.RuntimeConstraints
	Runtime     runtime.Spec
}

// CreateSession allocates a tenant-owned runtime and provisions it when needed.
func (l *Lifecycle) CreateSession(ctx context.Context, req CreateSessionRequest) (*runtime.Info, error) {
	if req.Runtime.Tenant != "" && req.Runtime.Tenant != req.Tenant {
		return nil, ErrTenantOwnershipMismatch
	}
	req.Runtime.Tenant = req.Tenant

	allocation, err := l.allocator.Allocate(ctx, scheduling.Request{
		Tenant:             req.Tenant,
		Session:            req.Session,
		DesiredRuntimeName: req.Runtime.Name,
		AllowReuse:         req.AllowReuse,
		RuntimeClass:       req.Runtime.RuntimeClass,
		SecurityClass:      req.Runtime.SecurityClass,
		Image:              req.Runtime.Image,
		Constraints:        req.Constraints,
	})
	if err != nil {
		return nil, err
	}

	if allocation.Reuse {
		return l.rt.Get(ctx, req.Tenant, allocation.RuntimeName)
	}

	req.Runtime.Name = allocation.RuntimeName
	return l.rt.Create(ctx, req.Runtime)
}

// DeleteSession tears down the boundary only for its owning tenant.
func (l *Lifecycle) DeleteSession(ctx context.Context, tenant, name string) error {
	return l.rt.Delete(ctx, tenant, name)
}
