package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Workload",type="string",JSONPath=`.spec.workloadRef.name`
// +kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=`.spec.totalReplicas`
// +kubebuilder:printcolumn:name="Current",type="integer",JSONPath=`.status.currentReplicas`
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// WorkloadDistribution manages distribution of a workload across locations.
// It orchestrates how workloads are replicated and deployed across multiple SyncTargets
// based on placement policies, providing advanced rollout strategies and per-location
// customization capabilities.
type WorkloadDistribution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadDistributionSpec   `json:"spec"`
	Status WorkloadDistributionStatus `json:"status,omitempty"`
}

// WorkloadDistributionSpec defines how to distribute a workload across locations
type WorkloadDistributionSpec struct {
	// WorkloadRef references the workload to distribute
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// TotalReplicas desired across all locations
	// +kubebuilder:validation:Minimum=0
	TotalReplicas int32 `json:"totalReplicas"`

	// PlacementPolicyRef references the placement policy to use for location selection
	// If not specified, manual distribution via Distributions field is expected
	// +optional
	PlacementPolicyRef *ObjectReference `json:"placementPolicyRef,omitempty"`

	// Distributions defines explicit per-location distribution overriding placement policy
	// When specified, this takes precedence over placement policy decisions
	// +optional
	Distributions []LocationDistribution `json:"distributions,omitempty"`

	// RolloutStrategy defines how updates are rolled out across locations
	// +optional
	RolloutStrategy *RolloutStrategy `json:"rolloutStrategy,omitempty"`

	// ResourceOverrides allows customizing resource requirements per location
	// +optional
	ResourceOverrides []ResourceOverride `json:"resourceOverrides,omitempty"`

	// Paused stops reconciliation when true, allowing manual intervention
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// WorkloadReference identifies a workload resource to be distributed
type WorkloadReference struct {
	// APIVersion of the workload (e.g., "apps/v1")
	APIVersion string `json:"apiVersion"`

	// Kind of the workload (e.g., "Deployment", "StatefulSet")
	Kind string `json:"kind"`

	// Name of the workload resource
	Name string `json:"name"`

	// Namespace of the workload (if namespaced resource)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ObjectReference is a reference to another Kubernetes object
type ObjectReference struct {
	// Name of the referenced object
	Name string `json:"name"`

	// Namespace of the referenced object (if namespaced)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// LocationDistribution specifies replica distribution for a specific location
type LocationDistribution struct {
	// LocationName identifies the SyncTarget location
	LocationName string `json:"locationName"`

	// Replicas specifies the desired number of replicas at this location
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`

	// Priority for this location during rollouts (lower values have higher priority)
	// Used to determine rollout order when updating workloads
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Priority *int32 `json:"priority,omitempty"`
}

// RolloutStrategy defines how updates are rolled out across locations
type RolloutStrategy struct {
	// Type of rollout strategy to use
	// +kubebuilder:validation:Enum=RollingUpdate;Recreate;BlueGreen
	Type RolloutType `json:"type"`

	// RollingUpdate configuration for rolling update strategy
	// +optional
	RollingUpdate *RollingUpdateStrategy `json:"rollingUpdate,omitempty"`

	// BlueGreen configuration for blue-green deployment strategy
	// +optional
	BlueGreen *BlueGreenStrategy `json:"blueGreen,omitempty"`
}

// RolloutType represents the type of rollout strategy
type RolloutType string

const (
	// RolloutTypeRollingUpdate performs rolling updates across locations
	RolloutTypeRollingUpdate RolloutType = "RollingUpdate"
	// RolloutTypeRecreate terminates all replicas before creating new ones
	RolloutTypeRecreate RolloutType = "Recreate"
	// RolloutTypeBlueGreen maintains two environments and switches between them
	RolloutTypeBlueGreen RolloutType = "BlueGreen"
)

// RollingUpdateStrategy configures rolling update behavior
type RollingUpdateStrategy struct {
	// MaxUnavailable specifies maximum number of locations that can be unavailable
	// during the update. Can be an absolute number or percentage.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// MaxSurge specifies maximum number of locations that can be created above
	// the desired number during update. Can be an absolute number or percentage.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`

	// Partition indicates the ordinal at which the rollout should be partitioned
	// for canary deployments. Locations with ordinal >= partition won't be updated.
	// +optional
	Partition *int32 `json:"partition,omitempty"`
}

// BlueGreenStrategy configures blue-green deployment behavior
type BlueGreenStrategy struct {
	// ActiveService identifies the service selector for the active environment
	ActiveService string `json:"activeService"`

	// PreviewService identifies the service selector for the preview environment
	// +optional
	PreviewService string `json:"previewService,omitempty"`

	// AutoPromotionEnabled automatically promotes preview to active after validation
	// +optional
	AutoPromotionEnabled bool `json:"autoPromotionEnabled,omitempty"`

	// ScaleDownDelaySeconds specifies delay before scaling down the old environment
	// after promotion
	// +optional
	ScaleDownDelaySeconds *int32 `json:"scaleDownDelaySeconds,omitempty"`
}

// ResourceOverride allows overriding resource requirements for specific locations
type ResourceOverride struct {
	// LocationName specifies the location to apply this override
	LocationName string `json:"locationName"`

	// ResourceRequirements specifies the resource requirements override
	ResourceRequirements ResourceRequirements `json:"resourceRequirements"`
}

// ResourceRequirements describes compute resource requirements
type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed
	// +optional
	Limits map[string]string `json:"limits,omitempty"`

	// Requests describes the minimum amount of compute resources required
	// +optional
	Requests map[string]string `json:"requests,omitempty"`
}

// WorkloadDistributionStatus defines the observed state of WorkloadDistribution
type WorkloadDistributionStatus struct {
	// Phase represents the current phase of the distribution lifecycle
	// +kubebuilder:validation:Enum=Pending;Distributing;Distributed;Failed;Paused
	Phase DistributionPhase `json:"phase,omitempty"`

	// CurrentReplicas is the total number of currently running replicas across all locations
	CurrentReplicas int32 `json:"currentReplicas"`

	// ReadyReplicas is the total number of ready replicas across all locations
	ReadyReplicas int32 `json:"readyReplicas"`

	// UpdatedReplicas is the total number of replicas that have the updated spec
	UpdatedReplicas int32 `json:"updatedReplicas"`

	// LocationStatuses contains per-location status information
	// +optional
	LocationStatuses []LocationStatus `json:"locationStatuses,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastDistributionTime records when the distribution was last updated
	// +optional
	LastDistributionTime *metav1.Time `json:"lastDistributionTime,omitempty"`

	// Conditions represent the current service state of the WorkloadDistribution
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

// DistributionPhase represents the lifecycle phase of a WorkloadDistribution
type DistributionPhase string

const (
	// DistributionPhasePending indicates the distribution is waiting to be processed
	DistributionPhasePending DistributionPhase = "Pending"
	// DistributionPhaseDistributing indicates the distribution is actively being rolled out
	DistributionPhaseDistributing DistributionPhase = "Distributing"
	// DistributionPhaseDistributed indicates all replicas have been successfully distributed
	DistributionPhaseDistributed DistributionPhase = "Distributed"
	// DistributionPhaseFailed indicates the distribution has failed
	DistributionPhaseFailed DistributionPhase = "Failed"
	// DistributionPhasePaused indicates the distribution is paused
	DistributionPhasePaused DistributionPhase = "Paused"
)

// LocationStatus tracks the status of workload distribution at a specific location
type LocationStatus struct {
	// LocationName identifies the location this status refers to
	LocationName string `json:"locationName"`

	// AllocatedReplicas is the number of replicas allocated to this location
	AllocatedReplicas int32 `json:"allocatedReplicas"`

	// CurrentReplicas is the actual number of replicas running at this location
	CurrentReplicas int32 `json:"currentReplicas"`

	// ReadyReplicas is the number of ready replicas at this location
	ReadyReplicas int32 `json:"readyReplicas"`

	// SyncedGeneration represents the generation of the spec that was last synced
	// +optional
	SyncedGeneration int64 `json:"syncedGeneration,omitempty"`

	// LastSyncTime records when this location was last synchronized
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Message provides additional information about the status of this location
	// +optional
	Message string `json:"message,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadDistributionList contains a list of WorkloadDistributions
type WorkloadDistributionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadDistribution `json:"items"`
}
