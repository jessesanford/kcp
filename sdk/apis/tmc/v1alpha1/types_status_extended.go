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
	"k8s.io/apimachinery/pkg/util/intstr"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// This file contains extended TMC status types for advanced placement, scaling, traffic monitoring,
// and session management. These types build upon the core status types for advanced TMC features.

// ==========================
// Advanced Placement Status
// ==========================

// WorkloadPlacementAdvancedStatus communicates advanced placement policy state.
type WorkloadPlacementAdvancedStatus struct {
	// Conditions represent the latest available observations
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// SelectedClusters lists the clusters selected for workload placement
	// +optional
	SelectedClusters []string `json:"selectedClusters,omitempty"`

	// PlacedWorkloads tracks the workloads that have been placed
	// +optional
	PlacedWorkloads []PlacedWorkloadAdvanced `json:"placedWorkloads,omitempty"`

	// RolloutState tracks the current rollout state
	// +optional
	RolloutState *RolloutState `json:"rolloutState,omitempty"`

	// TrafficState tracks the current traffic splitting state
	// +optional
	TrafficState *TrafficState `json:"trafficState,omitempty"`

	// LastPlacementTime is the timestamp of the last placement decision
	// +optional
	LastPlacementTime *metav1.Time `json:"lastPlacementTime,omitempty"`
}

// PlacedWorkloadAdvanced represents a workload placed with advanced features
type PlacedWorkloadAdvanced struct {
	// WorkloadRef references the placed workload
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// ClusterName is the name of the cluster where the workload was placed
	ClusterName string `json:"clusterName"`

	// PlacementTime is when the workload was placed
	PlacementTime metav1.Time `json:"placementTime"`

	// Status indicates the current status of the placed workload
	Status PlacedWorkloadStatus `json:"status"`

	// RolloutStage indicates which rollout stage this workload is in
	// +optional
	RolloutStage string `json:"rolloutStage,omitempty"`

	// TrafficWeight indicates the percentage of traffic routed to this workload
	// +optional
	TrafficWeight *int32 `json:"trafficWeight,omitempty"`
}

// RolloutState tracks the current state of a rollout operation
type RolloutState struct {
	// Phase indicates the current phase of the rollout
	Phase RolloutPhase `json:"phase"`

	// CurrentStage indicates the current stage in the rollout
	CurrentStage int32 `json:"currentStage"`

	// TotalStages indicates the total number of rollout stages
	TotalStages int32 `json:"totalStages"`

	// StartTime indicates when the rollout started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime indicates when the rollout completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Message provides additional details about the rollout state
	// +optional
	Message string `json:"message,omitempty"`
}

// TrafficState tracks the current state of traffic splitting
type TrafficState struct {
	// TargetWeights defines the desired traffic weights per cluster
	TargetWeights map[string]int32 `json:"targetWeights"`

	// CurrentWeights defines the actual traffic weights per cluster
	CurrentWeights map[string]int32 `json:"currentWeights"`

	// LastUpdated indicates when traffic weights were last updated
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// ==========================
// Scaling Status Types
// ==========================

// WorkloadScalingPolicyStatus defines the observed scaling state.
type WorkloadScalingPolicyStatus struct {
	// Conditions represent the latest available observations
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// CurrentReplicas is the current total number of replicas across clusters
	// +optional
	CurrentReplicas *int32 `json:"currentReplicas,omitempty"`

	// DesiredReplicas is the desired total number of replicas
	// +optional
	DesiredReplicas *int32 `json:"desiredReplicas,omitempty"`

	// ClusterReplicas shows current replica distribution across clusters
	// +optional
	ClusterReplicas map[string]int32 `json:"clusterReplicas,omitempty"`

	// LastScaleTime indicates when the last scaling operation occurred
	// +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`

	// CurrentMetrics shows current values of scaling metrics
	// +optional
	CurrentMetrics []CurrentMetricStatus `json:"currentMetrics,omitempty"`

	// ObservedWorkloads lists workloads currently managed by this policy
	// +optional
	ObservedWorkloads []WorkloadReference `json:"observedWorkloads,omitempty"`
}

// CurrentMetricStatus shows the current status of a scaling metric
type CurrentMetricStatus struct {
	// Type identifies the metric type
	Type ScalingMetricType `json:"type"`

	// CurrentValue is the current value of the metric
	CurrentValue intstr.IntOrString `json:"currentValue"`

	// TargetValue is the target value for this metric
	TargetValue intstr.IntOrString `json:"targetValue"`

	// MetricName is the name of the metric (for custom metrics)
	// +optional
	MetricName string `json:"metricName,omitempty"`
}

// ==========================
// Session Management Status
// ==========================

// WorkloadSessionPolicyStatus tracks session affinity policy state.
type WorkloadSessionPolicyStatus struct {
	// Conditions represent the latest available observations
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// ActiveSessions tracks the number of active sessions per backend
	// +optional
	ActiveSessions map[string]int32 `json:"activeSessions,omitempty"`

	// BackendHealth tracks the health status of session backends
	// +optional
	BackendHealth []SessionBackendStatus `json:"backendHealth,omitempty"`

	// LastSessionRouted indicates when the last session was routed
	// +optional
	LastSessionRouted *metav1.Time `json:"lastSessionRouted,omitempty"`

	// SessionDistribution shows how sessions are distributed across backends
	// +optional
	SessionDistribution map[string]float64 `json:"sessionDistribution,omitempty"`
}

// SessionBackendStatus tracks the status of a session backend
type SessionBackendStatus struct {
	// BackendName identifies the backend
	BackendName string `json:"backendName"`

	// ClusterName identifies the cluster hosting the backend
	ClusterName string `json:"clusterName"`

	// HealthStatus indicates the health of the backend
	HealthStatus BackendHealthStatus `json:"healthStatus"`

	// ActiveConnections shows the number of active connections
	ActiveConnections int32 `json:"activeConnections"`

	// LastHealthCheck indicates when the backend was last checked
	// +optional
	LastHealthCheck *metav1.Time `json:"lastHealthCheck,omitempty"`

	// ResponseTime shows the average response time
	// +optional
	ResponseTime *metav1.Duration `json:"responseTime,omitempty"`
}

// ==========================
// Traffic Monitoring Status
// ==========================

// TrafficMetricsStatus tracks traffic patterns and performance metrics.
type TrafficMetricsStatus struct {
	// Conditions represent the latest available observations
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// RequestsPerSecond tracks current request rates
	// +optional
	RequestsPerSecond map[string]float64 `json:"requestsPerSecond,omitempty"`

	// ResponseTimes tracks response time percentiles
	// +optional
	ResponseTimes *ResponseTimeMetrics `json:"responseTimes,omitempty"`

	// ErrorRates tracks error rates per cluster
	// +optional
	ErrorRates map[string]float64 `json:"errorRates,omitempty"`

	// LastUpdated indicates when metrics were last updated
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// CollectionWindow indicates the time window for metric collection
	// +optional
	CollectionWindow *metav1.Duration `json:"collectionWindow,omitempty"`
}

// ResponseTimeMetrics contains response time statistics
type ResponseTimeMetrics struct {
	// P50 is the 50th percentile response time
	P50 metav1.Duration `json:"p50"`

	// P90 is the 90th percentile response time
	P90 metav1.Duration `json:"p90"`

	// P99 is the 99th percentile response time
	P99 metav1.Duration `json:"p99"`

	// Average is the average response time
	Average metav1.Duration `json:"average"`
}

// ==========================
// Extended Types and Enums
// ==========================

// WorkloadReference references a Kubernetes workload (shared with core types)
type WorkloadReference struct {
	// APIVersion is the API version of the workload
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the workload
	Kind string `json:"kind"`

	// Name is the name of the workload
	Name string `json:"name"`

	// Namespace is the namespace of the workload
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// PlacedWorkloadStatus represents the status of a placed workload (shared with core types)
// +kubebuilder:validation:Enum=Pending;Placed;Failed;Removed
type PlacedWorkloadStatus string

const (
	// PlacedWorkloadStatusPending indicates the workload is waiting to be placed
	PlacedWorkloadStatusPending PlacedWorkloadStatus = "Pending"

	// PlacedWorkloadStatusPlaced indicates the workload has been successfully placed
	PlacedWorkloadStatusPlaced PlacedWorkloadStatus = "Placed"

	// PlacedWorkloadStatusFailed indicates the workload placement failed
	PlacedWorkloadStatusFailed PlacedWorkloadStatus = "Failed"

	// PlacedWorkloadStatusRemoved indicates the workload has been removed
	PlacedWorkloadStatusRemoved PlacedWorkloadStatus = "Removed"
)

// RolloutPhase represents the phase of a rollout operation
// +kubebuilder:validation:Enum=Pending;InProgress;Paused;Completed;Failed
type RolloutPhase string

const (
	// RolloutPhasePending indicates the rollout is pending
	RolloutPhasePending RolloutPhase = "Pending"

	// RolloutPhaseInProgress indicates the rollout is in progress
	RolloutPhaseInProgress RolloutPhase = "InProgress"

	// RolloutPhasePaused indicates the rollout is paused
	RolloutPhasePaused RolloutPhase = "Paused"

	// RolloutPhaseCompleted indicates the rollout is completed
	RolloutPhaseCompleted RolloutPhase = "Completed"

	// RolloutPhaseFailed indicates the rollout failed
	RolloutPhaseFailed RolloutPhase = "Failed"
)

// BackendHealthStatus represents the health status of a session backend
// +kubebuilder:validation:Enum=Healthy;Unhealthy;Unknown
type BackendHealthStatus string

const (
	// BackendHealthStatusHealthy indicates the backend is healthy
	BackendHealthStatusHealthy BackendHealthStatus = "Healthy"

	// BackendHealthStatusUnhealthy indicates the backend is unhealthy
	BackendHealthStatusUnhealthy BackendHealthStatus = "Unhealthy"

	// BackendHealthStatusUnknown indicates the backend health is unknown
	BackendHealthStatusUnknown BackendHealthStatus = "Unknown"
)

// ScalingMetricType defines the types of metrics for scaling
// +kubebuilder:validation:Enum=CPUUtilization;MemoryUtilization;RequestsPerSecond;QueueLength;Custom
type ScalingMetricType string

const (
	// CPUUtilizationMetric scales based on CPU utilization percentage
	CPUUtilizationMetric ScalingMetricType = "CPUUtilization"
	// MemoryUtilizationMetric scales based on memory utilization percentage
	MemoryUtilizationMetric ScalingMetricType = "MemoryUtilization"
	// RequestsPerSecondMetric scales based on requests per second
	RequestsPerSecondMetric ScalingMetricType = "RequestsPerSecond"
	// QueueLengthMetric scales based on queue length
	QueueLengthMetric ScalingMetricType = "QueueLength"
	// CustomMetric scales based on a custom metric
	CustomMetric ScalingMetricType = "Custom"
)