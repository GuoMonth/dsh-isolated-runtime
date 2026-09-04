package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion identifies the Cell API served by this package.
	GroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}
	// SchemeBuilder registers Cell API objects with a Kubernetes scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme adds all Cell API objects to a Kubernetes scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, &Cell{}, &CellList{}, &CellSnapshot{}, &CellSnapshotList{})
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
