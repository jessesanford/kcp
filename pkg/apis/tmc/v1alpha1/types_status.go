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

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// WorkloadStatusAggregator provides unified status views for TMC workloads
// across multiple clusters. This enables TMC to present consistent status
// information regardless of where workloads are actually running.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories=tmc
// +kubebuilder:printcolumn:name="Total Workloads",type="integer",JSONPath=".status.totalWorkloads"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyWorkloads"
// +kubebuilder:printcolumn:name="Overall Status",type="string",JSONPath=".status.overallStatus"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type WorkloadStatusAggregator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadStatusAggregatorSpec   `json:"spec,omitempty"`
	Status WorkloadStatusAggregatorStatus `json:"status,omitempty"`
}

// WorkloadStatusAggregatorSpec defines which workloads to aggregate status for
type WorkloadStatusAggregatorSpec struct {
	// WorkloadSelector specifies which workloads to aggregate status for
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector specifies which clusters to collect status from
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// StatusFields defines which status fields to aggregate
	// +optional
	StatusFields []StatusFieldSelector `json:"statusFields,omitempty"`

	// UpdateInterval defines how often to update aggregated status
	// +optional
	UpdateInterval *metav1.Duration `json:"updateInterval,omitempty"`
}

// StatusFieldSelector defines which status fields to extract and aggregate
type StatusFieldSelector struct {
	// FieldPath is the JSONPath to the status field (e.g., "status.replicas")
	FieldPath string `json:"fieldPath"`

	// AggregationType defines how to aggregate this field across clusters
	AggregationType StatusAggregationType `json:"aggregationType"`

	// DisplayName is the name to use when displaying this field
	// +optional
	DisplayName string `json:"displayName,omitempty"`
}

// StatusAggregationType defines how to aggregate status fields
type StatusAggregationType string

const (
	// StatusSumAggregation sums numeric values across clusters
	StatusSumAggregation StatusAggregationType = "Sum"
	// StatusMaxAggregation takes the maximum value across clusters
	StatusMaxAggregation StatusAggregationType = "Max"
	// StatusMinAggregation takes the minimum value across clusters
	StatusMinAggregation StatusAggregationType = "Min"
	// StatusAverageAggregation averages numeric values across clusters
	StatusAverageAggregation StatusAggregationType = "Average"
	// StatusFirstNonEmptyAggregation takes the first non-empty value
	StatusFirstNonEmptyAggregation StatusAggregationType = "FirstNonEmpty"
	// StatusConcatAggregation concatenates string values
	StatusConcatAggregation StatusAggregationType = "Concat"
)

// WorkloadStatusAggregatorStatus shows the aggregated status across clusters
type WorkloadStatusAggregatorStatus struct {
	// Conditions represent the latest available observations
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// LastUpdateTime indicates when status was last aggregated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// TotalWorkloads is the total number of workloads being tracked
	// +optional
	TotalWorkloads *int32 `json:"totalWorkloads,omitempty"`

	// ReadyWorkloads is the number of workloads that are ready
	// +optional
	ReadyWorkloads *int32 `json:"readyWorkloads,omitempty"`

	// OverallStatus provides a summary status across all workloads
	// +optional
	OverallStatus WorkloadOverallStatus `json:"overallStatus,omitempty"`

	// ClusterStatus shows status breakdown by cluster
	// +optional
	ClusterStatus map[string]ClusterWorkloadStatus `json:"clusterStatus,omitempty"`

	// WorkloadStatus shows individual workload statuses
	// +optional
	WorkloadStatus []WorkloadStatusSummary `json:"workloadStatus,omitempty"`

	// AggregatedFields contains aggregated status field values
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	AggregatedFields map[string]string `json:"aggregatedFields,omitempty"`
}

// WorkloadOverallStatus represents the overall status across all workloads
type WorkloadOverallStatus string

const (
	// AllReadyStatus indicates all workloads are ready
	AllReadyStatus WorkloadOverallStatus = "AllReady"
	// MostlyReadyStatus indicates most workloads are ready (>80%)
	MostlyReadyStatus WorkloadOverallStatus = "MostlyReady"
	// PartiallyReadyStatus indicates some workloads are ready (20-80%)
	PartiallyReadyStatus WorkloadOverallStatus = "PartiallyReady"
	// NotReadyStatus indicates few or no workloads are ready (<20%)
	NotReadyStatus WorkloadOverallStatus = "NotReady"
	// UnknownStatus indicates status cannot be determined
	UnknownStatus WorkloadOverallStatus = "Unknown"
)

// ClusterWorkloadStatus shows workload status for a specific cluster
type ClusterWorkloadStatus struct {
	// ClusterName identifies the cluster
	ClusterName string `json:"clusterName"`

	// WorkloadCount is the number of workloads in this cluster
	WorkloadCount int32 `json:"workloadCount"`

	// ReadyCount is the number of ready workloads in this cluster
	ReadyCount int32 `json:"readyCount"`

	// LastSeen indicates when this cluster was last observed
	LastSeen metav1.Time `json:"lastSeen"`

	// Reachable indicates if the cluster is currently reachable
	Reachable bool `json:"reachable"`
}

// WorkloadStatusSummary provides a summary of an individual workload's status
type WorkloadStatusSummary struct {
	// WorkloadRef identifies the workload
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// ClusterName indicates which cluster this workload is in
	ClusterName string `json:"clusterName"`

	// Ready indicates if the workload is ready
	Ready bool `json:"ready"`

	// Phase indicates the current workload phase
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides additional status information
	// +optional
	Message string `json:"message,omitempty"`

	// LastTransitionTime indicates when the status last changed
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Conditions shows workload-specific conditions
	// +optional
	Conditions []WorkloadCondition `json:"conditions,omitempty"`
}

// WorkloadCondition represents a condition of a workload
type WorkloadCondition struct {
	// Type is the type of condition
	Type string `json:"type"`

	// Status is the status of the condition (True, False, Unknown)
	Status metav1.ConditionStatus `json:"status"`

	// Reason is a brief reason for the condition's status
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message for the condition
	// +optional
	Message string `json:"message,omitempty"`

	// LastTransitionTime is when the condition last changed
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// WorkloadStatusAggregatorList contains a list of WorkloadStatusAggregator
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadStatusAggregatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadStatusAggregator `json:"items"`
}
