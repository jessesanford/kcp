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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Location",type="string",JSONPath=`.spec.locationRef.name`
// +kubebuilder:printcolumn:name="APIs",type="integer",JSONPath=`.status.discoveredAPIs`
// +kubebuilder:printcolumn:name="Features",type="integer",JSONPath=`.status.discoveredFeatures`
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// APIDiscovery discovers APIs and capabilities at a location
type APIDiscovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APIDiscoverySpec   `json:"spec"`
	Status APIDiscoveryStatus `json:"status,omitempty"`
}

// APIDiscoverySpec defines discovery parameters
type APIDiscoverySpec struct {
	// LocationRef references the location to discover
	LocationRef LocationReference `json:"locationRef"`

	// DiscoveryPolicy controls discovery behavior
	// +optional
	DiscoveryPolicy *DiscoveryPolicy `json:"discoveryPolicy,omitempty"`

	// FeatureGates to probe
	// +optional
	FeatureGates []string `json:"featureGates,omitempty"`

	// RefreshInterval for discovery
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=duration
	RefreshInterval *metav1.Duration `json:"refreshInterval,omitempty"`

	// Paused stops discovery
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// LocationReference identifies a location
type LocationReference struct {
	// Name of the SyncTarget
	Name string `json:"name"`

	// Namespace if applicable
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// DiscoveryPolicy controls discovery behavior
type DiscoveryPolicy struct {
	// Scope of discovery
	// +kubebuilder:validation:Enum=Full;Partial;Minimal
	Scope DiscoveryScope `json:"scope"`

	// IncludeDeprecated APIs
	// +optional
	IncludeDeprecated bool `json:"includeDeprecated,omitempty"`

	// IncludeBeta APIs
	// +optional
	IncludeBeta bool `json:"includeBeta,omitempty"`

	// IncludeAlpha APIs
	// +optional
	IncludeAlpha bool `json:"includeAlpha,omitempty"`

	// ResourceFilters to apply
	// +optional
	ResourceFilters []ResourceFilter `json:"resourceFilters,omitempty"`
}

type DiscoveryScope string

const (
	DiscoveryScopeFull    DiscoveryScope = "Full"
	DiscoveryScopePartial DiscoveryScope = "Partial"
	DiscoveryScopeMinimal DiscoveryScope = "Minimal"
)

// ResourceFilter filters discovered resources
type ResourceFilter struct {
	// Group to filter
	// +optional
	Group string `json:"group,omitempty"`

	// Version to filter
	// +optional
	Version string `json:"version,omitempty"`

	// Kind to filter
	// +optional
	Kind string `json:"kind,omitempty"`

	// Action to take
	// +kubebuilder:validation:Enum=Include;Exclude
	Action FilterAction `json:"action"`
}

type FilterAction string

const (
	FilterActionInclude FilterAction = "Include"
	FilterActionExclude FilterAction = "Exclude"
)

// APIDiscoveryStatus contains discovery results
type APIDiscoveryStatus struct {
	// Phase of discovery
	// +kubebuilder:validation:Enum=Pending;Discovering;Discovered;Failed
	Phase DiscoveryPhase `json:"phase,omitempty"`

	// DiscoveredAPIs count
	DiscoveredAPIs int32 `json:"discoveredAPIs"`

	// DiscoveredFeatures count
	DiscoveredFeatures int32 `json:"discoveredFeatures"`

	// APIGroups discovered
	// +optional
	APIGroups []DiscoveredAPIGroup `json:"apiGroups,omitempty"`

	// Features discovered
	// +optional
	Features []DiscoveredFeature `json:"features,omitempty"`

	// ServerVersion discovered
	// +optional
	ServerVersion *version.Info `json:"serverVersion,omitempty"`

	// LastDiscoveryTime
	// +optional
	LastDiscoveryTime *metav1.Time `json:"lastDiscoveryTime,omitempty"`

	// ObservedGeneration
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions of discovery
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

type DiscoveryPhase string

const (
	DiscoveryPhasePending     DiscoveryPhase = "Pending"
	DiscoveryPhaseDiscovering DiscoveryPhase = "Discovering"
	DiscoveryPhaseDiscovered  DiscoveryPhase = "Discovered"
	DiscoveryPhaseFailed      DiscoveryPhase = "Failed"
)

// DiscoveredAPIGroup represents a discovered API group
type DiscoveredAPIGroup struct {
	// Name of the group
	Name string `json:"name"`

	// Versions available
	Versions []DiscoveredVersion `json:"versions"`

	// PreferredVersion
	// +optional
	PreferredVersion string `json:"preferredVersion,omitempty"`
}

// DiscoveredVersion represents a discovered version
type DiscoveredVersion struct {
	// Version string
	Version string `json:"version"`

	// Resources in this version
	Resources []DiscoveredResource `json:"resources"`

	// Deprecated status
	// +optional
	Deprecated bool `json:"deprecated,omitempty"`
}

// DiscoveredResource represents a discovered resource
type DiscoveredResource struct {
	// Kind of the resource
	Kind string `json:"kind"`

	// Name (plural) of the resource
	Name string `json:"name"`

	// Namespaced or cluster-scoped
	Namespaced bool `json:"namespaced"`

	// Verbs supported
	Verbs []string `json:"verbs"`

	// ShortNames available
	// +optional
	ShortNames []string `json:"shortNames,omitempty"`

	// Categories this resource belongs to
	// +optional
	Categories []string `json:"categories,omitempty"`

	// StorageVersion if applicable
	// +optional
	StorageVersion string `json:"storageVersion,omitempty"`
}

// DiscoveredFeature represents a discovered feature/capability
type DiscoveredFeature struct {
	// Name of the feature
	Name string `json:"name"`

	// Enabled status
	Enabled bool `json:"enabled"`

	// Version of the feature
	// +optional
	Version string `json:"version,omitempty"`

	// Configuration if applicable
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Configuration *runtime.RawExtension `json:"configuration,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// APIDiscoveryList contains a list of APIDiscoveries
type APIDiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIDiscovery `json:"items"`
}
