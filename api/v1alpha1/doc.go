// Package v1alpha1 contains the Kubernetes API contract for DSH Cells.
//
// +kubebuilder:object:generate=true
// +groupName=dsh.isolated.io
//
//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0 object paths=./...
//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0 crd paths=./... output:crd:artifacts:config=../../config/crd/bases
package v1alpha1

const (
	// GroupName is the Kubernetes API group owned by this project.
	GroupName = "dsh.isolated.io"
	// Version is the first, intentionally unstable Cell API version.
	Version = "v1alpha1"
)
