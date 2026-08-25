package kubernetes

import (
	"encoding/json"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

// MarshalJSON keeps networkIsolation explicit even when false. The Runtime CRD
// defaults an omitted field to true, so omitting false would silently change a
// caller's requested desired state at the Kubernetes API boundary.
func (s runtimeCRSpec) MarshalJSON() ([]byte, error) {
	type wire struct {
		Tenant           string                `json:"tenant"`
		RuntimeClass     string                `json:"runtimeClass,omitempty"`
		SecurityClass    runtime.SecurityClass `json:"securityClass,omitempty"`
		Image            string                `json:"image"`
		NetworkIsolation bool                  `json:"networkIsolation"`
		ResourceLimits   map[string]string     `json:"resourceLimits,omitempty"`
	}
	return json.Marshal(wire(s))
}
