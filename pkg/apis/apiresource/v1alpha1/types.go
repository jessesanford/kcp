/*
Copyright The KCP Authors.

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

	conditionsv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Location",type="string",JSONPath=`.spec.locationRef.name`
// +kubebuilder:printcolumn:name="API",type="string",JSONPath=`.spec.api`
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// NegotiatedAPIResource represents a negotiated API resource capability
type NegotiatedAPIResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NegotiatedAPIResourceSpec   `json:"spec"`
	Status NegotiatedAPIResourceStatus `json:"status,omitempty"`
}

// NegotiatedAPIResourceSpec defines the negotiation requirements
type NegotiatedAPIResourceSpec struct {
	// LocationRef references the target location
	LocationRef LocationReference `json:"locationRef"`

	// API specifies the API resource to negotiate
	API APIResourceRef `json:"api"`

	// Requirements for the API resource
	// +optional
	Requirements *APIRequirements `json:"requirements,omitempty"`
}

// APIResourceRef identifies an API resource
type APIResourceRef struct {
	// Group of the API
	// +optional
	Group string `json:"group,omitempty"`

	// Version of the API
	Version string `json:"version"`

	// Kind of the resource
	Kind string `json:"kind"`

	// Resource name (plural)
	Resource string `json:"resource"`
}

// APIRequirements defines requirements for API compatibility
type APIRequirements struct {
	// MinVersion required
	// +optional
	MinVersion string `json:"minVersion,omitempty"`

	// MaxVersion supported
	// +optional
	MaxVersion string `json:"maxVersion,omitempty"`

	// RequiredVerbs that must be supported
	// +optional
	RequiredVerbs []string `json:"requiredVerbs,omitempty"`

	// RequiredFields that must be present
	// +optional
	RequiredFields []string `json:"requiredFields,omitempty"`
}

// NegotiatedAPIResourceStatus represents the negotiation status
type NegotiatedAPIResourceStatus struct {
	// Phase of negotiation
	// +kubebuilder:validation:Enum=Pending;Negotiating;Compatible;Incompatible;Failed
	Phase NegotiationPhase `json:"phase,omitempty"`

	// NegotiatedVersion that was agreed upon
	// +optional
	NegotiatedVersion string `json:"negotiatedVersion,omitempty"`

	// SupportedVerbs at the target
	// +optional
	SupportedVerbs []string `json:"supportedVerbs,omitempty"`

	// CompatibilityScore (0-100)
	// +optional
	CompatibilityScore *int32 `json:"compatibilityScore,omitempty"`

	// ObservedGeneration
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions of negotiation
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

type NegotiationPhase string

const (
	NegotiationPhasePending      NegotiationPhase = "Pending"
	NegotiationPhaseNegotiating  NegotiationPhase = "Negotiating"
	NegotiationPhaseCompatible   NegotiationPhase = "Compatible"
	NegotiationPhaseIncompatible NegotiationPhase = "Incompatible"
	NegotiationPhaseFailed       NegotiationPhase = "Failed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NegotiatedAPIResourceList contains a list of NegotiatedAPIResources
type NegotiatedAPIResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NegotiatedAPIResource `json:"items"`
}
