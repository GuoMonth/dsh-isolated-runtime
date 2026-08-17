// Package gateway is the admission point (④): it authenticates, authorizes,
// and resolves a session to its runtime — fail-closed.
package gateway

import (
	"context"
	"errors"
)

// ErrDenied is returned whenever admission fails. The Gateway is fail-closed:
// anything it cannot positively resolve is denied.
var ErrDenied = errors.New("gateway: admission denied")

// AdmissionRequest is what the Gateway receives for a session.
type AdmissionRequest struct {
	// Tenant is the owning tenant.
	Tenant string `json:"tenant"`
	// Session is the DSH session identifier.
	Session string `json:"session"`
	// Principal is the authenticated identity. It is opaque to this layer.
	Principal string `json:"principal"`
}

// AdmissionDecision is the outcome of admission.
type AdmissionDecision struct {
	Allowed    bool   `json:"allowed"`
	Reason     string `json:"reason,omitempty"`
	RuntimeRef string `json:"runtimeRef,omitempty"`
}

// Authorizer decides whether a principal may act for a tenant. A nil
// Authorizer denies everything.
type Authorizer interface {
	Authorize(ctx context.Context, principal, tenant string) error
}

// SessionResolver resolves a tenant+session to its runtime. A nil resolver
// denies everything.
type SessionResolver interface {
	Resolve(ctx context.Context, tenant, session string) (string, error)
}

// Admitter authorizes the principal, then resolves the runtime.
type Admitter struct {
	auth    Authorizer
	resolve SessionResolver
}

// NewAdmitter builds an Admitter. Either dependency may be nil; nil means deny.
func NewAdmitter(a Authorizer, r SessionResolver) *Admitter {
	return &Admitter{auth: a, resolve: r}
}

// Admit authorizes and resolves. It returns a non-nil error unless both
// succeed — the Gateway never admits on partial information.
func (a *Admitter) Admit(ctx context.Context, req AdmissionRequest) (AdmissionDecision, error) {
	if a.auth == nil || a.resolve == nil {
		return AdmissionDecision{Allowed: false, Reason: "no authorizer/resolver configured"}, ErrDenied
	}
	if err := a.auth.Authorize(ctx, req.Principal, req.Tenant); err != nil {
		return AdmissionDecision{Allowed: false, Reason: err.Error()}, ErrDenied
	}
	ref, err := a.resolve.Resolve(ctx, req.Tenant, req.Session)
	if err != nil {
		return AdmissionDecision{Allowed: false, Reason: "session unresolved"}, ErrDenied
	}
	return AdmissionDecision{Allowed: true, RuntimeRef: ref}, nil
}
