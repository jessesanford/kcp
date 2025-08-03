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

package tmc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "Healthy"
	HealthStatusDegraded  HealthStatus = "Degraded"
	HealthStatusUnhealthy HealthStatus = "Unhealthy"
	HealthStatusUnknown   HealthStatus = "Unknown"
)

// ComponentType represents different TMC components
type ComponentType string

const (
	ComponentTypePlacementController     ComponentType = "PlacementController"
	ComponentTypeSyncTargetController    ComponentType = "SyncTargetController"
	ComponentTypeMigrationEngine         ComponentType = "MigrationEngine"
	ComponentTypeStrategyRegistry        ComponentType = "StrategyRegistry"
	ComponentTypeClusterHealthTracker    ComponentType = "ClusterHealthTracker"
	ComponentTypeRolloutCoordinator      ComponentType = "RolloutCoordinator"
	ComponentTypeVirtualWorkspaceManager ComponentType = "VirtualWorkspaceManager"
	ComponentTypeResourceAggregator      ComponentType = "CrossClusterResourceAggregator"
	ComponentTypeProjectionController    ComponentType = "WorkloadProjectionController"
	ComponentTypeRecoveryManager         ComponentType = "RecoveryManager"
)

// HealthCheck represents a health check for a component
type HealthCheck struct {
	ComponentType ComponentType
	ComponentID   string
	Status        HealthStatus
	Message       string
	Details       map[string]interface{}
	Timestamp     time.Time
	Duration      time.Duration
	Error         error
}

// HealthMonitor monitors the health of TMC components
type HealthMonitor struct {
	healthChecks    map[string]*HealthCheck
	healthProviders map[ComponentType]HealthProvider
	mu              sync.RWMutex

	// Configuration
	checkInterval      time.Duration
	healthTimeout      time.Duration
	degradedThreshold  time.Duration
	unhealthyThreshold time.Duration

	// State
	running bool
	stopCh  chan struct{}

	// Metrics
	totalChecks     int64
	healthyChecks   int64
	degradedChecks  int64
	unhealthyChecks int64
	errorChecks     int64
}

// HealthProvider interface for components to provide health information
type HealthProvider interface {
	// GetHealth returns the current health status of the component
	GetHealth(ctx context.Context) *HealthCheck

	// GetComponentID returns a unique identifier for this component instance
	GetComponentID() string

	// GetComponentType returns the type of this component
	GetComponentType() ComponentType
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{
		healthChecks:       make(map[string]*HealthCheck),
		healthProviders:    make(map[ComponentType]HealthProvider),
		checkInterval:      30 * time.Second,
		healthTimeout:      10 * time.Second,
		degradedThreshold:  2 * time.Minute,
		unhealthyThreshold: 5 * time.Minute,
		stopCh:             make(chan struct{}),
	}
}

// Start starts the health monitoring
func (hm *HealthMonitor) Start(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "HealthMonitor")
	logger.Info("Starting health monitor")
	defer logger.Info("Shutting down health monitor")

	hm.running = true
	defer func() { hm.running = false }()

	// Start periodic health checks
	go wait.UntilWithContext(ctx, hm.performHealthChecks, hm.checkInterval)

	<-ctx.Done()
}

// Stop stops the health monitoring
func (hm *HealthMonitor) Stop() {
	if hm.running {
		close(hm.stopCh)
	}
}

// RegisterHealthProvider registers a health provider for a component type
func (hm *HealthMonitor) RegisterHealthProvider(provider HealthProvider) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.healthProviders[provider.GetComponentType()] = provider

	klog.V(2).Info("Registered health provider",
		"componentType", provider.GetComponentType(),
		"componentID", provider.GetComponentID())
}

// UnregisterHealthProvider unregisters a health provider
func (hm *HealthMonitor) UnregisterHealthProvider(componentType ComponentType) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	delete(hm.healthProviders, componentType)

	// Clean up health checks for this component type
	for key, check := range hm.healthChecks {
		if check.ComponentType == componentType {
			delete(hm.healthChecks, key)
		}
	}
}

// GetComponentHealth returns the health status of a specific component
func (hm *HealthMonitor) GetComponentHealth(componentType ComponentType, componentID string) (*HealthCheck, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", componentType, componentID)
	check, exists := hm.healthChecks[key]
	return check, exists
}

// GetOverallHealth returns the overall health status of the TMC system
func (hm *HealthMonitor) GetOverallHealth() *HealthCheck {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if len(hm.healthChecks) == 0 {
		return &HealthCheck{
			ComponentType: "TMCSystem",
			ComponentID:   "overall",
			Status:        HealthStatusUnknown,
			Message:       "No components registered",
			Timestamp:     time.Now(),
		}
	}

	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0
	unknownCount := 0

	for _, check := range hm.healthChecks {
		switch check.Status {
		case HealthStatusHealthy:
			healthyCount++
		case HealthStatusDegraded:
			degradedCount++
		case HealthStatusUnhealthy:
			unhealthyCount++
		case HealthStatusUnknown:
			unknownCount++
		}
	}

	totalComponents := len(hm.healthChecks)
	overallStatus := HealthStatusHealthy
	message := fmt.Sprintf("All %d components healthy", totalComponents)

	if unhealthyCount > 0 {
		overallStatus = HealthStatusUnhealthy
		message = fmt.Sprintf("%d/%d components unhealthy", unhealthyCount, totalComponents)
	} else if degradedCount > 0 {
		overallStatus = HealthStatusDegraded
		message = fmt.Sprintf("%d/%d components degraded", degradedCount, totalComponents)
	} else if unknownCount > 0 {
		overallStatus = HealthStatusUnknown
		message = fmt.Sprintf("%d/%d components status unknown", unknownCount, totalComponents)
	}

	return &HealthCheck{
		ComponentType: "TMCSystem",
		ComponentID:   "overall",
		Status:        overallStatus,
		Message:       message,
		Details: map[string]interface{}{
			"totalComponents":     totalComponents,
			"healthyComponents":   healthyCount,
			"degradedComponents":  degradedCount,
			"unhealthyComponents": unhealthyCount,
			"unknownComponents":   unknownCount,
		},
		Timestamp: time.Now(),
	}
}

// GetAllComponentHealth returns health status of all components
func (hm *HealthMonitor) GetAllComponentHealth() map[string]*HealthCheck {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make(map[string]*HealthCheck)
	for key, check := range hm.healthChecks {
		result[key] = check
	}
	return result
}

func (hm *HealthMonitor) performHealthChecks(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "HealthChecker")

	hm.mu.RLock()
	providers := make([]HealthProvider, 0, len(hm.healthProviders))
	for _, provider := range hm.healthProviders {
		providers = append(providers, provider)
	}
	hm.mu.RUnlock()

	for _, provider := range providers {
		hm.performComponentHealthCheck(ctx, provider)
	}

	hm.totalChecks++
	logger.V(4).Info("Completed health check cycle", "componentsChecked", len(providers))
}

func (hm *HealthMonitor) performComponentHealthCheck(ctx context.Context, provider HealthProvider) {
	logger := klog.FromContext(ctx).WithValues(
		"componentType", provider.GetComponentType(),
		"componentID", provider.GetComponentID(),
	)

	// Create timeout context for health check
	checkCtx, cancel := context.WithTimeout(ctx, hm.healthTimeout)
	defer cancel()

	startTime := time.Now()
	healthCheck := provider.GetHealth(checkCtx)
	duration := time.Since(startTime)

	if healthCheck == nil {
		healthCheck = &HealthCheck{
			ComponentType: provider.GetComponentType(),
			ComponentID:   provider.GetComponentID(),
			Status:        HealthStatusUnknown,
			Message:       "Health check returned nil",
			Timestamp:     time.Now(),
		}
	}

	healthCheck.Duration = duration
	healthCheck.Timestamp = time.Now()

	// Determine status based on response time and errors
	if healthCheck.Error != nil {
		healthCheck.Status = HealthStatusUnhealthy
		if healthCheck.Message == "" {
			healthCheck.Message = healthCheck.Error.Error()
		}
		hm.errorChecks++
	} else if duration > hm.unhealthyThreshold {
		healthCheck.Status = HealthStatusUnhealthy
		healthCheck.Message = fmt.Sprintf("Health check took too long: %v", duration)
		hm.unhealthyChecks++
	} else if duration > hm.degradedThreshold {
		if healthCheck.Status == HealthStatusHealthy {
			healthCheck.Status = HealthStatusDegraded
			healthCheck.Message = fmt.Sprintf("Health check slow: %v", duration)
		}
		hm.degradedChecks++
	} else {
		hm.healthyChecks++
	}

	// Store the health check result
	key := fmt.Sprintf("%s:%s", healthCheck.ComponentType, healthCheck.ComponentID)
	hm.mu.Lock()
	hm.healthChecks[key] = healthCheck
	hm.mu.Unlock()

	logger.V(3).Info("Component health check completed",
		"status", healthCheck.Status,
		"duration", duration,
		"message", healthCheck.Message)
}

// HealthMetrics returns health monitoring metrics
func (hm *HealthMonitor) HealthMetrics() map[string]interface{} {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	return map[string]interface{}{
		"totalChecks":          hm.totalChecks,
		"healthyChecks":        hm.healthyChecks,
		"degradedChecks":       hm.degradedChecks,
		"unhealthyChecks":      hm.unhealthyChecks,
		"errorChecks":          hm.errorChecks,
		"registeredComponents": len(hm.healthProviders),
		"activeComponents":     len(hm.healthChecks),
	}
}

// Default health provider implementations

// BaseHealthProvider provides a base implementation for health providers
type BaseHealthProvider struct {
	componentType ComponentType
	componentID   string
	healthFunc    func(ctx context.Context) *HealthCheck
}

// NewBaseHealthProvider creates a new base health provider
func NewBaseHealthProvider(componentType ComponentType, componentID string, healthFunc func(ctx context.Context) *HealthCheck) *BaseHealthProvider {
	return &BaseHealthProvider{
		componentType: componentType,
		componentID:   componentID,
		healthFunc:    healthFunc,
	}
}

func (bhp *BaseHealthProvider) GetHealth(ctx context.Context) *HealthCheck {
	if bhp.healthFunc != nil {
		return bhp.healthFunc(ctx)
	}

	return &HealthCheck{
		ComponentType: bhp.componentType,
		ComponentID:   bhp.componentID,
		Status:        HealthStatusHealthy,
		Message:       "Component operational",
		Details:       make(map[string]interface{}),
		Timestamp:     time.Now(),
	}
}

func (bhp *BaseHealthProvider) GetComponentID() string {
	return bhp.componentID
}

func (bhp *BaseHealthProvider) GetComponentType() ComponentType {
	return bhp.componentType
}

// ClusterHealthProvider provides health information for cluster components
type ClusterHealthProvider struct {
	*BaseHealthProvider
	clusterName    string
	logicalCluster logicalcluster.Name
	lastActivity   time.Time
	errorCount     int64
	successCount   int64
}

// NewClusterHealthProvider creates a health provider for cluster components
func NewClusterHealthProvider(componentType ComponentType, clusterName string, logicalCluster logicalcluster.Name) *ClusterHealthProvider {
	componentID := fmt.Sprintf("%s-%s", clusterName, logicalCluster)

	chp := &ClusterHealthProvider{
		clusterName:    clusterName,
		logicalCluster: logicalCluster,
		lastActivity:   time.Now(),
	}

	chp.BaseHealthProvider = NewBaseHealthProvider(componentType, componentID, chp.getClusterHealth)
	return chp
}

func (chp *ClusterHealthProvider) getClusterHealth(ctx context.Context) *HealthCheck {
	status := HealthStatusHealthy
	message := "Cluster component operational"

	// Check activity recency
	timeSinceActivity := time.Since(chp.lastActivity)
	if timeSinceActivity > 5*time.Minute {
		status = HealthStatusDegraded
		message = fmt.Sprintf("No activity for %v", timeSinceActivity)
	}

	// Check error rate
	if chp.errorCount > 0 && chp.successCount > 0 {
		errorRate := float64(chp.errorCount) / float64(chp.errorCount+chp.successCount)
		if errorRate > 0.5 {
			status = HealthStatusUnhealthy
			message = fmt.Sprintf("High error rate: %.2f%%", errorRate*100)
		} else if errorRate > 0.1 {
			status = HealthStatusDegraded
			message = fmt.Sprintf("Elevated error rate: %.2f%%", errorRate*100)
		}
	}

	return &HealthCheck{
		ComponentType: chp.componentType,
		ComponentID:   chp.componentID,
		Status:        status,
		Message:       message,
		Details: map[string]interface{}{
			"clusterName":       chp.clusterName,
			"logicalCluster":    chp.logicalCluster.String(),
			"lastActivity":      chp.lastActivity,
			"timeSinceActivity": timeSinceActivity.String(),
			"errorCount":        chp.errorCount,
			"successCount":      chp.successCount,
		},
		Timestamp: time.Now(),
	}
}

// RecordActivity records successful activity
func (chp *ClusterHealthProvider) RecordActivity() {
	chp.lastActivity = time.Now()
	chp.successCount++
}

// RecordError records an error
func (chp *ClusterHealthProvider) RecordError() {
	chp.errorCount++
}

// SystemHealthProvider provides system-wide health information
type SystemHealthProvider struct {
	*BaseHealthProvider
	metrics map[string]interface{}
	mu      sync.RWMutex
}

// NewSystemHealthProvider creates a system health provider
func NewSystemHealthProvider(componentType ComponentType, componentID string) *SystemHealthProvider {
	shp := &SystemHealthProvider{
		metrics: make(map[string]interface{}),
	}

	shp.BaseHealthProvider = NewBaseHealthProvider(componentType, componentID, shp.getSystemHealth)
	return shp
}

func (shp *SystemHealthProvider) getSystemHealth(ctx context.Context) *HealthCheck {
	shp.mu.RLock()
	defer shp.mu.RUnlock()

	status := HealthStatusHealthy
	message := "System component operational"

	// Add system-specific health logic here
	// For example: check memory usage, goroutine count, etc.

	return &HealthCheck{
		ComponentType: shp.componentType,
		ComponentID:   shp.componentID,
		Status:        status,
		Message:       message,
		Details:       shp.metrics,
		Timestamp:     time.Now(),
	}
}

// UpdateMetrics updates the system metrics
func (shp *SystemHealthProvider) UpdateMetrics(metrics map[string]interface{}) {
	shp.mu.Lock()
	defer shp.mu.Unlock()

	for key, value := range metrics {
		shp.metrics[key] = value
	}
}

// HealthAggregator aggregates health information across multiple sources
type HealthAggregator struct {
	healthMonitor *HealthMonitor
	mu            sync.RWMutex
}

// NewHealthAggregator creates a new health aggregator
func NewHealthAggregator(healthMonitor *HealthMonitor) *HealthAggregator {
	return &HealthAggregator{
		healthMonitor: healthMonitor,
	}
}

// GetClusterHealth returns aggregated health for all components in a cluster
func (ha *HealthAggregator) GetClusterHealth(clusterName string) *HealthCheck {
	ha.mu.RLock()
	defer ha.mu.RUnlock()

	allHealth := ha.healthMonitor.GetAllComponentHealth()
	clusterComponents := make([]*HealthCheck, 0)

	for _, check := range allHealth {
		if check.Details != nil {
			if checkCluster, exists := check.Details["clusterName"]; exists {
				if checkCluster == clusterName {
					clusterComponents = append(clusterComponents, check)
				}
			}
		}
	}

	if len(clusterComponents) == 0 {
		return &HealthCheck{
			ComponentType: "ClusterAggregate",
			ComponentID:   clusterName,
			Status:        HealthStatusUnknown,
			Message:       "No components found for cluster",
			Timestamp:     time.Now(),
		}
	}

	// Aggregate status
	overallStatus := HealthStatusHealthy
	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0

	for _, check := range clusterComponents {
		switch check.Status {
		case HealthStatusHealthy:
			healthyCount++
		case HealthStatusDegraded:
			degradedCount++
			if overallStatus == HealthStatusHealthy {
				overallStatus = HealthStatusDegraded
			}
		case HealthStatusUnhealthy:
			unhealthyCount++
			overallStatus = HealthStatusUnhealthy
		}
	}

	message := fmt.Sprintf("Cluster health: %d healthy, %d degraded, %d unhealthy",
		healthyCount, degradedCount, unhealthyCount)

	return &HealthCheck{
		ComponentType: "ClusterAggregate",
		ComponentID:   clusterName,
		Status:        overallStatus,
		Message:       message,
		Details: map[string]interface{}{
			"totalComponents":     len(clusterComponents),
			"healthyComponents":   healthyCount,
			"degradedComponents":  degradedCount,
			"unhealthyComponents": unhealthyCount,
		},
		Timestamp: time.Now(),
	}
}
