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

// ConnectionMetrics defines the interface for collecting connection health metrics.
type ConnectionMetrics interface {
	// GetHeartbeatStatus returns the last heartbeat timestamp and if it's current.
	GetHeartbeatStatus() (time.Time, bool)
	
	// GetConnectionLatency returns the current connection latency.
	GetConnectionLatency() time.Duration
	
	// GetThroughput returns the current connection throughput (bytes per second).
	GetThroughput() float64
	
	// GetConnectionUptime returns how long the connection has been up.
	GetConnectionUptime() time.Duration
	
	// GetReconnectionCount returns the number of reconnections that have occurred.
	GetReconnectionCount() int64
	
	// GetLastErrorTime returns the timestamp of the last connection error.
	GetLastErrorTime() time.Time
	
	// GetActiveConnections returns the number of active connections.
	GetActiveConnections() int
	
	// IsConnected returns true if the connection is currently active.
	IsConnected() bool
}

// ConnectionHealthMonitor monitors the health of network connections in the TMC system.
type ConnectionHealthMonitor struct {
	name       string
	metrics    ConnectionMetrics
	thresholds ConnectionHealthThresholds
	lastCheck  time.Time
	mutex      sync.RWMutex
}

// ConnectionHealthThresholds defines the thresholds for determining connection health.
type ConnectionHealthThresholds struct {
	// MaxHeartbeatAge is the maximum acceptable time since the last heartbeat.
	MaxHeartbeatAge time.Duration `json:"max_heartbeat_age"`
	
	// MaxConnectionLatency is the maximum acceptable connection latency.
	MaxConnectionLatency time.Duration `json:"max_connection_latency"`
	
	// MinThroughput is the minimum acceptable connection throughput (bytes/sec).
	MinThroughput float64 `json:"min_throughput"`
	
	// MaxReconnectionRate is the maximum acceptable reconnections per hour.
	MaxReconnectionRate float64 `json:"max_reconnection_rate"`
	
	// MinUptime is the minimum expected connection uptime before considering it stable.
	MinUptime time.Duration `json:"min_uptime"`
	
	// MaxErrorAge is the maximum acceptable time since last error (indicates recovery).
	MaxErrorAge time.Duration `json:"max_error_age"`
}

// DefaultConnectionHealthThresholds returns default health thresholds for connection monitoring.
func DefaultConnectionHealthThresholds() ConnectionHealthThresholds {
	return ConnectionHealthThresholds{
		MaxHeartbeatAge:      2 * time.Minute,
		MaxConnectionLatency: 1 * time.Second,
		MinThroughput:        1000.0, // 1KB/sec minimum
		MaxReconnectionRate:  5.0,    // 5 reconnections per hour
		MinUptime:            5 * time.Minute,
		MaxErrorAge:          1 * time.Hour,
	}
}

// NewConnectionHealthMonitor creates a new connection health monitor.
func NewConnectionHealthMonitor(name string, metrics ConnectionMetrics, thresholds ConnectionHealthThresholds) health.HealthChecker {
	monitor := &ConnectionHealthMonitor{
		name:       name,
		metrics:    metrics,
		thresholds: thresholds,
	}
	
	return health.NewBaseHealthChecker(fmt.Sprintf("connection-%s", name), monitor.checkHealth)
}

// checkHealth performs the actual health check for the connection.
func (c *ConnectionHealthMonitor) checkHealth(ctx context.Context) health.HealthStatus {
	c.mutex.Lock()
	c.lastCheck = time.Now()
	c.mutex.Unlock()
	
	var issues []string
	details := make(map[string]interface{})
	
	// Check connection status
	connected := c.metrics.IsConnected()
	details["connected"] = connected
	if !connected {
		issues = append(issues, "connection is not active")
	}
	
	// Check heartbeat status
	lastHeartbeat, heartbeatCurrent := c.metrics.GetHeartbeatStatus()
	heartbeatAge := time.Since(lastHeartbeat)
	details["last_heartbeat"] = lastHeartbeat
	details["heartbeat_current"] = heartbeatCurrent
	details["heartbeat_age_seconds"] = heartbeatAge.Seconds()
	details["max_heartbeat_age_seconds"] = c.thresholds.MaxHeartbeatAge.Seconds()
	if !heartbeatCurrent || heartbeatAge > c.thresholds.MaxHeartbeatAge {
		issues = append(issues, fmt.Sprintf("heartbeat too old: %v > %v", 
			heartbeatAge, c.thresholds.MaxHeartbeatAge))
	}
	
	// Check connection latency
	latency := c.metrics.GetConnectionLatency()
	details["connection_latency_ms"] = latency.Milliseconds()
	details["max_connection_latency_ms"] = c.thresholds.MaxConnectionLatency.Milliseconds()
	if latency > c.thresholds.MaxConnectionLatency {
		issues = append(issues, fmt.Sprintf("connection latency too high: %v > %v", 
			latency, c.thresholds.MaxConnectionLatency))
	}
	
	// Check throughput
	throughput := c.metrics.GetThroughput()
	details["throughput_bytes_per_sec"] = throughput
	details["min_throughput_bytes_per_sec"] = c.thresholds.MinThroughput
	if connected && throughput < c.thresholds.MinThroughput {
		issues = append(issues, fmt.Sprintf("throughput too low: %.2f < %.2f bytes/sec", 
			throughput, c.thresholds.MinThroughput))
	}
	
	// Check uptime
	uptime := c.metrics.GetConnectionUptime()
	details["connection_uptime_seconds"] = uptime.Seconds()
	details["min_uptime_seconds"] = c.thresholds.MinUptime.Seconds()
	
	// Check reconnection rate
	reconnectionCount := c.metrics.GetReconnectionCount()
	reconnectionRate := float64(reconnectionCount) / (uptime.Hours() + 1) // +1 to avoid division by zero
	details["reconnection_count"] = reconnectionCount
	details["reconnection_rate_per_hour"] = reconnectionRate
	details["max_reconnection_rate_per_hour"] = c.thresholds.MaxReconnectionRate
	if reconnectionRate > c.thresholds.MaxReconnectionRate {
		issues = append(issues, fmt.Sprintf("reconnection rate too high: %.2f > %.2f per hour", 
			reconnectionRate, c.thresholds.MaxReconnectionRate))
	}
	
	// Check last error time (recent errors might indicate instability)
	lastErrorTime := c.metrics.GetLastErrorTime()
	errorAge := time.Since(lastErrorTime)
	details["last_error_time"] = lastErrorTime
	details["error_age_seconds"] = errorAge.Seconds()
	details["max_error_age_seconds"] = c.thresholds.MaxErrorAge.Seconds()
	if errorAge < c.thresholds.MaxErrorAge && !lastErrorTime.IsZero() {
		issues = append(issues, fmt.Sprintf("recent connection error: %v ago", errorAge.Truncate(time.Second)))
	}
	
	// Additional context information
	details["active_connections"] = c.metrics.GetActiveConnections()
	
	// Determine overall health
	healthy := len(issues) == 0
	var message string
	if healthy {
		message = fmt.Sprintf("connection %s is healthy (latency: %v, uptime: %v, throughput: %.0f bytes/sec)", 
			c.name, latency.Truncate(time.Millisecond), uptime.Truncate(time.Second), throughput)
	} else {
		message = fmt.Sprintf("connection %s has %d issue(s): %v", c.name, len(issues), issues)
	}
	
	return health.HealthStatus{
		Healthy:   healthy,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// MockConnectionMetrics is a mock implementation of ConnectionMetrics for testing.
type MockConnectionMetrics struct {
	heartbeatTime       time.Time
	heartbeatCurrent    bool
	connectionLatency   time.Duration
	throughput          float64
	connectionUptime    time.Duration
	reconnectionCount   int64
	lastErrorTime       time.Time
	activeConnections   int
	connected           bool
	mutex               sync.RWMutex
}

// NewMockConnectionMetrics creates a new mock connection metrics instance.
func NewMockConnectionMetrics() *MockConnectionMetrics {
	return &MockConnectionMetrics{
		heartbeatTime:     time.Now(),
		heartbeatCurrent:  true,
		connectionLatency: 100 * time.Millisecond,
		throughput:        5000.0,
		connectionUptime:  2 * time.Hour,
		reconnectionCount: 2,
		lastErrorTime:     time.Now().Add(-2 * time.Hour),
		activeConnections: 3,
		connected:         true,
	}
}

// GetHeartbeatStatus returns the mock heartbeat status.
func (m *MockConnectionMetrics) GetHeartbeatStatus() (time.Time, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.heartbeatTime, m.heartbeatCurrent
}

// SetHeartbeatStatus sets the mock heartbeat status.
func (m *MockConnectionMetrics) SetHeartbeatStatus(t time.Time, current bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.heartbeatTime = t
	m.heartbeatCurrent = current
}

// GetConnectionLatency returns the mock connection latency.
func (m *MockConnectionMetrics) GetConnectionLatency() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.connectionLatency
}

// SetConnectionLatency sets the mock connection latency.
func (m *MockConnectionMetrics) SetConnectionLatency(latency time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.connectionLatency = latency
}

// GetThroughput returns the mock throughput.
func (m *MockConnectionMetrics) GetThroughput() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.throughput
}

// SetThroughput sets the mock throughput.
func (m *MockConnectionMetrics) SetThroughput(throughput float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.throughput = throughput
}

// GetConnectionUptime returns the mock connection uptime.
func (m *MockConnectionMetrics) GetConnectionUptime() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.connectionUptime
}

// GetReconnectionCount returns the mock reconnection count.
func (m *MockConnectionMetrics) GetReconnectionCount() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.reconnectionCount
}

// GetLastErrorTime returns the mock last error time.
func (m *MockConnectionMetrics) GetLastErrorTime() time.Time {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.lastErrorTime
}

// GetActiveConnections returns the mock active connections count.
func (m *MockConnectionMetrics) GetActiveConnections() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.activeConnections
}

// IsConnected returns the mock connection status.
func (m *MockConnectionMetrics) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.connected
}

// SetConnected sets the mock connection status.
func (m *MockConnectionMetrics) SetConnected(connected bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.connected = connected
}