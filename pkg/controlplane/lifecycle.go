package controlplane

import (
	"context"

	"github.com/GuoMonth/dsh-isolated-runtime/api/v1alpha1"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
	"github.com/GuoMonth/dsh-isolated-runtime/pkg/scheduling"
)

// Lifecycle orchestrates create/resume/delete across ③ scheduling and ① the
// isolation boundary (⑥).
type Lifecycle struct {
	rt    runtime.Runtime
	sched scheduling.Scheduler
}

// New builds a Lifecycle.
func New(rt runtime.Runtime, sched scheduling.Scheduler) *Lifecycle {
	return &Lifecycle{rt: rt, sched: sched}
}

// CreateSession places a session (③) and provisions its boundary (①).
func (l *Lifecycle) CreateSession(ctx context.Context, constraints v1alpha1.SchedulingConstraints, spec runtime.Spec) (*runtime.Info, error) {
	p, err := l.sched.Place(ctx, constraints)
	if err != nil {
		return nil, err
	}
	spec.Name = p.RuntimeName
	return l.rt.Create(ctx, spec)
}

// DeleteSession tears down the boundary.
func (l *Lifecycle) DeleteSession(ctx context.Context, name string) error {
	return l.rt.Delete(ctx, name)
}
