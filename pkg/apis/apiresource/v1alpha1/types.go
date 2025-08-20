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

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// GroupVersionSpec represents an API group version for negotiation
type GroupVersionSpec struct {
	// Group is the API group name (e.g., "apps", "extensions")
	Group string `json:"group"`

	// Version is the API version (e.g., "v1", "v1beta1", "v1alpha1")
	Version string `json:"version"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Compatible",type="string",JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="APIVersion",type="string",JSONPath=`.spec.groupVersion`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// NegotiatedAPIResource represents an API resource that has been negotiated
// between a workspace and sync targets for compatibility. This enables dynamic
// API discovery and compatibility checking to determine what workloads can be
// placed where in a multi-cluster environment.
type NegotiatedAPIResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired negotiation state
	Spec NegotiatedAPIResourceSpec `json:"spec"`

	// Status defines the observed state of the negotiation
	// +optional
	Status NegotiatedAPIResourceStatus `json:"status,omitempty"`
}

// NegotiatedAPIResourceSpec defines the desired negotiation state for an API resource.
type NegotiatedAPIResourceSpec struct {
	// GroupVersion is the API group version to negotiate
	GroupVersion GroupVersionSpec `json:"groupVersion"`

	// Resources defines the list of resources to negotiate within this group version
	Resources []ResourceNegotiation `json:"resources"`

	// CommonSchema defines the shared schema that all sync targets must support.
	// This represents the intersection of capabilities across all targets.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	CommonSchema *runtime.RawExtension `json:"commonSchema,omitempty"`

	// Publish determines if this negotiated API should be published to sync targets
	// for consumption by workload placement decisions.
	// +optional
	Publish bool `json:"publish,omitempty"`
}

// ResourceNegotiation defines the negotiation parameters for a specific API resource.
type ResourceNegotiation struct {
	// Resource is the resource name (plural form, e.g., "deployments")
	Resource string `json:"resource"`

	// Kind is the resource kind (e.g., "Deployment")
	Kind string `json:"kind"`

	// Scope defines whether this is a cluster-scoped or namespaced resource
	// +kubebuilder:validation:Enum=Cluster;Namespaced
	Scope ResourceScope `json:"scope"`

	// Subresources lists the subresources that should be supported (e.g., "status", "scale")
	// +optional
	Subresources []string `json:"subresources,omitempty"`

	// RequiredFields specifies fields that must be supported by all sync targets
	// for compatibility. This allows validation of schema compatibility.
	// +optional
	RequiredFields []FieldRequirement `json:"requiredFields,omitempty"`
}

// ResourceScope defines the scope of a Kubernetes resource.
type ResourceScope string

const (
	// ClusterScoped indicates the resource is cluster-scoped
	ClusterScoped ResourceScope = "Cluster"
	// NamespacedScoped indicates the resource is namespace-scoped
	NamespacedScoped ResourceScope = "Namespaced"
)

// FieldRequirement specifies a required field for API compatibility.
type FieldRequirement struct {
	// Path is the JSONPath to the field (e.g., ".spec.replicas")
	Path string `json:"path"`

	// Type is the expected field type (e.g., "integer", "string", "object")
	Type string `json:"type"`

	// Required indicates whether this field must be present for compatibility
	Required bool `json:"required"`
}

// NegotiatedAPIResourceStatus defines the observed state of API negotiation.
type NegotiatedAPIResourceStatus struct {
	// Phase represents the current phase of the negotiation process
	// +kubebuilder:validation:Enum=Pending;Negotiating;Compatible;Incompatible
	Phase NegotiationPhase `json:"phase,omitempty"`

	// CompatibleLocations lists sync targets where this API is fully supported
	// +optional
	CompatibleLocations []CompatibleLocation `json:"compatibleLocations,omitempty"`

	// IncompatibleLocations lists sync targets where this API is not supported
	// along with the reasons for incompatibility
	// +optional
	IncompatibleLocations []IncompatibleLocation `json:"incompatibleLocations,omitempty"`

	// LastNegotiationTime records when the negotiation was last attempted
	// +optional
	LastNegotiationTime *metav1.Time `json:"lastNegotiationTime,omitempty"`

	// Conditions represent the current observed conditions of the negotiation
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

// NegotiationPhase represents the phase of API negotiation.
type NegotiationPhase string

const (
	// NegotiationPending indicates negotiation has not yet started
	NegotiationPending NegotiationPhase = "Pending"
	// NegotiationNegotiating indicates negotiation is in progress
	NegotiationNegotiating NegotiationPhase = "Negotiating"
	// NegotiationCompatible indicates the API is compatible across targets
	NegotiationCompatible NegotiationPhase = "Compatible"
	// NegotiationIncompatible indicates the API has compatibility issues
	NegotiationIncompatible NegotiationPhase = "Incompatible"
)

// CompatibleLocation represents a sync target that supports this API.
type CompatibleLocation struct {
	// Name of the sync target location
	Name string `json:"name"`

	// SupportedVersions lists the API versions supported at this location
	// +optional
	SupportedVersions []string `json:"supportedVersions,omitempty"`

	// Constraints lists any constraints or limitations at this location
	// +optional
	Constraints []LocationConstraint `json:"constraints,omitempty"`
}

// IncompatibleLocation represents a sync target that does not support this API.
type IncompatibleLocation struct {
	// Name of the sync target location
	Name string `json:"name"`

	// Reason provides a human-readable explanation for the incompatibility
	Reason string `json:"reason"`

	// MissingFields lists required fields that are not supported at this location
	// +optional
	MissingFields []string `json:"missingFields,omitempty"`
}

// LocationConstraint defines a constraint or limitation at a specific location.
type LocationConstraint struct {
	// Type categorizes the constraint (e.g., "version", "feature", "field")
	Type string `json:"type"`

	// Value provides the constraint details
	Value string `json:"value"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NegotiatedAPIResourceList contains a list of NegotiatedAPIResource objects.
type NegotiatedAPIResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NegotiatedAPIResource `json:"items"`
}
