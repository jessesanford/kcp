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

package autoscaling

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// MetricsCollector defines the interface for collecting metrics across clusters.
// Implementations should gather metrics from all relevant clusters for making
// scaling decisions.
type MetricsCollector interface {
	// CollectMetrics gathers current metrics for the given HPA policy across all relevant clusters.
	// It returns aggregated metrics data that can be used for scaling decisions.
	CollectMetrics(ctx context.Context, hpaPolicy *tmcv1alpha1.HorizontalPodAutoscalerPolicy) (*MetricsData, error)
}

// ScalingExecutor defines the interface for executing scaling decisions.
// Implementations should handle the actual scaling operations across clusters
// based on the scaling decisions provided.
type ScalingExecutor interface {
	// ExecuteScaling performs the actual scaling operations across clusters
	// based on the scaling decision provided.
	ExecuteScaling(ctx context.Context, hpaPolicy *tmcv1alpha1.HorizontalPodAutoscalerPolicy, decision *ScalingDecision) error
}

// MetricsData contains metrics collected from all relevant clusters.
type MetricsData struct {
	// Timestamp when the metrics were collected
	Timestamp time.Time

	// ClusterMetrics contains per-cluster metrics data
	ClusterMetrics map[string]*ClusterMetricsData
}

// ClusterMetricsData contains metrics for a single cluster.
type ClusterMetricsData struct {
	// ClusterName is the name of the cluster
	ClusterName string

	// CurrentReplicas is the current number of replicas in this cluster
	CurrentReplicas *int32

	// DesiredReplicas is the desired number of replicas in this cluster
	DesiredReplicas *int32

	// ResourceUtilization contains resource utilization metrics (e.g., CPU percentage)
	ResourceUtilization *float64

	// CustomMetrics contains custom metrics specific to this cluster
	CustomMetrics map[string]float64

	// LastScaleTime is the last time this cluster was scaled
	LastScaleTime *metav1.Time

	// HealthScore represents the health score of this cluster (0-100)
	HealthScore *float64

	// Capacity contains information about cluster capacity
	Capacity *ClusterCapacity
}

// ClusterCapacity represents the capacity information for a cluster.
type ClusterCapacity struct {
	// MaxReplicas is the maximum number of replicas this cluster can handle
	MaxReplicas int32

	// AvailableResources contains available resources in this cluster
	AvailableResources map[string]float64

	// LoadPercentage represents the current load on this cluster (0-100)
	LoadPercentage float64
}

// ScalingDecision represents the result of a scaling decision calculation.
type ScalingDecision struct {
	// ShouldScale indicates whether scaling action should be taken
	ShouldScale bool

	// CurrentReplicas is the current total number of replicas across all clusters
	CurrentReplicas int32

	// DesiredReplicas is the desired total number of replicas across all clusters
	DesiredReplicas int32

	// Reason provides a human-readable explanation for the scaling decision
	Reason string

	// ClusterDecisions contains per-cluster scaling decisions
	ClusterDecisions []ClusterScalingDecision

	// Timestamp when this decision was made
	Timestamp time.Time

	// StabilizationWindow indicates how long to wait before making another scaling decision
	StabilizationWindow time.Duration
}

// ClusterScalingDecision represents a scaling decision for a specific cluster.
type ClusterScalingDecision struct {
	// ClusterName is the name of the cluster
	ClusterName string

	// CurrentReplicas is the current number of replicas in this cluster
	CurrentReplicas int32

	// DesiredReplicas is the desired number of replicas in this cluster
	DesiredReplicas int32

	// ScalingAction indicates the type of scaling action needed
	ScalingAction ScalingAction

	// Priority indicates the priority of this scaling action (higher number = higher priority)
	Priority int

	// Reason provides explanation for this cluster-specific decision
	Reason string
}

// ScalingAction represents the type of scaling action to be performed.
type ScalingAction string

const (
	// ScaleUp indicates replicas should be increased
	ScaleUp ScalingAction = "ScaleUp"

	// ScaleDown indicates replicas should be decreased
	ScaleDown ScalingAction = "ScaleDown"

	// NoAction indicates no scaling action is needed
	NoAction ScalingAction = "NoAction"
)

// MetricsQuery represents a query for specific metrics.
type MetricsQuery struct {
	// MetricName is the name of the metric to query
	MetricName string

	// Selector is used to filter the metrics query
	Selector map[string]string

	// TimeRange specifies the time range for the metrics query
	TimeRange *TimeRange
}

// TimeRange represents a time range for metrics queries.
type TimeRange struct {
	// Start is the start time of the range
	Start time.Time

	// End is the end time of the range
	End time.Time

	// Duration is the duration of the range (alternative to Start/End)
	Duration time.Duration
}

// ScalingEvent represents a scaling event that occurred.
type ScalingEvent struct {
	// Timestamp when the scaling event occurred
	Timestamp time.Time

	// PolicyName is the name of the HPA policy that triggered the event
	PolicyName string

	// ClusterName is the name of the cluster where scaling occurred
	ClusterName string

	// Action is the scaling action that was performed
	Action ScalingAction

	// FromReplicas is the number of replicas before scaling
	FromReplicas int32

	// ToReplicas is the number of replicas after scaling
	ToReplicas int32

	// Reason provides explanation for the scaling event
	Reason string

	// Success indicates whether the scaling operation was successful
	Success bool

	// Error contains error information if the scaling failed
	Error string
}