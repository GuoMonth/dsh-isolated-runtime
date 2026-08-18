// Package scheduling allocates tenant sessions to tenant-owned runtimes (③).
//
// Despite the package/process name, this layer does not choose Kubernetes
// Nodes. Kubernetes remains responsible for Pod-to-Node scheduling.
package scheduling

import (
	"context"

	"github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
)

// Request is the input to runtime allocation.
type Request struct {
	Tenant             string
	Session            string
	DesiredRuntimeName string
	AllowReuse         bool
	Constraints        v1alpha1.RuntimeConstraints
}

// Allocation is the result of deciding whether to reuse or create a runtime.
type Allocation struct {
	RuntimeName string
	Reuse       bool
}

// Allocator decides which tenant-owned runtime should serve a session.
type Allocator interface {
	Allocate(ctx context.Context, req Request) (Allocation, error)
}
