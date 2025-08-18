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

// ControllerMetrics defines the interface for collecting controller metrics.
type ControllerMetrics interface {
	// GetWorkQueueDepth returns the current depth of the controller's work queue.
	GetWorkQueueDepth() int
	
	// GetWorkQueueAdds returns the total number of items added to the work queue.
	GetWorkQueueAdds() int64
	
	// GetWorkQueueRetries returns the number of items being retried.
	GetWorkQueueRetries() int
	
	// GetReconcileRate returns the reconciliation rate (reconciles per minute).
	GetReconcileRate() float64
	
	// GetReconcileErrors returns the total number of reconciliation errors.
	GetReconcileErrors() int64
	
	// GetLastReconcileTime returns the timestamp of the last reconciliation.
	GetLastReconcileTime() time.Time
	
	// GetAverageReconcileLatency returns the average reconciliation latency.
	GetAverageReconcileLatency() time.Duration
	
	// IsLeaderElected returns true if this controller instance is the leader.
	IsLeaderElected() bool
}

// ControllerHealthMonitor monitors the health of a TMC controller component.
type ControllerHealthMonitor struct {
	name       string
	metrics    ControllerMetrics
	thresholds ControllerHealthThresholds
	lastCheck  time.Time
	mutex      sync.RWMutex
}

// ControllerHealthThresholds defines the thresholds for determining controller health.
type ControllerHealthThresholds struct {
	// MaxWorkQueueDepth is the maximum acceptable work queue depth.
	MaxWorkQueueDepth int `json:"max_work_queue_depth"`
	
	// MaxRetryCount is the maximum acceptable number of retries.
	MaxRetryCount int `json:"max_retry_count"`
	
	// MaxReconcileAge is the maximum acceptable time since last reconcile.
	MaxReconcileAge time.Duration `json:"max_reconcile_age"`
	
	// MaxReconcileLatency is the maximum acceptable reconciliation latency.
	MaxReconcileLatency time.Duration `json:"max_reconcile_latency"`
	
	// MinReconcileRate is the minimum expected reconcile rate (per minute).
	MinReconcileRate float64 `json:"min_reconcile_rate"`
}

// DefaultControllerHealthThresholds returns default health thresholds for controller monitoring.
func DefaultControllerHealthThresholds() ControllerHealthThresholds {
	return ControllerHealthThresholds{
		MaxWorkQueueDepth:   500,
		MaxRetryCount:       50,
		MaxReconcileAge:     10 * time.Minute,
		MaxReconcileLatency: 5 * time.Second,
		MinReconcileRate:    1.0, // 1 reconcile per minute minimum
	}
}

// NewControllerHealthMonitor creates a new controller health monitor.
func NewControllerHealthMonitor(name string, metrics ControllerMetrics, thresholds ControllerHealthThresholds) health.HealthChecker {
	monitor := &ControllerHealthMonitor{
		name:       name,
		metrics:    metrics,
		thresholds: thresholds,
	}
	
	return health.NewBaseHealthChecker(fmt.Sprintf("controller-%s", name), monitor.checkHealth)
}

// checkHealth performs the actual health check for the controller.
func (c *ControllerHealthMonitor) checkHealth(ctx context.Context) health.HealthStatus {
	c.mutex.Lock()
	c.lastCheck = time.Now()
	c.mutex.Unlock()
	
	var issues []string
	details := make(map[string]interface{})
	
	// Check leader election status
	isLeader := c.metrics.IsLeaderElected()
	details["is_leader"] = isLeader
	if !isLeader {
		// Being a non-leader is not necessarily unhealthy, but worth noting
		details["leader_note"] = "controller is not currently the elected leader"
	}
	
	// Check work queue depth
	queueDepth := c.metrics.GetWorkQueueDepth()
	details["work_queue_depth"] = queueDepth
	details["max_work_queue_depth"] = c.thresholds.MaxWorkQueueDepth
	if queueDepth > c.thresholds.MaxWorkQueueDepth {
		issues = append(issues, fmt.Sprintf("work queue depth too high: %d > %d", queueDepth, c.thresholds.MaxWorkQueueDepth))
	}
	
	// Check retry count
	retryCount := c.metrics.GetWorkQueueRetries()
	details["retry_count"] = retryCount
	details["max_retry_count"] = c.thresholds.MaxRetryCount
	if retryCount > c.thresholds.MaxRetryCount {
		issues = append(issues, fmt.Sprintf("too many retries: %d > %d", retryCount, c.thresholds.MaxRetryCount))
	}
	
	// Check last reconcile time
	lastReconcile := c.metrics.GetLastReconcileTime()
	reconcileAge := time.Since(lastReconcile)
	details["last_reconcile_time"] = lastReconcile
	details["reconcile_age_seconds"] = reconcileAge.Seconds()
	details["max_reconcile_age_seconds"] = c.thresholds.MaxReconcileAge.Seconds()
	if reconcileAge > c.thresholds.MaxReconcileAge {
		issues = append(issues, fmt.Sprintf("last reconcile too old: %v > %v", reconcileAge, c.thresholds.MaxReconcileAge))
	}
	
	// Check reconcile latency
	reconcileLatency := c.metrics.GetAverageReconcileLatency()
	details["reconcile_latency_ms"] = reconcileLatency.Milliseconds()
	details["max_reconcile_latency_ms"] = c.thresholds.MaxReconcileLatency.Milliseconds()
	if reconcileLatency > c.thresholds.MaxReconcileLatency {
		issues = append(issues, fmt.Sprintf("reconcile latency too high: %v > %v", reconcileLatency, c.thresholds.MaxReconcileLatency))
	}
	
	// Check reconcile rate (only if we're the leader)
	reconcileRate := c.metrics.GetReconcileRate()
	details["reconcile_rate"] = reconcileRate
	details["min_reconcile_rate"] = c.thresholds.MinReconcileRate
	if isLeader && reconcileRate < c.thresholds.MinReconcileRate {
		issues = append(issues, fmt.Sprintf("reconcile rate too low: %.2f < %.2f", reconcileRate, c.thresholds.MinReconcileRate))
	}
	
	// Add additional metrics for context
	details["work_queue_adds"] = c.metrics.GetWorkQueueAdds()
	details["reconcile_errors"] = c.metrics.GetReconcileErrors()
	
	// Determine overall health
	healthy := len(issues) == 0
	var message string
	if healthy {
		leaderStatus := ""
		if isLeader {
			leaderStatus = " (leader)"
		}
		message = fmt.Sprintf("controller %s is healthy%s (queue: %d, last reconcile: %v ago)", 
			c.name, leaderStatus, queueDepth, reconcileAge.Truncate(time.Second))
	} else {
		message = fmt.Sprintf("controller %s has %d issue(s): %v", c.name, len(issues), issues)
	}
	
	return health.HealthStatus{
		Healthy:   healthy,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// MockControllerMetrics is a mock implementation of ControllerMetrics for testing.
type MockControllerMetrics struct {
	workQueueDepth          int
	workQueueAdds           int64
	workQueueRetries        int
	reconcileRate           float64
	reconcileErrors         int64
	lastReconcileTime       time.Time
	averageReconcileLatency time.Duration
	isLeaderElected         bool
	mutex                   sync.RWMutex
}

// NewMockControllerMetrics creates a new mock controller metrics instance.
func NewMockControllerMetrics() *MockControllerMetrics {
	return &MockControllerMetrics{
		workQueueDepth:          10,
		workQueueAdds:           5000,
		workQueueRetries:        2,
		reconcileRate:           10.0,
		reconcileErrors:         3,
		lastReconcileTime:       time.Now().Add(-30 * time.Second),
		averageReconcileLatency: 500 * time.Millisecond,
		isLeaderElected:         true,
	}
}

// GetWorkQueueDepth returns the mock work queue depth.
func (m *MockControllerMetrics) GetWorkQueueDepth() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.workQueueDepth
}

// SetWorkQueueDepth sets the mock work queue depth.
func (m *MockControllerMetrics) SetWorkQueueDepth(depth int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.workQueueDepth = depth
}

// GetWorkQueueAdds returns the mock work queue adds.
func (m *MockControllerMetrics) GetWorkQueueAdds() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.workQueueAdds
}

// GetWorkQueueRetries returns the mock work queue retries.
func (m *MockControllerMetrics) GetWorkQueueRetries() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.workQueueRetries
}

// SetWorkQueueRetries sets the mock work queue retries.
func (m *MockControllerMetrics) SetWorkQueueRetries(retries int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.workQueueRetries = retries
}

// GetReconcileRate returns the mock reconcile rate.
func (m *MockControllerMetrics) GetReconcileRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.reconcileRate
}

// GetReconcileErrors returns the mock reconcile errors.
func (m *MockControllerMetrics) GetReconcileErrors() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.reconcileErrors
}

// GetLastReconcileTime returns the mock last reconcile time.
func (m *MockControllerMetrics) GetLastReconcileTime() time.Time {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.lastReconcileTime
}

// SetLastReconcileTime sets the mock last reconcile time.
func (m *MockControllerMetrics) SetLastReconcileTime(t time.Time) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.lastReconcileTime = t
}

// GetAverageReconcileLatency returns the mock average reconcile latency.
func (m *MockControllerMetrics) GetAverageReconcileLatency() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.averageReconcileLatency
}

// IsLeaderElected returns the mock leader election status.
func (m *MockControllerMetrics) IsLeaderElected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isLeaderElected
}

// SetLeaderElected sets the mock leader election status.
func (m *MockControllerMetrics) SetLeaderElected(leader bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.isLeaderElected = leader
}