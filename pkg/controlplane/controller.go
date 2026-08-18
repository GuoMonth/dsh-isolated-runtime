// Package controlplane orchestrates the six layers (⑥): it drives the session
// lifecycle across admission, runtime allocation, provisioning, and the
// isolation boundary.
package controlplane

import "context"

// Reconciler drives desired state toward actual state (⑥). The controller loop
// (M1) will invoke it on every RuntimeSession change.
type Reconciler interface {
	Reconcile(ctx context.Context, key string) error
}
