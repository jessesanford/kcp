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

package status

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatusAggregator defines the interface for aggregating placement status
// across multiple sync targets and workspaces.
type StatusAggregator interface {
	// AggregateStatus collects and consolidates placement status from all sync targets
	AggregateStatus(ctx context.Context, placement *WorkloadPlacement) (*AggregatedStatus, error)
	
	// CollectSyncTargetStatus gathers status from individual sync targets
	CollectSyncTargetStatus(ctx context.Context, targets []SyncTarget) ([]TargetStatus, error)
	
	// CalculateOverallHealth determines the overall health based on individual target status
	CalculateOverallHealth(statuses []TargetStatus) HealthStatus
	
	// RecordMetrics records aggregated status metrics for observability
	RecordMetrics(status *AggregatedStatus) error
}

// SyncTarget represents a sync target for status collection
type SyncTarget struct {
	// Name is the sync target name
	Name string
	
	// Workspace is the logical cluster workspace
	Workspace string
	
	// URL is the sync target API endpoint
	URL string
	
	// LastSeen is the last time we successfully contacted this target
	LastSeen metav1.Time
}

// TargetStatus represents the status of a single sync target
type TargetStatus struct {
	// Target is the sync target this status refers to
	Target SyncTarget
	
	// Health indicates the health of this target
	Health HealthStatus
	
	// ResourceCount is the number of resources managed by this target
	ResourceCount int
	
	// ReadyResources is the number of ready resources
	ReadyResources int
	
	// LastUpdated is when this status was last updated
	LastUpdated metav1.Time
	
	// Conditions provides detailed status conditions
	Conditions []metav1.Condition
	
	// Error contains any error encountered during status collection
	Error error
}

// AggregatedStatus represents the aggregated status across all sync targets
type AggregatedStatus struct {
	// OverallHealth is the calculated overall health
	OverallHealth HealthStatus
	
	// TargetStatuses contains status for each sync target
	TargetStatuses []TargetStatus
	
	// TotalResources is the total number of resources across all targets
	TotalResources int
	
	// ReadyResources is the total number of ready resources
	ReadyResources int
	
	// HealthyTargets is the number of healthy sync targets
	HealthyTargets int
	
	// TotalTargets is the total number of sync targets
	TotalTargets int
	
	// SuccessPercentage is the percentage of successful placements
	SuccessPercentage float64
	
	// LastAggregated is when this status was last aggregated
	LastAggregated metav1.Time
	
	// AggregationLatency is the time taken to aggregate status
	AggregationLatency time.Duration
}

// HealthStatus represents the health state of a target or aggregated status
type HealthStatus string

const (
	// HealthStatusHealthy indicates all systems are functioning normally
	HealthStatusHealthy HealthStatus = "Healthy"
	
	// HealthStatusDegraded indicates some issues but still functional
	HealthStatusDegraded HealthStatus = "Degraded"
	
	// HealthStatusUnhealthy indicates significant issues affecting functionality
	HealthStatusUnhealthy HealthStatus = "Unhealthy"
	
	// HealthStatusUnknown indicates status cannot be determined
	HealthStatusUnknown HealthStatus = "Unknown"
)

// String returns the string representation of HealthStatus
func (h HealthStatus) String() string {
	return string(h)
}

// IsHealthy returns true if the health status indicates a healthy state
func (h HealthStatus) IsHealthy() bool {
	return h == HealthStatusHealthy
}

// IsDegraded returns true if the health status indicates a degraded state
func (h HealthStatus) IsDegraded() bool {
	return h == HealthStatusDegraded
}

// IsUnhealthy returns true if the health status indicates an unhealthy state
func (h HealthStatus) IsUnhealthy() bool {
	return h == HealthStatusUnhealthy
}

// IsUnknown returns true if the health status is unknown
func (h HealthStatus) IsUnknown() bool {
	return h == HealthStatusUnknown
}

// WorkloadPlacement represents a simplified placement resource for status aggregation
type WorkloadPlacement struct {
	metav1.ObjectMeta
	Spec   WorkloadPlacementSpec
	Status WorkloadPlacementStatus
}

// WorkloadPlacementSpec defines the desired state of workload placement
type WorkloadPlacementSpec struct {
	LocationResource *LocationResourceReference
}

// LocationResourceReference references a location resource
type LocationResourceReference struct {
	Name      string
	Workspace string
}

// WorkloadPlacementStatus defines the observed state of workload placement
type WorkloadPlacementStatus struct {
	Conditions []metav1.Condition
}