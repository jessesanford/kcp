package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion is the group version for virtual workspace APIs
var GroupVersion = schema.GroupVersion{Group: "virtual.kcp.io", Version: "v1alpha1"}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return GroupVersion.WithResource(resource).GroupResource()
}

var (
	// SchemeBuilder builds the scheme
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds types to the scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// addKnownTypes adds the list of known types to the given scheme
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&APIResource{},
		&APIResourceList{},
		&VirtualWorkspace{},
		&VirtualWorkspaceList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}