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

package syncer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// HealthMonitor monitors the health of syncer components and integrates with TMC health system
type HealthMonitor struct {
	// Configuration
	syncTargetName string
	checkInterval  time.Duration
	
	// References
	engine    *Engine
	tmcHealth *tmc.HealthMonitor
	
	// Health providers
	healthProvider *SyncerHealthProvider
	
	// State
	started bool
	stopCh  chan struct{}
	mu      sync.RWMutex
}

// HealthMonitorOptions configures the health monitor
type HealthMonitorOptions struct {
	SyncTargetName string
	Engine         *Engine
	TMCHealth      *tmc.HealthMonitor
	CheckInterval  time.Duration
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(options HealthMonitorOptions) (*HealthMonitor, error) {
	logger := klog.Background().WithValues(
		"component", "HealthMonitor",
		"syncTarget", options.SyncTargetName,
	)
	logger.Info("Creating health monitor")

	// Set default check interval
	checkInterval := options.CheckInterval
	if checkInterval == 0 {
		checkInterval = 30 * time.Second
	}

	hm := &HealthMonitor{
		syncTargetName: options.SyncTargetName,
		checkInterval:  checkInterval,
		engine:         options.Engine,
		tmcHealth:      options.TMCHealth,
		stopCh:         make(chan struct{}),
	}

	// Create health provider for this syncer
	healthProvider := NewSyncerHealthProvider(options.SyncTargetName, options.Engine)
	hm.healthProvider = healthProvider

	logger.Info("Successfully created health monitor")
	return hm, nil
}

// Start starts the health monitor
func (hm *HealthMonitor) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues(
		"component", "HealthMonitor",
		"syncTarget", hm.syncTargetName,
	)
	logger.Info("Starting health monitor")

	hm.mu.Lock()
	if hm.started {
		hm.mu.Unlock()
		return fmt.Errorf("health monitor already started")
	}
	hm.started = true
	hm.mu.Unlock()

	// Register with TMC health monitor
	if hm.tmcHealth != nil {
		hm.tmcHealth.RegisterHealthProvider(hm.healthProvider)
		logger.Info("Registered with TMC health monitor")
	}

	logger.Info("Health monitor started successfully")
	return nil
}

// Stop stops the health monitor
func (hm *HealthMonitor) Stop() {
	logger := klog.Background().WithValues(
		"component", "HealthMonitor",
		"syncTarget", hm.syncTargetName,
	)
	logger.Info("Stopping health monitor")

	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.started {
		return
	}

	// Unregister from TMC health monitor
	if hm.tmcHealth != nil {
		hm.tmcHealth.UnregisterHealthProvider(tmc.ComponentTypeSyncTargetController)
		logger.Info("Unregistered from TMC health monitor")
	}

	close(hm.stopCh)
	hm.started = false

	logger.Info("Health monitor stopped")
}

// GetHealth returns the current health status of the syncer
func (hm *HealthMonitor) GetHealth(ctx context.Context) *tmc.HealthCheck {
	return hm.healthProvider.GetHealth(ctx)
}

// SyncerHealthProvider implements TMC HealthProvider for the syncer
type SyncerHealthProvider struct {
	syncTargetName string
	engine         *Engine
	componentID    string
}

// NewSyncerHealthProvider creates a new syncer health provider
func NewSyncerHealthProvider(syncTargetName string, engine *Engine) *SyncerHealthProvider {
	return &SyncerHealthProvider{
		syncTargetName: syncTargetName,
		engine:         engine,
		componentID:    fmt.Sprintf("syncer-%s", syncTargetName),
	}
}

// GetHealth returns the current health status of the syncer
func (shp *SyncerHealthProvider) GetHealth(ctx context.Context) *tmc.HealthCheck {
	logger := klog.FromContext(ctx).WithValues(
		"component", "SyncerHealthProvider",
		"syncTarget", shp.syncTargetName,
	)

	healthCheck := &tmc.HealthCheck{
		ComponentType: tmc.ComponentTypeSyncTargetController,
		ComponentID:   shp.componentID,
		Timestamp:     time.Now(),
		Details:       make(map[string]interface{}),
	}

	if shp.engine == nil {
		healthCheck.Status = tmc.HealthStatusUnhealthy
		healthCheck.Message = "Engine not initialized"
		healthCheck.Error = fmt.Errorf("syncer engine is nil")
		return healthCheck
	}

	// Get engine status
	engineStatus := shp.engine.GetStatus()
	if engineStatus == nil {
		healthCheck.Status = tmc.HealthStatusUnhealthy
		healthCheck.Message = "Unable to get engine status"
		healthCheck.Error = fmt.Errorf("engine status is nil")
		return healthCheck
	}

	// Add engine details
	healthCheck.Details["engineStarted"] = engineStatus.Started
	healthCheck.Details["syncTargetName"] = engineStatus.SyncTargetName
	healthCheck.Details["workspaceCluster"] = engineStatus.WorkspaceCluster.String()
	healthCheck.Details["resourceControllers"] = engineStatus.ResourceControllers
	healthCheck.Details["syncCount"] = engineStatus.SyncCount
	healthCheck.Details["errorCount"] = engineStatus.ErrorCount
	healthCheck.Details["lastSyncTime"] = engineStatus.LastSyncTime
	healthCheck.Details["uptime"] = engineStatus.Uptime.String()

	// Determine health status
	if !engineStatus.Started {
		healthCheck.Status = tmc.HealthStatusUnhealthy
		healthCheck.Message = "Engine not started"
		return healthCheck
	}

	// Check error rate
	if engineStatus.SyncCount > 0 {
		errorRate := float64(engineStatus.ErrorCount) / float64(engineStatus.SyncCount)
		healthCheck.Details["errorRate"] = errorRate

		if errorRate > 0.5 {
			healthCheck.Status = tmc.HealthStatusUnhealthy
			healthCheck.Message = fmt.Sprintf("High error rate: %.2f%%", errorRate*100)
			return healthCheck
		} else if errorRate > 0.1 {
			healthCheck.Status = tmc.HealthStatusDegraded
			healthCheck.Message = fmt.Sprintf("Elevated error rate: %.2f%%", errorRate*100)
			return healthCheck
		}
	}

	// Check if we have any active resource controllers
	if engineStatus.ResourceControllers == 0 {
		healthCheck.Status = tmc.HealthStatusDegraded
		healthCheck.Message = "No active resource controllers"
		return healthCheck
	}

	// Check last sync time
	timeSinceLastSync := time.Since(engineStatus.LastSyncTime)
	healthCheck.Details["timeSinceLastSync"] = timeSinceLastSync.String()

	if timeSinceLastSync > 10*time.Minute {
		healthCheck.Status = tmc.HealthStatusDegraded
		healthCheck.Message = fmt.Sprintf("No sync activity for %v", timeSinceLastSync)
		return healthCheck
	}

	// Get status reporter health if available
	if shp.engine.statusReporter != nil {
		statusReporterStatus := shp.engine.statusReporter.GetStatus()
		healthCheck.Details["statusReporter"] = map[string]interface{}{
			"started":           statusReporterStatus.Started,
			"heartbeatCount":    statusReporterStatus.HeartbeatCount,
			"errorCount":        statusReporterStatus.ErrorCount,
			"lastHeartbeat":     statusReporterStatus.LastHeartbeat,
			"connectionHealthy": statusReporterStatus.ConnectionHealthy,
		}

		if !statusReporterStatus.ConnectionHealthy {
			healthCheck.Status = tmc.HealthStatusDegraded
			healthCheck.Message = "Status reporter connection unhealthy"
			return healthCheck
		}

		// Check heartbeat recency
		timeSinceHeartbeat := time.Since(statusReporterStatus.LastHeartbeat)
		if timeSinceHeartbeat > 2*statusReporterStatus.HeartbeatPeriod {
			healthCheck.Status = tmc.HealthStatusDegraded
			healthCheck.Message = fmt.Sprintf("No heartbeat for %v", timeSinceHeartbeat)
			return healthCheck
		}
	}

	// All checks passed
	healthCheck.Status = tmc.HealthStatusHealthy
	healthCheck.Message = fmt.Sprintf("Syncer operational with %d controllers", engineStatus.ResourceControllers)

	logger.V(4).Info("Health check completed", "status", healthCheck.Status, "message", healthCheck.Message)
	return healthCheck
}

// GetComponentID returns the component ID
func (shp *SyncerHealthProvider) GetComponentID() string {
	return shp.componentID
}

// GetComponentType returns the component type
func (shp *SyncerHealthProvider) GetComponentType() tmc.ComponentType {
	return tmc.ComponentTypeSyncTargetController
}

// HealthStatus represents the overall health status of the syncer
type HealthStatus struct {
	Status               tmc.HealthStatus
	Message              string
	EngineStarted        bool
	ResourceControllers  int
	SyncCount            int64
	ErrorCount           int64
	ErrorRate            float64
	LastSyncTime         time.Time
	TimeSinceLastSync    time.Duration
	StatusReporterHealthy bool
	LastHeartbeat        time.Time
	TimeSinceHeartbeat   time.Duration
	Uptime               time.Duration
}

// GetOverallHealth returns a simplified health status
func (hm *HealthMonitor) GetOverallHealth(ctx context.Context) *HealthStatus {
	healthCheck := hm.GetHealth(ctx)
	
	status := &HealthStatus{
		Status:  healthCheck.Status,
		Message: healthCheck.Message,
	}

	// Extract details from health check
	if details := healthCheck.Details; details != nil {
		if val, ok := details["engineStarted"].(bool); ok {
			status.EngineStarted = val
		}
		if val, ok := details["resourceControllers"].(int); ok {
			status.ResourceControllers = val
		}
		if val, ok := details["syncCount"].(int64); ok {
			status.SyncCount = val
		}
		if val, ok := details["errorCount"].(int64); ok {
			status.ErrorCount = val
		}
		if val, ok := details["errorRate"].(float64); ok {
			status.ErrorRate = val
		}
		if val, ok := details["lastSyncTime"].(time.Time); ok {
			status.LastSyncTime = val
		}
		if val, ok := details["timeSinceLastSync"].(time.Duration); ok {
			status.TimeSinceLastSync = val
		}
		
		// Status reporter details
		if statusReporter, ok := details["statusReporter"].(map[string]interface{}); ok {
			if val, ok := statusReporter["connectionHealthy"].(bool); ok {
				status.StatusReporterHealthy = val
			}
			if val, ok := statusReporter["lastHeartbeat"].(time.Time); ok {
				status.LastHeartbeat = val
				status.TimeSinceHeartbeat = time.Since(val)
			}
		}
	}

	return status
}