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

// GroupName is the group name for TMC API objects
const GroupName = "tmc.kcp.io"

var (
	// SchemeGroupVersion is the group version used to register TMC API objects
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

	// SchemeBuilder is the scheme builder for TMC API objects
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	
	// AddToScheme applies the TMC scheme to the specified scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// addKnownTypes registers the TMC API objects with the scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		SchemeGroupVersion,
		// Extended status types are embedded in other API types and do not need to be registered directly
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}