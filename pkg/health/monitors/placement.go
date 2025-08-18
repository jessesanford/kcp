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

package monitors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kcp-dev/kcp/pkg/health"
)

// PlacementMetrics defines the interface for collecting placement engine metrics.
type PlacementMetrics interface {
	// GetSchedulerAvailability returns true if the placement scheduler is available.
	GetSchedulerAvailability() bool
	
	// GetPolicyEngineStatus returns true if the policy engine is running.
	GetPolicyEngineStatus() bool
	
	// GetPendingPlacements returns the number of pending placement decisions.
	GetPendingPlacements() int
	
	// GetPlacementLatency returns the average placement decision latency.
	GetPlacementLatency() time.Duration
	
	// GetPlacementSuccessRate returns the success rate of placement decisions (0.0-1.0).
	GetPlacementSuccessRate() float64
	
	// GetLastPlacementTime returns the timestamp of the last placement decision.
	GetLastPlacementTime() time.Time
	
	// GetRegisteredClusters returns the number of clusters available for placement.
	GetRegisteredClusters() int
	
	// GetHealthyClusters returns the number of healthy clusters available for placement.
	GetHealthyClusters() int
}

// PlacementHealthMonitor monitors the health of the TMC placement engine.
type PlacementHealthMonitor struct {
	name       string
	metrics    PlacementMetrics
	thresholds PlacementHealthThresholds
	lastCheck  time.Time
	mutex      sync.RWMutex
}

// PlacementHealthThresholds defines the thresholds for determining placement engine health.
type PlacementHealthThresholds struct {
	// MaxPendingPlacements is the maximum acceptable number of pending placements.
	MaxPendingPlacements int `json:"max_pending_placements"`
	
	// MaxPlacementLatency is the maximum acceptable placement decision latency.
	MaxPlacementLatency time.Duration `json:"max_placement_latency"`
	
	// MinSuccessRate is the minimum acceptable placement success rate.
	MinSuccessRate float64 `json:"min_success_rate"`
	
	// MaxPlacementAge is the maximum acceptable time since last placement.
	MaxPlacementAge time.Duration `json:"max_placement_age"`
	
	// MinHealthyClusterRatio is the minimum ratio of healthy to registered clusters.
	MinHealthyClusterRatio float64 `json:"min_healthy_cluster_ratio"`
}

// DefaultPlacementHealthThresholds returns default health thresholds for placement monitoring.
func DefaultPlacementHealthThresholds() PlacementHealthThresholds {
	return PlacementHealthThresholds{
		MaxPendingPlacements:   100,
		MaxPlacementLatency:    2 * time.Second,
		MinSuccessRate:         0.95, // 95% success rate
		MaxPlacementAge:        15 * time.Minute,
		MinHealthyClusterRatio: 0.8, // 80% of clusters should be healthy
	}
}

// NewPlacementHealthMonitor creates a new placement engine health monitor.
func NewPlacementHealthMonitor(name string, metrics PlacementMetrics, thresholds PlacementHealthThresholds) health.HealthChecker {
	monitor := &PlacementHealthMonitor{
		name:       name,
		metrics:    metrics,
		thresholds: thresholds,
	}
	
	return health.NewBaseHealthChecker(fmt.Sprintf("placement-%s", name), monitor.checkHealth)
}

// checkHealth performs the actual health check for the placement engine.
func (p *PlacementHealthMonitor) checkHealth(ctx context.Context) health.HealthStatus {
	p.mutex.Lock()
	p.lastCheck = time.Now()
	p.mutex.Unlock()
	
	var issues []string
	details := make(map[string]interface{})
	
	// Check scheduler availability
	schedulerAvailable := p.metrics.GetSchedulerAvailability()
	details["scheduler_available"] = schedulerAvailable
	if !schedulerAvailable {
		issues = append(issues, "placement scheduler is not available")
	}
	
	// Check policy engine status
	policyEngineRunning := p.metrics.GetPolicyEngineStatus()
	details["policy_engine_running"] = policyEngineRunning
	if !policyEngineRunning {
		issues = append(issues, "policy engine is not running")
	}
	
	// Check pending placements
	pendingPlacements := p.metrics.GetPendingPlacements()
	details["pending_placements"] = pendingPlacements
	details["max_pending_placements"] = p.thresholds.MaxPendingPlacements
	if pendingPlacements > p.thresholds.MaxPendingPlacements {
		issues = append(issues, fmt.Sprintf("too many pending placements: %d > %d", 
			pendingPlacements, p.thresholds.MaxPendingPlacements))
	}
	
	// Check placement latency
	placementLatency := p.metrics.GetPlacementLatency()
	details["placement_latency_ms"] = placementLatency.Milliseconds()
	details["max_placement_latency_ms"] = p.thresholds.MaxPlacementLatency.Milliseconds()
	if placementLatency > p.thresholds.MaxPlacementLatency {
		issues = append(issues, fmt.Sprintf("placement latency too high: %v > %v", 
			placementLatency, p.thresholds.MaxPlacementLatency))
	}
	
	// Check placement success rate
	successRate := p.metrics.GetPlacementSuccessRate()
	details["success_rate"] = successRate
	details["min_success_rate"] = p.thresholds.MinSuccessRate
	if successRate < p.thresholds.MinSuccessRate {
		issues = append(issues, fmt.Sprintf("placement success rate too low: %.2f < %.2f", 
			successRate, p.thresholds.MinSuccessRate))
	}
	
	// Check last placement time
	lastPlacement := p.metrics.GetLastPlacementTime()
	placementAge := time.Since(lastPlacement)
	details["last_placement_time"] = lastPlacement
	details["placement_age_seconds"] = placementAge.Seconds()
	details["max_placement_age_seconds"] = p.thresholds.MaxPlacementAge.Seconds()
	if placementAge > p.thresholds.MaxPlacementAge {
		issues = append(issues, fmt.Sprintf("last placement too old: %v > %v", 
			placementAge, p.thresholds.MaxPlacementAge))
	}
	
	// Check cluster health ratio
	registeredClusters := p.metrics.GetRegisteredClusters()
	healthyClusters := p.metrics.GetHealthyClusters()
	details["registered_clusters"] = registeredClusters
	details["healthy_clusters"] = healthyClusters
	details["min_healthy_cluster_ratio"] = p.thresholds.MinHealthyClusterRatio
	
	var healthyRatio float64
	if registeredClusters > 0 {
		healthyRatio = float64(healthyClusters) / float64(registeredClusters)
	}
	details["healthy_cluster_ratio"] = healthyRatio
	
	if healthyRatio < p.thresholds.MinHealthyClusterRatio {
		issues = append(issues, fmt.Sprintf("healthy cluster ratio too low: %.2f < %.2f (%d/%d clusters healthy)", 
			healthyRatio, p.thresholds.MinHealthyClusterRatio, healthyClusters, registeredClusters))
	}
	
	// Determine overall health
	healthy := len(issues) == 0
	var message string
	if healthy {
		message = fmt.Sprintf("placement engine %s is healthy (pending: %d, latency: %v, success rate: %.1f%%)", 
			p.name, pendingPlacements, placementLatency.Truncate(time.Millisecond), successRate*100)
	} else {
		message = fmt.Sprintf("placement engine %s has %d issue(s): %v", p.name, len(issues), issues)
	}
	
	return health.HealthStatus{
		Healthy:   healthy,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// MockPlacementMetrics is a mock implementation of PlacementMetrics for testing.
type MockPlacementMetrics struct {
	schedulerAvailable    bool
	policyEngineRunning   bool
	pendingPlacements     int
	placementLatency      time.Duration
	placementSuccessRate  float64
	lastPlacementTime     time.Time
	registeredClusters    int
	healthyClusters       int
	mutex                 sync.RWMutex
}

// NewMockPlacementMetrics creates a new mock placement metrics instance.
func NewMockPlacementMetrics() *MockPlacementMetrics {
	return &MockPlacementMetrics{
		schedulerAvailable:   true,
		policyEngineRunning:  true,
		pendingPlacements:    5,
		placementLatency:     500 * time.Millisecond,
		placementSuccessRate: 0.98,
		lastPlacementTime:    time.Now().Add(-2 * time.Minute),
		registeredClusters:   10,
		healthyClusters:      9,
	}
}

// GetSchedulerAvailability returns the mock scheduler availability.
func (m *MockPlacementMetrics) GetSchedulerAvailability() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.schedulerAvailable
}

// SetSchedulerAvailability sets the mock scheduler availability.
func (m *MockPlacementMetrics) SetSchedulerAvailability(available bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.schedulerAvailable = available
}

// GetPolicyEngineStatus returns the mock policy engine status.
func (m *MockPlacementMetrics) GetPolicyEngineStatus() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.policyEngineRunning
}

// SetPolicyEngineStatus sets the mock policy engine status.
func (m *MockPlacementMetrics) SetPolicyEngineStatus(running bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.policyEngineRunning = running
}

// GetPendingPlacements returns the mock pending placements.
func (m *MockPlacementMetrics) GetPendingPlacements() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.pendingPlacements
}

// SetPendingPlacements sets the mock pending placements.
func (m *MockPlacementMetrics) SetPendingPlacements(pending int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.pendingPlacements = pending
}

// GetPlacementLatency returns the mock placement latency.
func (m *MockPlacementMetrics) GetPlacementLatency() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.placementLatency
}

// GetPlacementSuccessRate returns the mock placement success rate.
func (m *MockPlacementMetrics) GetPlacementSuccessRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.placementSuccessRate
}

// GetLastPlacementTime returns the mock last placement time.
func (m *MockPlacementMetrics) GetLastPlacementTime() time.Time {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.lastPlacementTime
}

// GetRegisteredClusters returns the mock registered clusters count.
func (m *MockPlacementMetrics) GetRegisteredClusters() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.registeredClusters
}

// GetHealthyClusters returns the mock healthy clusters count.
func (m *MockPlacementMetrics) GetHealthyClusters() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.healthyClusters
}

// SetClusterCounts sets the mock cluster counts.
func (m *MockPlacementMetrics) SetClusterCounts(registered, healthy int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.registeredClusters = registered
	m.healthyClusters = healthy
}