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

// SyncerMetrics defines the interface for collecting syncer metrics.
// This interface allows the health monitor to be decoupled from specific metrics implementations.
type SyncerMetrics interface {
	// GetQueueDepth returns the current depth of the syncer work queue.
	GetQueueDepth() int
	
	// GetLastSyncTime returns the timestamp of the last successful sync operation.
	GetLastSyncTime() time.Time
	
	// GetErrorRate returns the error rate (errors per minute) over the last period.
	GetErrorRate() float64
	
	// GetSyncLatency returns the average sync latency in milliseconds.
	GetSyncLatency() time.Duration
	
	// IsConnected returns true if the syncer is connected to its target cluster.
	IsConnected() bool
	
	// GetTotalSyncs returns the total number of sync operations performed.
	GetTotalSyncs() int64
	
	// GetTotalErrors returns the total number of sync errors.
	GetTotalErrors() int64
}

// SyncerHealthMonitor monitors the health of a TMC syncer component.
type SyncerHealthMonitor struct {
	name         string
	metrics      SyncerMetrics
	thresholds   SyncerHealthThresholds
	lastCheck    time.Time
	mutex        sync.RWMutex
}

// SyncerHealthThresholds defines the thresholds for determining syncer health.
type SyncerHealthThresholds struct {
	// MaxQueueDepth is the maximum acceptable work queue depth.
	MaxQueueDepth int `json:"max_queue_depth"`
	
	// MaxSyncAge is the maximum acceptable time since the last sync.
	MaxSyncAge time.Duration `json:"max_sync_age"`
	
	// MaxErrorRate is the maximum acceptable error rate (errors per minute).
	MaxErrorRate float64 `json:"max_error_rate"`
	
	// MaxSyncLatency is the maximum acceptable sync latency.
	MaxSyncLatency time.Duration `json:"max_sync_latency"`
}

// DefaultSyncerHealthThresholds returns default health thresholds for syncer monitoring.
func DefaultSyncerHealthThresholds() SyncerHealthThresholds {
	return SyncerHealthThresholds{
		MaxQueueDepth:  1000,
		MaxSyncAge:     5 * time.Minute,
		MaxErrorRate:   10.0, // 10 errors per minute
		MaxSyncLatency: 30 * time.Second,
	}
}

// NewSyncerHealthMonitor creates a new syncer health monitor.
func NewSyncerHealthMonitor(name string, metrics SyncerMetrics, thresholds SyncerHealthThresholds) health.HealthChecker {
	monitor := &SyncerHealthMonitor{
		name:       name,
		metrics:    metrics,
		thresholds: thresholds,
	}
	
	return health.NewBaseHealthChecker(fmt.Sprintf("syncer-%s", name), monitor.checkHealth)
}

// checkHealth performs the actual health check for the syncer.
func (s *SyncerHealthMonitor) checkHealth(ctx context.Context) health.HealthStatus {
	s.mutex.Lock()
	s.lastCheck = time.Now()
	s.mutex.Unlock()
	
	var issues []string
	details := make(map[string]interface{})
	
	// Check connection status
	connected := s.metrics.IsConnected()
	details["connected"] = connected
	if !connected {
		issues = append(issues, "syncer is not connected to target cluster")
	}
	
	// Check queue depth
	queueDepth := s.metrics.GetQueueDepth()
	details["queue_depth"] = queueDepth
	details["max_queue_depth"] = s.thresholds.MaxQueueDepth
	if queueDepth > s.thresholds.MaxQueueDepth {
		issues = append(issues, fmt.Sprintf("queue depth too high: %d > %d", queueDepth, s.thresholds.MaxQueueDepth))
	}
	
	// Check last sync time
	lastSync := s.metrics.GetLastSyncTime()
	syncAge := time.Since(lastSync)
	details["last_sync_time"] = lastSync
	details["sync_age_seconds"] = syncAge.Seconds()
	details["max_sync_age_seconds"] = s.thresholds.MaxSyncAge.Seconds()
	if syncAge > s.thresholds.MaxSyncAge {
		issues = append(issues, fmt.Sprintf("last sync too old: %v > %v", syncAge, s.thresholds.MaxSyncAge))
	}
	
	// Check error rate
	errorRate := s.metrics.GetErrorRate()
	details["error_rate"] = errorRate
	details["max_error_rate"] = s.thresholds.MaxErrorRate
	if errorRate > s.thresholds.MaxErrorRate {
		issues = append(issues, fmt.Sprintf("error rate too high: %.2f > %.2f", errorRate, s.thresholds.MaxErrorRate))
	}
	
	// Check sync latency
	syncLatency := s.metrics.GetSyncLatency()
	details["sync_latency_ms"] = syncLatency.Milliseconds()
	details["max_sync_latency_ms"] = s.thresholds.MaxSyncLatency.Milliseconds()
	if syncLatency > s.thresholds.MaxSyncLatency {
		issues = append(issues, fmt.Sprintf("sync latency too high: %v > %v", syncLatency, s.thresholds.MaxSyncLatency))
	}
	
	// Add additional metrics for context
	details["total_syncs"] = s.metrics.GetTotalSyncs()
	details["total_errors"] = s.metrics.GetTotalErrors()
	
	// Determine overall health
	healthy := len(issues) == 0
	var message string
	if healthy {
		message = fmt.Sprintf("syncer %s is healthy (queue: %d, last sync: %v ago)", 
			s.name, queueDepth, syncAge.Truncate(time.Second))
	} else {
		message = fmt.Sprintf("syncer %s has %d issue(s): %v", s.name, len(issues), issues)
	}
	
	return health.HealthStatus{
		Healthy:   healthy,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// MockSyncerMetrics is a mock implementation of SyncerMetrics for testing.
type MockSyncerMetrics struct {
	queueDepth    int
	lastSyncTime  time.Time
	errorRate     float64
	syncLatency   time.Duration
	connected     bool
	totalSyncs    int64
	totalErrors   int64
	mutex         sync.RWMutex
}

// NewMockSyncerMetrics creates a new mock syncer metrics instance.
func NewMockSyncerMetrics() *MockSyncerMetrics {
	return &MockSyncerMetrics{
		queueDepth:   0,
		lastSyncTime: time.Now(),
		errorRate:    0.0,
		syncLatency:  100 * time.Millisecond,
		connected:    true,
		totalSyncs:   1000,
		totalErrors:  5,
	}
}

// GetQueueDepth returns the mock queue depth.
func (m *MockSyncerMetrics) GetQueueDepth() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.queueDepth
}

// SetQueueDepth sets the mock queue depth.
func (m *MockSyncerMetrics) SetQueueDepth(depth int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.queueDepth = depth
}

// GetLastSyncTime returns the mock last sync time.
func (m *MockSyncerMetrics) GetLastSyncTime() time.Time {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.lastSyncTime
}

// SetLastSyncTime sets the mock last sync time.
func (m *MockSyncerMetrics) SetLastSyncTime(t time.Time) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.lastSyncTime = t
}

// GetErrorRate returns the mock error rate.
func (m *MockSyncerMetrics) GetErrorRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.errorRate
}

// SetErrorRate sets the mock error rate.
func (m *MockSyncerMetrics) SetErrorRate(rate float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.errorRate = rate
}

// GetSyncLatency returns the mock sync latency.
func (m *MockSyncerMetrics) GetSyncLatency() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.syncLatency
}

// SetSyncLatency sets the mock sync latency.
func (m *MockSyncerMetrics) SetSyncLatency(latency time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.syncLatency = latency
}

// IsConnected returns the mock connection status.
func (m *MockSyncerMetrics) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.connected
}

// SetConnected sets the mock connection status.
func (m *MockSyncerMetrics) SetConnected(connected bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.connected = connected
}

// GetTotalSyncs returns the mock total syncs.
func (m *MockSyncerMetrics) GetTotalSyncs() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.totalSyncs
}

// GetTotalErrors returns the mock total errors.
func (m *MockSyncerMetrics) GetTotalErrors() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.totalErrors
}