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
)

// ClusterRegistrationInterface defines the contract for cluster registration resources.
// Implementations MUST provide these methods for TMC controllers to function properly.
// This interface abstracts cluster registration operations across different implementations.
type ClusterRegistrationInterface interface {
	// GetName returns the cluster's unique identifier within the workspace.
	// The name MUST be unique within the workspace and immutable once set.
	GetName() string

	// GetLocation returns the geographic or logical location of the cluster.
	// Location is used for placement decisions and locality-aware scheduling.
	// MUST return a non-empty string for valid registrations.
	GetLocation() string

	// GetCapabilities returns the list of capabilities this cluster provides.
	// Capabilities include supported API versions, available resources, and features.
	// The list MUST be stable for a given cluster configuration.
	GetCapabilities() []string

	// IsReady returns true if the cluster is ready to accept workloads.
	// A cluster is ready when it passes health checks and can communicate.
	// Controllers MUST NOT place workloads on non-ready clusters.
	IsReady() bool

	// IsHealthy returns true if the cluster is currently healthy.
	// Health is determined by recent heartbeats and operational status.
	// Unhealthy clusters may trigger workload redistribution.
	IsHealthy() bool

	// GetConditions returns the current condition set for the cluster.
	// Conditions provide detailed status information for debugging and monitoring.
	// The returned slice MUST be sorted by condition type for consistency.
	GetConditions() []metav1.Condition

	// GetLastHeartbeat returns the timestamp of the last successful heartbeat.
	// Controllers use heartbeats to determine cluster health and availability.
	// A nil return indicates no heartbeat has been received.
	GetLastHeartbeat() *metav1.Time

	// GetEndpoint returns the connection information for accessing the cluster.
	// The endpoint MUST be valid and accessible from TMC controllers.
	GetEndpoint() ClusterEndpointInfo
}

// WorkloadPlacementInterface defines the contract for workload placement policies.
// Implementations MUST provide these methods to enable workload distribution
// across clusters according to defined placement strategies.
type WorkloadPlacementInterface interface {
	// GetTargetClusters returns the list of cluster names selected for placement.
	// The list MUST be ordered by preference (best match first).
	// An empty list indicates no suitable clusters found.
	GetTargetClusters() []string

	// GetSelector returns the label selector used to match clusters.
	// The selector defines which clusters are eligible for placement.
	// A nil selector matches all available clusters.
	GetSelector() *metav1.LabelSelector

	// GetStrategy returns the placement strategy name.
	// Strategy determines how clusters are selected and workloads distributed.
	// MUST return one of the predefined strategy constants.
	GetStrategy() string

	// GetNumberOfClusters returns the desired number of target clusters.
	// This value guides cluster selection algorithms.
	// A value of 0 indicates no limit on cluster count.
	GetNumberOfClusters() int32

	// IsPlaced returns true if workloads have been successfully placed.
	// Placement is considered successful when all target clusters accept workloads.
	IsPlaced() bool

	// GetPlacedWorkloads returns information about successfully placed workloads.
	// The returned slice contains references to workloads and their target clusters.
	GetPlacedWorkloads() []PlacedWorkloadInfo

	// GetLastPlacementTime returns when the most recent placement occurred.
	// Controllers use this timestamp for placement scheduling and reconciliation.
	// A nil return indicates no placement has occurred.
	GetLastPlacementTime() *metav1.Time

	// GetConditions returns the current condition set for the placement.
	// Conditions provide detailed status information for placement operations.
	GetConditions() []metav1.Condition
}

// PlacementStrategyInterface defines the contract for placement strategy implementations.
// Strategies determine how clusters are selected and workloads are distributed
// based on various criteria like load, location, and resource availability.
type PlacementStrategyInterface interface {
	// Evaluate analyzes available clusters and returns selected cluster names.
	// The algorithm MUST consider cluster health, capacity, and placement constraints.
	// The returned list MUST be ordered by preference (best match first).
	// An empty list indicates no suitable clusters found.
	// 
	// Parameters:
	//   clusters: Available clusters for evaluation
	//   maxClusters: Maximum number of clusters to select (0 = no limit)
	//   constraints: Additional placement constraints to consider
	//
	// Returns:
	//   selectedClusters: Ordered list of cluster names for placement
	//   reason: Human-readable explanation of the selection logic
	//   error: Any error encountered during evaluation
	Evaluate(clusters []ClusterRegistrationInterface, maxClusters int32, constraints PlacementConstraints) (selectedClusters []string, reason string, err error)

	// GetName returns the unique name identifier for this strategy.
	// Strategy names MUST match the constants defined in contracts.
	GetName() string

	// SupportsConstraints returns true if this strategy supports the given constraints.
	// Controllers use this method to validate placement configurations.
	SupportsConstraints(constraints PlacementConstraints) bool
}

// StatusUpdaterInterface defines the contract for updating resource status.
// Implementations MUST provide these methods to enable status management
// across different resource types in a consistent manner.
type StatusUpdaterInterface interface {
	// UpdateConditions updates the condition set for a resource.
	// The implementation MUST merge conditions intelligently:
	// - Update existing conditions with the same type
	// - Add new conditions that don't exist
	// - Preserve conditions not mentioned in the update
	//
	// Conditions MUST be sorted by type after the update.
	UpdateConditions(conditions []metav1.Condition) error

	// SetCondition sets a single condition, updating or adding as needed.
	// This is a convenience method for single condition updates.
	SetCondition(conditionType string, status metav1.ConditionStatus, reason, message string) error

	// GetConditions returns the current condition set.
	// The returned slice MUST be sorted by condition type.
	GetConditions() []metav1.Condition

	// IsConditionTrue returns true if the specified condition exists and is True.
	// Returns false if the condition doesn't exist or has a different status.
	IsConditionTrue(conditionType string) bool
}

// ClusterEndpointInfo contains connection information for a cluster.
// This is a value type used by the ClusterRegistrationInterface.
type ClusterEndpointInfo struct {
	// ServerURL is the URL of the Kubernetes API server
	ServerURL string

	// CABundle contains the certificate authority bundle for the cluster
	CABundle []byte

	// InsecureSkipTLSVerify controls whether to skip certificate verification
	InsecureSkipTLSVerify bool
}

// PlacedWorkloadInfo contains information about a workload placed on a cluster.
// This is a value type used by the WorkloadPlacementInterface.
type PlacedWorkloadInfo struct {
	// WorkloadRef references the placed workload
	WorkloadRef WorkloadReference

	// ClusterName is the name of the cluster where the workload was placed
	ClusterName string

	// PlacementTime is when the workload was placed
	PlacementTime metav1.Time

	// Status indicates the current status of the placed workload
	Status string
}

// WorkloadReference references a Kubernetes workload resource.
// This is a value type used throughout TMC interfaces.
type WorkloadReference struct {
	// APIVersion is the API version of the workload
	APIVersion string

	// Kind is the kind of the workload
	Kind string

	// Name is the name of the workload
	Name string

	// Namespace is the namespace of the workload (optional for cluster-scoped resources)
	Namespace string
}

// PlacementConstraints defines additional constraints for placement decisions.
// This is a value type used by the PlacementStrategyInterface.
type PlacementConstraints struct {
	// LocationAffinities specify preferred or required locations
	LocationAffinities []LocationAffinity

	// ResourceRequirements specify minimum resource requirements
	ResourceRequirements ResourceRequirements

	// AntiAffinities specify clusters or workloads to avoid
	AntiAffinities []AntiAffinity
}

// LocationAffinity defines location-based placement preferences.
type LocationAffinity struct {
	// Locations are the preferred or required locations
	Locations []string

	// Required indicates if this is a hard requirement (true) or preference (false)
	Required bool

	// Weight is the preference weight for soft affinities (1-100)
	Weight int32
}

// ResourceRequirements defines minimum resource requirements for placement.
type ResourceRequirements struct {
	// CPU requirements in milliCPU
	CPU *int64

	// Memory requirements in bytes
	Memory *int64

	// Storage requirements in bytes
	Storage *int64
}

// AntiAffinity defines resources or clusters to avoid during placement.
type AntiAffinity struct {
	// Type indicates what to avoid (e.g., "cluster", "workload")
	Type string

	// LabelSelector selects resources to avoid based on labels
	LabelSelector *metav1.LabelSelector
}