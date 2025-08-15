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
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	// Condition types for ClusterRegistration
	ClusterRegistrationReady     = "Ready"
	ClusterRegistrationHealthy   = "Healthy"
	ClusterRegistrationReachable = "Reachable"
	ClusterRegistrationSynced    = "Synced"

	// Condition types for WorkloadPlacement  
	WorkloadPlacementScheduled = "Scheduled"
	WorkloadPlacementDeployed  = "Deployed" 
	WorkloadPlacementReady     = "Ready"
	WorkloadPlacementSynced    = "Synced"

	// Condition reasons for ClusterRegistration
	ClusterRegistrationReasonHealthy    = "Healthy"
	ClusterRegistrationReasonUnhealthy  = "Unhealthy"
	ClusterRegistrationReasonReachable  = "Reachable"
	ClusterRegistrationReasonUnreachable = "Unreachable"

	// Condition reasons for WorkloadPlacement
	WorkloadPlacementReasonScheduled     = "Scheduled"
	WorkloadPlacementReasonNotScheduled  = "NotScheduled"
	WorkloadPlacementReasonDeployed      = "Deployed"
	WorkloadPlacementReasonDeployFailed  = "DeployFailed"
)

const (
	// Label keys for TMC resources
	TMCClusterLabelPrefix     = "tmc.kcp.io/"
	TMCClusterNameLabel       = TMCClusterLabelPrefix + "cluster-name"
	TMCLocationLabel          = TMCClusterLabelPrefix + "location"
	TMCCapabilityLabel        = TMCClusterLabelPrefix + "capability"
	TMCPlacementLabel         = TMCClusterLabelPrefix + "placement"
	TMCWorkloadLabel          = TMCClusterLabelPrefix + "workload"

	// Annotation keys for TMC resources
	TMCAnnotationPrefix       = "tmc.kcp.io/"
	TMCLastHeartbeatAnnotation = TMCAnnotationPrefix + "last-heartbeat"
	TMCPlacementStrategyAnnotation = TMCAnnotationPrefix + "placement-strategy"
	TMCClusterEndpointAnnotation = TMCAnnotationPrefix + "cluster-endpoint"

	// Finalizer names
	TMCClusterFinalizerName = "cluster.tmc.kcp.io/finalizer"
	TMCPlacementFinalizerName = "placement.tmc.kcp.io/finalizer"
)

const (
	// Placement strategies
	PlacementStrategyRoundRobin    = "RoundRobin"
	PlacementStrategyLeastLoaded   = "LeastLoaded"
	PlacementStrategyRandom        = "Random"
	PlacementStrategyLocationAware = "LocationAware"

	// Default placement strategy when none specified
	DefaultPlacementStrategy = PlacementStrategyRoundRobin

	// Default number of clusters for placement
	DefaultNumberOfClusters = int32(1)

	// Default health check interval
	DefaultHealthCheckInterval = 30 * time.Second

	// Default heartbeat timeout
	DefaultHeartbeatTimeout = 90 * time.Second
)

const (
	// Workload status values
	WorkloadStatusPending = "Pending"
	WorkloadStatusPlaced  = "Placed"
	WorkloadStatusFailed  = "Failed"
	WorkloadStatusRemoved = "Removed"

	// Anti-affinity types
	AntiAffinityTypeCluster  = "cluster"
	AntiAffinityTypeWorkload = "workload"

	// Resource requirement units
	ResourceUnitCPUMillis = "millicpu"
	ResourceUnitMemoryBytes = "bytes"
	ResourceUnitStorageBytes = "bytes"
)

// Validation error messages
var (
	ErrInvalidClusterName      = errors.New("cluster name must be a valid DNS-1123 label")
	ErrInvalidPlacementStrategy = errors.New("placement strategy must be one of the supported strategies")
	ErrInvalidLocation         = errors.New("location must be a non-empty string")
	ErrInvalidEndpoint         = errors.New("cluster endpoint must have a valid server URL")
	ErrInvalidNumberOfClusters = errors.New("number of clusters must be greater than 0")
)

// Cluster name validation regex (DNS-1123 label)
var clusterNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// ValidateClusterName validates that a cluster name meets TMC requirements.
// Cluster names MUST be valid DNS-1123 labels and between 1-63 characters.
func ValidateClusterName(name string) error {
	if name == "" {
		return ErrInvalidClusterName
	}
	if len(name) > 63 {
		return ErrInvalidClusterName
	}
	if !clusterNameRegex.MatchString(name) {
		return ErrInvalidClusterName
	}
	return nil
}

// ValidatePlacementStrategy validates that a placement strategy is supported.
// Only predefined strategy constants are considered valid.
func ValidatePlacementStrategy(strategy string) error {
	switch strategy {
	case PlacementStrategyRoundRobin,
		PlacementStrategyLeastLoaded,
		PlacementStrategyRandom,
		PlacementStrategyLocationAware:
		return nil
	default:
		return ErrInvalidPlacementStrategy
	}
}

// ValidateLocation validates that a location string meets TMC requirements.
// Locations MUST be non-empty and contain only valid characters.
func ValidateLocation(location string) error {
	if strings.TrimSpace(location) == "" {
		return ErrInvalidLocation
	}
	return nil
}

// ValidateClusterEndpoint validates that a cluster endpoint is properly configured.
// The server URL MUST be a valid HTTP/HTTPS URL.
func ValidateClusterEndpoint(endpoint ClusterEndpointInfo) error {
	if endpoint.ServerURL == "" {
		return ErrInvalidEndpoint
	}
	
	// Basic URL validation - must start with http:// or https://
	if !strings.HasPrefix(endpoint.ServerURL, "http://") && 
		!strings.HasPrefix(endpoint.ServerURL, "https://") {
		return ErrInvalidEndpoint
	}
	
	return nil
}

// ValidateNumberOfClusters validates that the number of clusters is reasonable.
// The value MUST be greater than 0 for valid placements.
func ValidateNumberOfClusters(num int32) error {
	if num <= 0 {
		return ErrInvalidNumberOfClusters
	}
	return nil
}

// IsValidPlacementStrategy returns true if the strategy is one of the supported strategies.
// This is a convenience function for strategy validation in controllers.
func IsValidPlacementStrategy(strategy string) bool {
	return ValidatePlacementStrategy(strategy) == nil
}

// GetSupportedPlacementStrategies returns a list of all supported placement strategies.
// Controllers can use this to provide user-friendly error messages or UI options.
func GetSupportedPlacementStrategies() []string {
	return []string{
		PlacementStrategyRoundRobin,
		PlacementStrategyLeastLoaded,
		PlacementStrategyRandom,
		PlacementStrategyLocationAware,
	}
}

// GetDefaultValues returns a struct containing all default values used by TMC.
// This is useful for controllers that need to set defaults programmatically.
func GetDefaultValues() DefaultValues {
	return DefaultValues{
		PlacementStrategy:      DefaultPlacementStrategy,
		NumberOfClusters:      DefaultNumberOfClusters,
		HealthCheckInterval:   DefaultHealthCheckInterval,
		HeartbeatTimeout:     DefaultHeartbeatTimeout,
	}
}

// DefaultValues contains all default values used by TMC components.
type DefaultValues struct {
	PlacementStrategy    string
	NumberOfClusters     int32
	HealthCheckInterval  time.Duration
	HeartbeatTimeout     time.Duration
}