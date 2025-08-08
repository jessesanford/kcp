/*
Copyright 2025 The KCP Authors.

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

// GroupName specifies the group name used to register the objects.
const GroupName = "workload.kcp.io"

// GroupVersion specifies the group and the version used to register the objects.
var GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

// SchemeGroupVersion is group version used to register these objects.
var SchemeGroupVersion = GroupVersion

// Resource takes an unqualified resource and returns a Group qualified GroupResource.
func Resource(resource string) schema.GroupResource {
	return GroupVersion.WithResource(resource).GroupResource()
}

var (
	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme is a global function that registers this API group & version to a scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// addKnownTypes adds our types to the API scheme by registering WorkloadPlacement and its list counterpart.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Placement{},
		&PlacementList{},
		&Location{},
		&LocationList{},
		&ResourceExport{},
		&ResourceExportList{},
		&ResourceImport{},
		&ResourceImportList{},
		&SyncTarget{},
		&SyncTargetList{},
		&SyncTargetHeartbeat{},
		&SyncTargetHeartbeatList{},
	)

	// register common meta types into schemas.
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}