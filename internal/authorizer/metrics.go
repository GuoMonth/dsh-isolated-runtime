package authorizer

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var errRBACDenied = &decisionError{"rbac-denied"}

type decisionError struct {
	reason string
}

func (e *decisionError) Error() string { return e.reason }

// PrometheusDecisions contains the only project-specific Phase 4 metric. Cell,
// Snapshot, Route, and workload state stays in Kubernetes objects rather than
// being mirrored in process memory.
type PrometheusDecisions struct {
	counter *prometheus.CounterVec
}

// NewMetricsRegistry returns an isolated registry and a recorder whose only
// variable label is the closed Decision enumeration above.
func NewMetricsRegistry() (*prometheus.Registry, *PrometheusDecisions) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "dsh",
		Subsystem: "authorizer",
		Name:      "decisions_total",
		Help:      "Authorization decisions by bounded outcome class.",
	}, []string{"decision"})
	registry.MustRegister(counter)
	metrics := &PrometheusDecisions{counter: counter}
	for _, decision := range allDecisions {
		counter.WithLabelValues(string(decision)).Add(0)
	}
	return registry, metrics
}

func (m *PrometheusDecisions) RecordDecision(decision Decision) {
	if m == nil {
		return
	}
	if !validDecision(decision) {
		decision = DecisionDependencyError
	}
	m.counter.WithLabelValues(string(decision)).Inc()
}

func validDecision(candidate Decision) bool {
	for _, decision := range allDecisions {
		if candidate == decision {
			return true
		}
	}
	return false
}
