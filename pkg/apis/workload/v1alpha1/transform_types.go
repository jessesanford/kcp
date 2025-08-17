package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=`.spec.targetRef.kind`
// +kubebuilder:printcolumn:name="Locations",type="integer",JSONPath=`.status.appliedLocations`
// +kubebuilder:printcolumn:name="Active",type="string",JSONPath=`.status.conditions[?(@.type=="Active")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// WorkloadTransform defines transformations to apply to workloads
type WorkloadTransform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadTransformSpec   `json:"spec"`
	Status WorkloadTransformStatus `json:"status,omitempty"`
}

// WorkloadTransformSpec defines transformation rules
type WorkloadTransformSpec struct {
	// TargetRef identifies what to transform
	TargetRef TransformTarget `json:"targetRef"`

	// LocationSelectors where to apply transforms
	// +optional
	LocationSelectors []LocationSelector `json:"locationSelectors,omitempty"`

	// Transforms to apply
	Transforms []Transform `json:"transforms"`

	// Priority when multiple transforms match
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	Priority int32 `json:"priority,omitempty"`

	// Paused stops applying transforms
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// TransformTarget identifies what to transform
type TransformTarget struct {
	// APIVersion of the target
	APIVersion string `json:"apiVersion"`

	// Kind of the target
	Kind string `json:"kind"`

	// Name of specific object
	// +optional
	Name string `json:"name,omitempty"`

	// LabelSelector for multiple objects
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// LocationSelector selects locations for transformation
type LocationSelector struct {
	// Name of specific location
	// +optional
	Name string `json:"name,omitempty"`

	// LabelSelector for locations
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// Transform defines a transformation operation
type Transform struct {
	// Type of transformation
	// +kubebuilder:validation:Enum=JSONPatch;StrategicMerge;Replace;Remove;Annotate;Label
	Type TransformType `json:"type"`

	// JSONPatch operations
	// +optional
	JSONPatch []JSONPatchOperation `json:"jsonPatch,omitempty"`

	// StrategicMerge patch
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	StrategicMerge *runtime.RawExtension `json:"strategicMerge,omitempty"`

	// Replace operation
	// +optional
	Replace *ReplaceOperation `json:"replace,omitempty"`

	// Remove operation
	// +optional
	Remove *RemoveOperation `json:"remove,omitempty"`

	// Annotations to add/update
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels to add/update
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

type TransformType string

const (
	TransformTypeJSONPatch      TransformType = "JSONPatch"
	TransformTypeStrategicMerge TransformType = "StrategicMerge"
	TransformTypeReplace        TransformType = "Replace"
	TransformTypeRemove         TransformType = "Remove"
	TransformTypeAnnotate       TransformType = "Annotate"
	TransformTypeLabel          TransformType = "Label"
)

// JSONPatchOperation defines a JSON patch operation
type JSONPatchOperation struct {
	// Op is the operation type
	// +kubebuilder:validation:Enum=add;remove;replace;copy;move;test
	Op string `json:"op"`

	// Path is the JSON path
	Path string `json:"path"`

	// Value for the operation
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Value *runtime.RawExtension `json:"value,omitempty"`

	// From path for copy/move
	// +optional
	From string `json:"from,omitempty"`
}

// ReplaceOperation replaces field values
type ReplaceOperation struct {
	// Path to replace
	Path string `json:"path"`

	// Value to set
	// +kubebuilder:pruning:PreserveUnknownFields
	Value runtime.RawExtension `json:"value"`
}

// RemoveOperation removes fields
type RemoveOperation struct {
	// Paths to remove
	Paths []string `json:"paths"`
}

// WorkloadTransformStatus defines the observed state
type WorkloadTransformStatus struct {
	// AppliedLocations count
	AppliedLocations int32 `json:"appliedLocations"`

	// TransformApplications tracks where applied
	// +optional
	TransformApplications []TransformApplication `json:"transformApplications,omitempty"`

	// ObservedGeneration last reconciled
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastAppliedTime
	// +optional
	LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`

	// Conditions of the transform
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

// TransformApplication tracks where transform is applied
type TransformApplication struct {
	// LocationName
	LocationName string `json:"locationName"`

	// AppliedAt timestamp
	AppliedAt metav1.Time `json:"appliedAt"`

	// ObjectsTransformed count
	ObjectsTransformed int32 `json:"objectsTransformed"`

	// Success status
	Success bool `json:"success"`

	// Message if failed
	// +optional
	Message string `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadTransformList contains a list of WorkloadTransforms
type WorkloadTransformList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadTransform `json:"items"`
}