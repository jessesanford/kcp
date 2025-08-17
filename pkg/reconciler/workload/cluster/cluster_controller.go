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

package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

// ClusterManager provides cluster registration management capabilities
// for the TMC workload placement system.
type ClusterManager struct {
	// allowedLocations defines the set of valid cluster locations
	allowedLocations sets.Set[string]
	
	// requiredLabels defines the mandatory labels for cluster registration
	requiredLabels sets.Set[string]
	
	// defaultCapabilities defines the default capabilities assigned to new clusters
	defaultCapabilities map[string]string
}

// NewClusterManager creates a new cluster manager with TMC-specific configuration.
func NewClusterManager() *ClusterManager {
	return &ClusterManager{
		allowedLocations: sets.New(
			"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1",
			"us-central-1", "eu-central-1", "ap-northeast-1",
		),
		requiredLabels: sets.New(
			"region", "zone", "provider",
		),
		defaultCapabilities: map[string]string{
			"compute":     "true",
			"storage":     "true", 
			"networking":  "true",
			"kubernetes":  "true",
		},
	}
}

// ValidateClusterRegistration performs comprehensive validation of cluster registration.
func (cm *ClusterManager) ValidateClusterRegistration(cluster *ClusterRegistration) error {
	var validationErrors []string

	// Validate basic fields
	if cluster.Name == "" {
		validationErrors = append(validationErrors, "cluster name is required")
	}
	
	if cluster.Spec.Location == "" {
		validationErrors = append(validationErrors, "cluster location is required")
	} else if !cm.allowedLocations.Has(cluster.Spec.Location) {
		validationErrors = append(validationErrors, fmt.Sprintf("location %q is not allowed, must be one of: %v", 
			cluster.Spec.Location, sets.List(cm.allowedLocations)))
	}

	// Validate required labels
	if cluster.Spec.Labels == nil {
		cluster.Spec.Labels = make(map[string]string)
	}
	
	for requiredLabel := range cm.requiredLabels {
		if _, exists := cluster.Spec.Labels[requiredLabel]; !exists {
			validationErrors = append(validationErrors, fmt.Sprintf("required label %q is missing", requiredLabel))
		}
	}

	// Validate label values
	if err := cm.validateLabelValues(cluster.Spec.Labels); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}

	// Validate capabilities
	if err := cm.validateCapabilities(cluster.Spec.Capabilities); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("cluster registration validation failed: %s", strings.Join(validationErrors, "; "))
	}

	return nil
}

// validateLabelValues validates the values of cluster labels.
func (cm *ClusterManager) validateLabelValues(labels map[string]string) error {
	// Validate region label
	if region, exists := labels["region"]; exists {
		validRegions := sets.New("us", "eu", "ap")
		regionPrefix := strings.Split(region, "-")[0]
		if !validRegions.Has(regionPrefix) {
			return fmt.Errorf("invalid region %q, must start with us-, eu-, or ap-", region)
		}
	}

	// Validate provider label
	if provider, exists := labels["provider"]; exists {
		validProviders := sets.New("aws", "gcp", "azure", "on-premises")
		if !validProviders.Has(provider) {
			return fmt.Errorf("invalid provider %q, must be one of: %v", provider, sets.List(validProviders))
		}
	}

	return nil
}

// validateCapabilities validates cluster capabilities.
func (cm *ClusterManager) validateCapabilities(capabilities map[string]string) error {
	validCapabilityValues := sets.New("true", "false", "limited")
	
	for capability, value := range capabilities {
		if !validCapabilityValues.Has(value) {
			return fmt.Errorf("invalid capability value for %q: %q, must be one of: %v", 
				capability, value, sets.List(validCapabilityValues))
		}
	}

	return nil
}

// PrepareClusterRegistration prepares a cluster registration with defaults and normalization.
func (cm *ClusterManager) PrepareClusterRegistration(cluster *ClusterRegistration) error {
	logger := klog.Background().WithValues("cluster", cluster.Name)
	logger.V(2).Info("preparing cluster registration")

	// Initialize empty maps if nil
	if cluster.Spec.Labels == nil {
		cluster.Spec.Labels = make(map[string]string)
	}
	if cluster.Spec.Capabilities == nil {
		cluster.Spec.Capabilities = make(map[string]string)
	}

	// Set default capabilities
	for capability, defaultValue := range cm.defaultCapabilities {
		if _, exists := cluster.Spec.Capabilities[capability]; !exists {
			cluster.Spec.Capabilities[capability] = defaultValue
		}
	}

	// Derive additional labels from location
	if cluster.Spec.Location != "" {
		locationParts := strings.Split(cluster.Spec.Location, "-")
		if len(locationParts) >= 2 {
			if cluster.Spec.Labels["region"] == "" {
				cluster.Spec.Labels["region"] = strings.Join(locationParts[:2], "-")
			}
			if cluster.Spec.Labels["zone"] == "" && len(locationParts) >= 3 {
				cluster.Spec.Labels["zone"] = locationParts[2]
			}
		}
	}

	// Add TMC-specific metadata labels
	cluster.Spec.Labels["tmc.kcp.io/managed"] = "true"
	cluster.Spec.Labels["tmc.kcp.io/registration-time"] = time.Now().Format(time.RFC3339)
	
	return nil
}

// GenerateSyncTargetSpec generates a SyncTarget specification for the cluster.
func (cm *ClusterManager) GenerateSyncTargetSpec(cluster *ClusterRegistration) (*SyncTargetSpec, error) {
	logger := klog.Background().WithValues("cluster", cluster.Name)
	logger.V(2).Info("generating SyncTarget specification")

	syncTarget := &SyncTargetSpec{
		Name:        fmt.Sprintf("synctarget-%s", cluster.Name),
		DisplayName: fmt.Sprintf("Sync Target for %s", cluster.Name),
		Location:    cluster.Spec.Location,
		Labels:      make(map[string]string),
		
		// Copy cluster capabilities to sync target
		Capabilities: make(map[string]string),
	}

	// Copy relevant labels to sync target
	for key, value := range cluster.Spec.Labels {
		if shouldCopyLabelToSyncTarget(key) {
			syncTarget.Labels[key] = value
		}
	}

	// Copy capabilities
	for capability, value := range cluster.Spec.Capabilities {
		syncTarget.Capabilities[capability] = value
	}

	// Add sync target specific labels
	syncTarget.Labels["synctarget.kcp.io/cluster"] = cluster.Name
	syncTarget.Labels["synctarget.kcp.io/location"] = cluster.Spec.Location

	return syncTarget, nil
}

// SyncTargetSpec defines the specification for a SyncTarget resource.
type SyncTargetSpec struct {
	Name         string            `json:"name"`
	DisplayName  string            `json:"displayName,omitempty"`
	Location     string            `json:"location"`
	Labels       map[string]string `json:"labels,omitempty"`
	Capabilities map[string]string `json:"capabilities,omitempty"`
}

// shouldCopyLabelToSyncTarget determines if a cluster label should be copied to the SyncTarget.
func shouldCopyLabelToSyncTarget(labelKey string) bool {
	// Copy specific labels to sync target
	copyLabels := sets.New(
		"region", "zone", "provider", "environment",
		"tmc.kcp.io/managed",
	)
	
	// Copy all labels with specific prefixes
	copyPrefixes := []string{
		"topology.kcp.io/",
		"workload.kcp.io/",
		"placement.kcp.io/",
	}
	
	if copyLabels.Has(labelKey) {
		return true
	}
	
	for _, prefix := range copyPrefixes {
		if strings.HasPrefix(labelKey, prefix) {
			return true
		}
	}
	
	return false
}

// GenerateClusterMetadata generates metadata for cluster registration tracking.
func (cm *ClusterManager) GenerateClusterMetadata(cluster *ClusterRegistration) ClusterMetadata {
	return ClusterMetadata{
		RegistrationID:   fmt.Sprintf("cluster-%s-%d", cluster.Name, time.Now().Unix()),
		Location:         cluster.Spec.Location,
		Capabilities:     cluster.Spec.Capabilities,
		Labels:           cluster.Spec.Labels,
		RegistrationTime: time.Now(),
	}
}

// ClusterMetadata contains metadata about cluster registration.
type ClusterMetadata struct {
	RegistrationID   string            `json:"registrationId"`
	Location         string            `json:"location"`
	Capabilities     map[string]string `json:"capabilities"`
	Labels           map[string]string `json:"labels"`
	RegistrationTime time.Time         `json:"registrationTime"`
}

// ClusterHealthChecker provides health checking capabilities for registered clusters.
type ClusterHealthChecker struct {
	// Timeout for health check operations
	healthCheckTimeout time.Duration
}

// NewClusterHealthChecker creates a new cluster health checker.
func NewClusterHealthChecker() *ClusterHealthChecker {
	return &ClusterHealthChecker{
		healthCheckTimeout: 30 * time.Second,
	}
}

// CheckClusterHealth performs comprehensive health checks on a cluster.
func (chc *ClusterHealthChecker) CheckClusterHealth(ctx context.Context, cluster *ClusterRegistration) (HealthStatus, error) {
	logger := klog.FromContext(ctx).WithValues("cluster", cluster.Name)
	logger.V(2).Info("performing cluster health check")

	// Create timeout context for health checks
	healthCtx, cancel := context.WithTimeout(ctx, chc.healthCheckTimeout)
	defer cancel()

	status := HealthStatus{
		ClusterName: cluster.Name,
		CheckTime:   time.Now(),
		Overall:     HealthStatusHealthy,
		Components:  make(map[string]ComponentHealth),
	}

	// Check cluster connectivity
	if err := chc.checkConnectivity(healthCtx, cluster); err != nil {
		status.Overall = HealthStatusUnhealthy
		status.Components["connectivity"] = ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: err.Error(),
		}
		logger.V(2).Info("cluster connectivity check failed", "error", err)
	} else {
		status.Components["connectivity"] = ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: "Connectivity check passed",
		}
	}

	// Check sync target status
	if err := chc.checkSyncTarget(healthCtx, cluster); err != nil {
		status.Overall = HealthStatusDegraded
		status.Components["synctarget"] = ComponentHealth{
			Status:  HealthStatusDegraded,
			Message: err.Error(),
		}
		logger.V(2).Info("sync target check degraded", "error", err)
	} else {
		status.Components["synctarget"] = ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: "SyncTarget is healthy",
		}
	}

	// Check resource availability
	if err := chc.checkResourceAvailability(healthCtx, cluster); err != nil {
		// Resource issues don't make cluster unhealthy, just degraded
		status.Components["resources"] = ComponentHealth{
			Status:  HealthStatusDegraded,
			Message: err.Error(),
		}
		if status.Overall == HealthStatusHealthy {
			status.Overall = HealthStatusDegraded
		}
		logger.V(2).Info("cluster resource check degraded", "error", err)
	} else {
		status.Components["resources"] = ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: "Resources are available",
		}
	}

	logger.V(2).Info("cluster health check completed", "overall", status.Overall)
	return status, nil
}

// HealthStatus represents the health status of a cluster.
type HealthStatus struct {
	ClusterName string                     `json:"clusterName"`
	CheckTime   time.Time                  `json:"checkTime"`
	Overall     HealthStatusValue          `json:"overall"`
	Components  map[string]ComponentHealth `json:"components"`
}

// ComponentHealth represents the health of a cluster component.
type ComponentHealth struct {
	Status  HealthStatusValue `json:"status"`
	Message string            `json:"message"`
}

// HealthStatusValue represents possible health status values.
type HealthStatusValue string

const (
	// HealthStatusHealthy indicates the cluster is healthy.
	HealthStatusHealthy HealthStatusValue = "Healthy"
	
	// HealthStatusDegraded indicates the cluster is degraded but operational.
	HealthStatusDegraded HealthStatusValue = "Degraded"
	
	// HealthStatusUnhealthy indicates the cluster is unhealthy.
	HealthStatusUnhealthy HealthStatusValue = "Unhealthy"
)

// checkConnectivity checks cluster connectivity.
func (chc *ClusterHealthChecker) checkConnectivity(ctx context.Context, cluster *ClusterRegistration) error {
	// Placeholder for connectivity check
	// In practice, this would check if the cluster API server is reachable
	return nil
}

// checkSyncTarget checks the status of the associated SyncTarget.
func (chc *ClusterHealthChecker) checkSyncTarget(ctx context.Context, cluster *ClusterRegistration) error {
	// Placeholder for sync target health check
	// In practice, this would verify the SyncTarget resource is healthy
	return nil
}

// checkResourceAvailability checks cluster resource availability.
func (chc *ClusterHealthChecker) checkResourceAvailability(ctx context.Context, cluster *ClusterRegistration) error {
	// Placeholder for resource availability check
	// In practice, this would check CPU, memory, storage availability
	return nil
}

// ClusterCapabilityDiscovery provides capability discovery for clusters.
type ClusterCapabilityDiscovery struct{}

// NewClusterCapabilityDiscovery creates a new cluster capability discovery service.
func NewClusterCapabilityDiscovery() *ClusterCapabilityDiscovery {
	return &ClusterCapabilityDiscovery{}
}

// DiscoverCapabilities discovers and updates cluster capabilities.
func (ccd *ClusterCapabilityDiscovery) DiscoverCapabilities(ctx context.Context, cluster *ClusterRegistration) (map[string]string, error) {
	logger := klog.FromContext(ctx).WithValues("cluster", cluster.Name)
	logger.V(2).Info("discovering cluster capabilities")

	capabilities := make(map[string]string)

	// Copy existing capabilities
	for k, v := range cluster.Spec.Capabilities {
		capabilities[k] = v
	}

	// Discover additional capabilities based on cluster properties
	if err := ccd.discoverComputeCapabilities(ctx, cluster, capabilities); err != nil {
		logger.Error(err, "failed to discover compute capabilities")
	}

	if err := ccd.discoverStorageCapabilities(ctx, cluster, capabilities); err != nil {
		logger.Error(err, "failed to discover storage capabilities")
	}

	if err := ccd.discoverNetworkingCapabilities(ctx, cluster, capabilities); err != nil {
		logger.Error(err, "failed to discover networking capabilities")
	}

	logger.V(2).Info("capability discovery completed", "capabilities", capabilities)
	return capabilities, nil
}

// discoverComputeCapabilities discovers compute-related capabilities.
func (ccd *ClusterCapabilityDiscovery) discoverComputeCapabilities(ctx context.Context, cluster *ClusterRegistration, capabilities map[string]string) error {
	// Placeholder for compute capability discovery
	capabilities["gpu"] = "false" // Default assumption
	capabilities["spot-instances"] = "true" // Default assumption
	return nil
}

// discoverStorageCapabilities discovers storage-related capabilities.
func (ccd *ClusterCapabilityDiscovery) discoverStorageCapabilities(ctx context.Context, cluster *ClusterRegistration, capabilities map[string]string) error {
	// Placeholder for storage capability discovery
	capabilities["persistent-volumes"] = "true"
	capabilities["block-storage"] = "true"
	return nil
}

// discoverNetworkingCapabilities discovers networking-related capabilities.
func (ccd *ClusterCapabilityDiscovery) discoverNetworkingCapabilities(ctx context.Context, cluster *ClusterRegistration, capabilities map[string]string) error {
	// Placeholder for networking capability discovery
	capabilities["load-balancer"] = "true"
	capabilities["ingress"] = "true"
	return nil
}