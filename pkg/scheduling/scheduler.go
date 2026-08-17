// Package scheduling decides where a session runs (③).
package scheduling

import (
	"context"

	"github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
)

// Placement is the result of a scheduling decision. It is runtime-agnostic:
// a runtime name, not a Kubernetes binding.
type Placement struct {
	RuntimeName string
	Node        string
}

// Scheduler decides where a session runs (③).
type Scheduler interface {
	Place(ctx context.Context, constraints v1alpha1.SchedulingConstraints) (Placement, error)
}
