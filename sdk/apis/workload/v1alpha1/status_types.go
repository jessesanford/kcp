package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Workload",type="string",JSONPath=`.spec.workloadRef.name`
// +kubebuilder:printcolumn:name="Healthy",type="integer",JSONPath=`.status.healthyLocations`
// +kubebuilder:printcolumn:name="Total",type="integer",JSONPath=`.status.totalLocations`
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=`.status.aggregatedPhase`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// WorkloadStatusAggregation aggregates status from multiple locations
type WorkloadStatusAggregation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadStatusAggregationSpec   `json:"spec"`
	Status WorkloadStatusAggregationStatus `json:"status,omitempty"`
}

// WorkloadStatusAggregationSpec defines what to aggregate
type WorkloadStatusAggregationSpec struct {
	// WorkloadRef identifies the workload
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// StatusFields to aggregate
	StatusFields []StatusFieldSelector `json:"statusFields"`

	// AggregationPolicy defines how to aggregate
	// +optional
	AggregationPolicy *AggregationPolicy `json:"aggregationPolicy,omitempty"`

	// HealthPolicy defines health criteria
	// +optional
	HealthPolicy *HealthPolicy `json:"healthPolicy,omitempty"`

	// UpdateFrequency for aggregation
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=duration
	UpdateFrequency *metav1.Duration `json:"updateFrequency,omitempty"`
}

// WorkloadReference identifies a workload resource
type WorkloadReference struct {
	// APIVersion of the workload
	APIVersion string `json:"apiVersion"`

	// Kind of the workload
	Kind string `json:"kind"`

	// Name of the workload
	Name string `json:"name"`

	// Namespace of the workload (if namespaced)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// StatusFieldSelector selects fields to aggregate
type StatusFieldSelector struct {
	// Path to the status field (JSONPath)
	Path string `json:"path"`

	// DisplayName for this field
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// AggregationType for this field
	// +kubebuilder:validation:Enum=Sum;Average;Min;Max;Latest;All;Count
	AggregationType AggregationType `json:"aggregationType"`

	// Priority for conflict resolution
	// +optional
	Priority int32 `json:"priority,omitempty"`
}

type AggregationType string

const (
	AggregationTypeSum     AggregationType = "Sum"
	AggregationTypeAverage AggregationType = "Average"
	AggregationTypeMin     AggregationType = "Min"
	AggregationTypeMax     AggregationType = "Max"
	AggregationTypeLatest  AggregationType = "Latest"
	AggregationTypeAll     AggregationType = "All"
	AggregationTypeCount   AggregationType = "Count"
)

// AggregationPolicy defines aggregation behavior
type AggregationPolicy struct {
	// Strategy for aggregation
	// +kubebuilder:validation:Enum=Optimistic;Pessimistic;Majority
	Strategy AggregationStrategy `json:"strategy"`

	// MinLocations required for valid aggregation
	// +optional
	// +kubebuilder:validation:Minimum=1
	MinLocations int32 `json:"minLocations,omitempty"`

	// RequireAllLocations for aggregation
	// +optional
	RequireAllLocations bool `json:"requireAllLocations,omitempty"`
}

type AggregationStrategy string

const (
	// Optimistic - assume success if any location succeeds
	AggregationStrategyOptimistic AggregationStrategy = "Optimistic"

	// Pessimistic - assume failure if any location fails
	AggregationStrategyPessimistic AggregationStrategy = "Pessimistic"

	// Majority - based on majority of locations
	AggregationStrategyMajority AggregationStrategy = "Majority"
)

// HealthPolicy defines health criteria
type HealthPolicy struct {
	// HealthyConditions required for health
	HealthyConditions []ConditionRequirement `json:"healthyConditions"`

	// UnhealthyConditions that indicate problems
	// +optional
	UnhealthyConditions []ConditionRequirement `json:"unhealthyConditions,omitempty"`

	// MinHealthyReplicas required
	// +optional
	MinHealthyReplicas *intstr.IntOrString `json:"minHealthyReplicas,omitempty"`
}

// ConditionRequirement defines a required condition
type ConditionRequirement struct {
	// Type of condition
	Type string `json:"type"`

	// Status required
	// +kubebuilder:validation:Enum=True;False;Unknown
	Status metav1.ConditionStatus `json:"status"`

	// Reason patterns to match
	// +optional
	Reasons []string `json:"reasons,omitempty"`
}

// WorkloadStatusAggregationStatus contains aggregated status
type WorkloadStatusAggregationStatus struct {
	// AggregatedPhase overall phase
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Unknown
	AggregatedPhase WorkloadPhase `json:"aggregatedPhase,omitempty"`

	// TotalLocations reporting status
	TotalLocations int32 `json:"totalLocations"`

	// HealthyLocations count
	HealthyLocations int32 `json:"healthyLocations"`

	// UnhealthyLocations count
	UnhealthyLocations int32 `json:"unhealthyLocations"`

	// LocationStatuses per location
	// +optional
	LocationStatuses []AggregatedLocationStatus `json:"locationStatuses,omitempty"`

	// AggregatedFields with values
	// +optional
	AggregatedFields []AggregatedField `json:"aggregatedFields,omitempty"`

	// LastAggregationTime
	// +optional
	LastAggregationTime *metav1.Time `json:"lastAggregationTime,omitempty"`

	// Conditions of aggregation
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

type WorkloadPhase string

const (
	WorkloadPhasePending   WorkloadPhase = "Pending"
	WorkloadPhaseRunning   WorkloadPhase = "Running"
	WorkloadPhaseSucceeded WorkloadPhase = "Succeeded"
	WorkloadPhaseFailed    WorkloadPhase = "Failed"
	WorkloadPhaseUnknown   WorkloadPhase = "Unknown"
)

// AggregatedLocationStatus for a specific location
type AggregatedLocationStatus struct {
	// LocationName
	LocationName string `json:"locationName"`

	// Phase at this location
	Phase WorkloadPhase `json:"phase"`

	// Healthy status
	Healthy bool `json:"healthy"`

	// LastUpdateTime from this location
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// ExtractedFields from this location
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	ExtractedFields runtime.RawExtension `json:"extractedFields,omitempty"`
}

// AggregatedField contains aggregated value
type AggregatedField struct {
	// Name of the field
	Name string `json:"name"`

	// DisplayName if provided
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Value after aggregation
	// +kubebuilder:pruning:PreserveUnknownFields
	Value runtime.RawExtension `json:"value"`

	// Sources contributing to this value
	// +optional
	Sources []string `json:"sources,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadStatusAggregationList contains a list
type WorkloadStatusAggregationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadStatusAggregation `json:"items"`
}
