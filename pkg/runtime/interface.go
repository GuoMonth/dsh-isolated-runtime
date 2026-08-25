// Package runtime defines the isolation-boundary seam (①).
//
// Kubernetes is the first backend. The abstraction is intentionally small and
// provisional until a second backend proves the common contract.
package runtime

import (
	"context"
	"errors"
)

// SecurityClass is a platform-defined isolation posture.
type SecurityClass string

const (
	// SecurityStandard uses the platform's hardened standard container policy.
	SecurityStandard SecurityClass = "standard"
	// SecuritySandboxed requests a stronger sandbox RuntimeClass for hostile
	// arbitrary-code workloads.
	SecuritySandboxed SecurityClass = "sandboxed"
)

// Spec describes the boundary to provision.
type Spec struct {
	Name             string            `json:"name"`
	Tenant           string            `json:"tenant"`
	RuntimeClass     string            `json:"runtimeClass,omitempty"`
	SecurityClass    SecurityClass     `json:"securityClass,omitempty"`
	Image            string            `json:"image,omitempty"`
	NetworkIsolation bool              `json:"networkIsolation,omitempty"`
	ResourceLimits   map[string]string `json:"resourceLimits,omitempty"`
}

// Info is the observed state of a boundary.
type Info struct {
	Name          string        `json:"name"`
	Tenant        string        `json:"tenant"`
	RuntimeClass  string        `json:"runtimeClass,omitempty"`
	SecurityClass SecurityClass `json:"securityClass,omitempty"`
	Image         string        `json:"image,omitempty"`
	Phase         string        `json:"phase"` // Pending, Running, Failed, Terminating
	Address       string        `json:"address,omitempty"`
	Message       string        `json:"message,omitempty"`
}

// Runtime is the isolation-boundary seam (①).
//
// Ownership is part of the contract: every runtime has exactly one tenant.
// Tenant-aware lookup/delete deliberately return ErrNotFound for a foreign
// tenant so callers cannot use this seam to enumerate another tenant's runtime.
// List with an empty tenant is reserved for trusted control-plane/admin callers.
type Runtime interface {
	Create(ctx context.Context, spec Spec) (*Info, error)
	Get(ctx context.Context, tenant, name string) (*Info, error)
	List(ctx context.Context, tenant string) ([]Info, error)
	Delete(ctx context.Context, tenant, name string) error
}

// Sentinel errors returned by backends.
var (
	ErrInvalidSpec = errors.New("runtime: invalid spec")
	ErrNotFound    = errors.New("runtime: not found")
	ErrConflict    = errors.New("runtime: already exists")
)
