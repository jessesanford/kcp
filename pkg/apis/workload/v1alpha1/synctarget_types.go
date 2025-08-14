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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// These are valid conditions of SyncTarget.
const (
	// SyncTargetReady indicates that the SyncTarget is ready to accept workloads.
	SyncTargetReady conditionsv1alpha1.ConditionType = "Ready"

	// SyncTargetSyncerReady indicates that the syncer component is connected and operational.
	SyncTargetSyncerReady conditionsv1alpha1.ConditionType = "SyncerReady"

	// SyncTargetClusterReady indicates that the target cluster is reachable and healthy.
	SyncTargetClusterReady conditionsv1alpha1.ConditionType = "ClusterReady"

	// Common condition reasons
	SyncerDisconnectedReason     = "SyncerDisconnected"
	ClusterUnreachableReason     = "ClusterUnreachable" 
	InvalidConfigurationReason   = "InvalidConfiguration"
	ValidationFailedReason       = "ValidationFailed"
	QuotaExceededReason         = "QuotaExceeded"
)

// SyncTarget represents a physical cluster that can receive workloads through
// the TMC syncer infrastructure. It defines the target cluster configuration,
// capacity limits, and workload selection criteria.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:printcolumn:name="Location",type="string",JSONPath=".spec.location"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Syncer",type="string",JSONPath=`.status.conditions[?(@.type=="SyncerReady")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the SyncTarget.
	Spec SyncTargetSpec `json:"spec,omitempty"`

	// Status communicates the observed state of the SyncTarget.
	Status SyncTargetStatus `json:"status,omitempty"`
}

// SyncTargetSpec defines the desired state of a SyncTarget.
type SyncTargetSpec struct {
	// clusterRef references the ClusterRegistration that this SyncTarget
	// is associated with. This establishes the connection to the physical cluster.
	//
	// +required
	ClusterRef ClusterReference `json:"clusterRef"`

	// location is a human-readable description of where this cluster is located
	// (e.g., "us-west-2", "datacenter-1", "edge-site-a").
	//
	// +optional
	Location string `json:"location,omitempty"`

	// syncerConfig contains configuration for the syncer component.
	//
	// +optional
	SyncerConfig *SyncerConfig `json:"syncerConfig,omitempty"`

	// resourceQuotas define capacity limits for this sync target.
	// If not specified, the target has unlimited capacity.
	//
	// +optional
	ResourceQuotas *ResourceQuotas `json:"resourceQuotas,omitempty"`

	// selector defines criteria for selecting workloads to sync to this target.
	// If not specified, all workloads are eligible for this target.
	//
	// +optional
	Selector *WorkloadSelector `json:"selector,omitempty"`

	// supportedResourceTypes lists the Kubernetes resource types that this
	// sync target can handle. If empty, all resource types are supported.
	//
	// +optional
	// +listType=set
	SupportedResourceTypes []string `json:"supportedResourceTypes,omitempty"`
}

// ClusterReference identifies a ClusterRegistration resource.
type ClusterReference struct {
	// name is the name of the ClusterRegistration resource.
	//
	// +required
	Name string `json:"name"`

	// workspace is the logical cluster workspace where the ClusterRegistration exists.
	// If empty, defaults to the same workspace as the SyncTarget.
	//
	// +optional
	Workspace string `json:"workspace,omitempty"`
}

// SyncerConfig contains configuration parameters for the syncer component.
type SyncerConfig struct {
	// syncMode defines how the syncer should operate.
	// Valid values are "push" (default), "pull", or "bidirectional".
	//
	// +optional
	// +kubebuilder:default="push"
	// +kubebuilder:validation:Enum=push;pull;bidirectional
	SyncMode string `json:"syncMode,omitempty"`

	// syncInterval specifies how often the syncer should reconcile state.
	// Defaults to "30s".
	//
	// +optional
	// +kubebuilder:default="30s"
	SyncInterval string `json:"syncInterval,omitempty"`

	// retryBackoff defines retry behavior for failed sync operations.
	//
	// +optional
	RetryBackoff *RetryBackoffConfig `json:"retryBackoff,omitempty"`
}

// RetryBackoffConfig defines retry backoff parameters.
type RetryBackoffConfig struct {
	// initialInterval is the initial retry interval.
	//
	// +optional
	// +kubebuilder:default="1s"
	InitialInterval string `json:"initialInterval,omitempty"`

	// maxInterval is the maximum retry interval.
	//
	// +optional
	// +kubebuilder:default="5m"
	MaxInterval string `json:"maxInterval,omitempty"`

	// multiplier is the backoff multiplier.
	//
	// +optional
	// +kubebuilder:default=2.0
	Multiplier float64 `json:"multiplier,omitempty"`
}

// ResourceQuotas defines capacity limits for a SyncTarget.
type ResourceQuotas struct {
	// cpu is the maximum CPU capacity that can be allocated to workloads.
	//
	// +optional
	CPU *resource.Quantity `json:"cpu,omitempty"`

	// memory is the maximum memory capacity that can be allocated to workloads.
	//
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty"`

	// storage is the maximum storage capacity that can be allocated to workloads.
	//
	// +optional
	Storage *resource.Quantity `json:"storage,omitempty"`

	// pods is the maximum number of pods that can be scheduled on this target.
	//
	// +optional
	Pods *resource.Quantity `json:"pods,omitempty"`

	// custom allows for arbitrary resource quota definitions.
	//
	// +optional
	Custom map[string]resource.Quantity `json:"custom,omitempty"`
}

// WorkloadSelector defines criteria for selecting workloads.
type WorkloadSelector struct {
	// matchLabels is a map of {key,value} pairs. A single {key,value} in the
	// matchLabels map is equivalent to an element of matchExpressions with
	// operator "In", values containing only value.
	//
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// matchExpressions is a list of label selector requirements.
	//
	// +optional
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`

	// namespaceSelector selects workloads based on their namespace labels.
	//
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// locations is a list of location names that workloads must match.
	// If specified, only workloads with matching location annotations will be selected.
	//
	// +optional
	// +listType=set
	Locations []string `json:"locations,omitempty"`
}

// SyncTargetStatus defines the observed state of a SyncTarget.
type SyncTargetStatus struct {
	// conditions is a list of conditions that apply to the SyncTarget.
	//
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// capacity reports the current resource capacity available on this sync target.
	//
	// +optional
	Capacity ResourceCapacity `json:"capacity,omitempty"`

	// allocatable reports the resources that are available for workload allocation
	// after accounting for system overhead.
	//
	// +optional
	Allocatable ResourceCapacity `json:"allocatable,omitempty"`

	// allocated reports the resources currently allocated to workloads.
	//
	// +optional
	Allocated ResourceCapacity `json:"allocated,omitempty"`

	// lastSyncTime is the last time the syncer successfully synced state
	// with the target cluster.
	//
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// syncerVersion is the version of the syncer component currently running
	// for this target.
	//
	// +optional
	SyncerVersion string `json:"syncerVersion,omitempty"`

	// workloadCount is the number of workloads currently assigned to this target.
	//
	// +optional
	WorkloadCount int32 `json:"workloadCount,omitempty"`

	// supportedResourceVersions maps resource types to their supported API versions
	// on the target cluster.
	//
	// +optional
	SupportedResourceVersions map[string][]string `json:"supportedResourceVersions,omitempty"`

	// virtualWorkspaces contains URLs to virtual workspaces for this sync target.
	//
	// +optional
	VirtualWorkspaces []VirtualWorkspace `json:"virtualWorkspaces,omitempty"`
}

// ResourceCapacity represents resource capacity information.
type ResourceCapacity struct {
	// cpu is the CPU capacity.
	//
	// +optional
	CPU *resource.Quantity `json:"cpu,omitempty"`

	// memory is the memory capacity.
	//
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty"`

	// storage is the storage capacity.
	//
	// +optional
	Storage *resource.Quantity `json:"storage,omitempty"`

	// pods is the pod capacity.
	//
	// +optional
	Pods *resource.Quantity `json:"pods,omitempty"`

	// custom contains arbitrary resource capacity information.
	//
	// +optional
	Custom map[string]resource.Quantity `json:"custom,omitempty"`
}

// VirtualWorkspace represents a virtual workspace endpoint for the sync target.
type VirtualWorkspace struct {
	// url is the virtual workspace URL.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:format:URL
	// +required
	URL string `json:"url"`

	// ready indicates whether the virtual workspace is ready to serve requests.
	//
	// +optional
	Ready bool `json:"ready,omitempty"`
}

// GetConditions returns the conditions of the SyncTarget.
func (s *SyncTarget) GetConditions() conditionsv1alpha1.Conditions {
	return s.Status.Conditions
}

// SetConditions sets the conditions of the SyncTarget.
func (s *SyncTarget) SetConditions(conditions conditionsv1alpha1.Conditions) {
	s.Status.Conditions = conditions
}

// GetCondition returns the condition with the given type.
// Returns nil if the condition is not found.
func (s *SyncTarget) GetCondition(conditionType conditionsv1alpha1.ConditionType) *conditionsv1alpha1.Condition {
	for i := range s.Status.Conditions {
		if s.Status.Conditions[i].Type == conditionType {
			return &s.Status.Conditions[i]
		}
	}
	return nil
}

// SetCondition updates or adds a condition to the SyncTarget's status.
// If a condition with the same type already exists, it will be updated.
// Otherwise, the condition will be added to the list.
func (s *SyncTarget) SetCondition(condition conditionsv1alpha1.Condition) {
	existingIndex := -1
	for i := range s.Status.Conditions {
		if s.Status.Conditions[i].Type == condition.Type {
			existingIndex = i
			break
		}
	}
	
	if existingIndex != -1 {
		s.Status.Conditions[existingIndex] = condition
	} else {
		s.Status.Conditions = append(s.Status.Conditions, condition)
	}
}

// HasCondition returns true if the SyncTarget has a condition of the given type.
func (s *SyncTarget) HasCondition(conditionType conditionsv1alpha1.ConditionType) bool {
	return s.GetCondition(conditionType) != nil
}

// IsConditionTrue returns true if the condition with the given type exists and has status True.
func (s *SyncTarget) IsConditionTrue(conditionType conditionsv1alpha1.ConditionType) bool {
	condition := s.GetCondition(conditionType)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// IsConditionFalse returns true if the condition with the given type exists and has status False.
func (s *SyncTarget) IsConditionFalse(conditionType conditionsv1alpha1.ConditionType) bool {
	condition := s.GetCondition(conditionType)
	return condition != nil && condition.Status == corev1.ConditionFalse
}

// IsReady returns true if the SyncTarget is ready to accept workloads.
func (s *SyncTarget) IsReady() bool {
	return s.IsConditionTrue(SyncTargetReady)
}

// IsSyncerReady returns true if the syncer component is connected and operational.
func (s *SyncTarget) IsSyncerReady() bool {
	return s.IsConditionTrue(SyncTargetSyncerReady)
}

// IsClusterReady returns true if the target cluster is reachable and healthy.
func (s *SyncTarget) IsClusterReady() bool {
	return s.IsConditionTrue(SyncTargetClusterReady)
}

// SyncTargetList contains a list of SyncTarget resources.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SyncTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncTarget `json:"items"`
}