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

var (
	SchemeGroupVersion = schema.GroupVersion{Group: "tmc.kcp.io", Version: "v1alpha1"}

	// SchemeBuilder builds a scheme with the types known to this package.
	SchemeBuilder runtime.SchemeBuilder
	localSchemeBuilder = &SchemeBuilder
	// AddToScheme applies all stored functions to the scheme. A non-nil error
	// indicates that one function failed.
	AddToScheme = localSchemeBuilder.AddToScheme
)

func init() {
	// We only register the types via the local scheme builder.
	localSchemeBuilder.Register(addKnownTypes)
}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// addKnownTypes adds our types to the API scheme by registering
// ClusterRegistration and WorkloadPlacement and their lists.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ClusterRegistration{},
		&ClusterRegistrationList{},
		&WorkloadPlacement{},
		&WorkloadPlacementList{},
	)
	// AddToGroupVersion allows the serialization of client types like ListOptions.
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}