/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// Version defines the API version
	Version = "v1alpha1"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// ResourceWithVersion takes an unqualified resource and returns a Group qualified GroupVersionResource
func ResourceWithVersion(resource string) schema.GroupVersionResource {
	return SchemeGroupVersion.WithResource(resource)
}

var (
	// SchemeBuilder is the scheme builder for TMC APIs
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme is a common registration function for mapping packaged scoped group & version keys to a scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// addKnownTypes adds the set of types defined in this package to the supplied scheme.
// NOTE: This is a skeleton implementation that registers no types yet.
// Actual type registration will happen in subsequent PRs that define the concrete types.
func addKnownTypes(scheme *runtime.Scheme) error {
	// TODO: Register concrete types here in future PRs:
	// scheme.AddKnownTypes(SchemeGroupVersion,
	//     &ClusterRegistration{},
	//     &ClusterRegistrationList{},
	//     &WorkloadPlacement{},
	//     &WorkloadPlacementList{},
	// )
	
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

// Known resource names - these will be used when concrete types are implemented
const (
	ClusterRegistrationResource = "clusterregistrations"
	WorkloadPlacementResource  = "workloadplacements"
)

// GetClusterRegistrationGVR returns the GroupVersionResource for ClusterRegistration
func GetClusterRegistrationGVR() schema.GroupVersionResource {
	return ResourceWithVersion(ClusterRegistrationResource)
}

// GetWorkloadPlacementGVR returns the GroupVersionResource for WorkloadPlacement
func GetWorkloadPlacementGVR() schema.GroupVersionResource {
	return ResourceWithVersion(WorkloadPlacementResource)
}

// GetClusterRegistrationGVK returns the GroupVersionKind for ClusterRegistration
func GetClusterRegistrationGVK() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("ClusterRegistration")
}

// GetWorkloadPlacementGVK returns the GroupVersionKind for WorkloadPlacement
func GetWorkloadPlacementGVK() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("WorkloadPlacement")
}