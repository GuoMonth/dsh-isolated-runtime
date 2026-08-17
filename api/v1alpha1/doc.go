// Package v1alpha1 defines the versioned API types for the
// dsh-isolated-runtime control plane.
//
// The types here are the public contract the six layers operate on. They are
// self-contained stand-ins for Kubernetes API types; the real metav1 /
// controller-runtime types are adopted at M1, when the controller is wired
// against the cluster.
package v1alpha1

// GroupName is the API group for isolated-runtime resources.
const GroupName = "runtime.dsh.io"

// Version is the current API version.
const Version = "v1alpha1"
