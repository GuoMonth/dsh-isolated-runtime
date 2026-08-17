package scheduling

import (
	"context"
	"errors"

	"github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
)

// ErrUnschedulable is returned when no candidate runtime satisfies the
// constraints.
var ErrUnschedulable = errors.New("scheduling: no runtime satisfies constraints")

// FirstFit binds a session to the first candidate runtime. It is a stub for the
// real scheduler (M1); candidate inventory is supplied explicitly.
type FirstFit struct {
	Runtimes []string
}

var _ Scheduler = (*FirstFit)(nil)

// Place returns the first candidate runtime, or ErrUnschedulable if none.
func (s *FirstFit) Place(ctx context.Context, c v1alpha1.SchedulingConstraints) (Placement, error) {
	if len(s.Runtimes) == 0 {
		return Placement{}, ErrUnschedulable
	}
	return Placement{RuntimeName: s.Runtimes[0]}, nil
}
