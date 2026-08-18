// Package gateway is the trusted admission primitive (④): authenticated
// transport establishes the Principal, then admission authorizes a target
// tenant/session and resolves its runtime fail-closed.
package gateway

import (
	"context"
	"errors"
)

// ErrDenied is returned whenever admission fails. Public callers receive one
// uniform denial; detailed reasons belong in server-side logs/audit.
var ErrDenied = errors.New("gateway: admission denied")

// Principal is an authenticated server-side identity. It is never decoded from
// an AdmissionRequest body.
type Principal struct {
	Subject string
}

type principalContextKey struct{}

// WithPrincipal attaches an authenticated principal to a trusted request
// context.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext returns the authenticated principal, if one was
// established by the transport boundary.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok && principal.Subject != ""
}

// AdmissionRequest identifies the target resource, not the caller identity.
type AdmissionRequest struct {
	Tenant  string `json:"tenant"`
	Session string `json:"session"`
}

// AdmissionDecision is the public outcome of admission.
type AdmissionDecision struct {
	Allowed    bool   `json:"allowed"`
	RuntimeRef string `json:"runtimeRef,omitempty"`
}

// Authorizer decides whether an authenticated principal may act for a tenant.
type Authorizer interface {
	Authorize(ctx context.Context, principal Principal, tenant string) error
}

// SessionResolver resolves a tenant+session to its runtime.
type SessionResolver interface {
	Resolve(ctx context.Context, tenant, session string) (string, error)
}

// Admitter authorizes the trusted principal, then resolves the runtime.
type Admitter struct {
	auth    Authorizer
	resolve SessionResolver
}

// NewAdmitter builds an Admitter. Either dependency may be nil; nil means deny.
func NewAdmitter(a Authorizer, r SessionResolver) *Admitter {
	return &Admitter{auth: a, resolve: r}
}

// Admit authorizes and resolves. Principal must already exist in the trusted
// server-side context.
func (a *Admitter) Admit(ctx context.Context, req AdmissionRequest) (AdmissionDecision, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok || a.auth == nil || a.resolve == nil || req.Tenant == "" || req.Session == "" {
		return AdmissionDecision{Allowed: false}, ErrDenied
	}
	if err := a.auth.Authorize(ctx, principal, req.Tenant); err != nil {
		return AdmissionDecision{Allowed: false}, ErrDenied
	}
	ref, err := a.resolve.Resolve(ctx, req.Tenant, req.Session)
	if err != nil || ref == "" {
		return AdmissionDecision{Allowed: false}, ErrDenied
	}
	return AdmissionDecision{Allowed: true, RuntimeRef: ref}, nil
}
