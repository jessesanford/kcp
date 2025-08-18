/*
Copyright 2023 The KCP Authors.

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

package collectors

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/metrics"
)

// ConnectionCollector collects metrics related to connections between control plane and clusters.
// It tracks connection state, reconnections, message throughput, and heartbeat latency.
type ConnectionCollector struct {
	mu sync.RWMutex

	// Prometheus metrics
	connectionState     *prometheus.GaugeVec
	reconnections       *prometheus.CounterVec
	messageThroughput   *prometheus.CounterVec
	heartbeatLatency    *prometheus.HistogramVec
	connectionDuration  *prometheus.GaugeVec
	messageErrors       *prometheus.CounterVec
	bandwidthUsage      *prometheus.GaugeVec
	activeConnections   *prometheus.GaugeVec

	// Internal state for metrics collection
	registry *metrics.MetricsRegistry
	enabled  bool
}

// NewConnectionCollector creates a new connection metrics collector.
func NewConnectionCollector() *ConnectionCollector {
	return &ConnectionCollector{
		enabled: true, // TODO: integrate with feature flags
	}
}

// Name returns the collector name for registration.
func (c *ConnectionCollector) Name() string {
	return "connection"
}

// Init initializes the collector with the provided registry.
func (c *ConnectionCollector) Init(registry *metrics.MetricsRegistry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.registry = registry
	prom := metrics.NewPrometheusMetrics(registry)

	// Connection state gauge - tracks current connection state (0=disconnected, 1=connected)
	c.connectionState = prom.NewGaugeVec(
		metrics.ConnectionSubsystem,
		"state",
		"Current connection state (0=disconnected, 1=connected)",
		[]string{metrics.LabelCluster, metrics.LabelEndpoint, metrics.LabelWorkspace},
	)

	// Reconnections counter - tracks number of reconnection attempts
	c.reconnections = prom.NewCounterVec(
		metrics.ConnectionSubsystem,
		"reconnections_total",
		"Total number of reconnection attempts",
		[]string{metrics.LabelCluster, metrics.LabelEndpoint, metrics.LabelStatus},
	)

	// Message throughput counter - tracks messages sent/received
	c.messageThroughput = prom.NewCounterVec(
		metrics.ConnectionSubsystem,
		"messages_total",
		"Total number of messages sent/received",
		[]string{metrics.LabelCluster, metrics.LabelEndpoint, "direction", "message_type"},
	)

	// Heartbeat latency histogram - tracks heartbeat round-trip time
	c.heartbeatLatency = prom.NewHistogramVec(
		metrics.ConnectionSubsystem,
		"heartbeat_duration_seconds",
		"Heartbeat round-trip latency",
		[]string{metrics.LabelCluster, metrics.LabelEndpoint},
		metrics.LatencyBuckets,
	)

	// Connection duration gauge - tracks how long connections have been active
	c.connectionDuration = prom.NewGaugeVec(
		metrics.ConnectionSubsystem,
		"duration_seconds",
		"Duration of current connection in seconds",
		[]string{metrics.LabelCluster, metrics.LabelEndpoint},
	)

	// Message errors counter - tracks message transmission errors
	c.messageErrors = prom.NewCounterVec(
		metrics.ConnectionSubsystem,
		"message_errors_total",
		"Total number of message transmission errors",
		[]string{metrics.LabelCluster, metrics.LabelEndpoint, metrics.LabelError, "direction"},
	)

	// Bandwidth usage gauge - tracks current bandwidth utilization
	c.bandwidthUsage = prom.NewGaugeVec(
		metrics.ConnectionSubsystem,
		"bandwidth_bytes_per_second",
		"Current bandwidth usage in bytes per second",
		[]string{metrics.LabelCluster, metrics.LabelEndpoint, "direction"},
	)

	// Active connections gauge - tracks number of active connections per endpoint
	c.activeConnections = prom.NewGaugeVec(
		metrics.ConnectionSubsystem,
		"active_total",
		"Number of currently active connections",
		[]string{metrics.LabelEndpoint, "connection_type"},
	)

	// Register all metrics with Prometheus
	prom.MustRegister(
		c.connectionState,
		c.reconnections,
		c.messageThroughput,
		c.heartbeatLatency,
		c.connectionDuration,
		c.messageErrors,
		c.bandwidthUsage,
		c.activeConnections,
	)

	klog.V(2).Info("Initialized TMC connection metrics collector")
	return nil
}

// Collect gathers current metrics from connection monitoring.
func (c *ConnectionCollector) Collect() error {
	if !c.enabled {
		return nil
	}

	// In a real implementation, this would collect metrics from actual connection monitoring
	klog.V(4).Info("Collecting connection metrics")
	return nil
}

// Close cleans up collector resources.
func (c *ConnectionCollector) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enabled = false
	klog.V(2).Info("Closed TMC connection metrics collector")
	return nil
}

// Public API methods for recording metrics
// These would be called by connection management components

// SetConnectionState sets the current connection state.
func (c *ConnectionCollector) SetConnectionState(cluster, endpoint, workspace string, connected bool) {
	if !c.enabled {
		return
	}

	stateValue := float64(0)
	if connected {
		stateValue = 1
	}
	c.connectionState.WithLabelValues(cluster, endpoint, workspace).Set(stateValue)
}

// RecordReconnection records a reconnection attempt.
func (c *ConnectionCollector) RecordReconnection(cluster, endpoint, status string) {
	if !c.enabled {
		return
	}

	c.reconnections.WithLabelValues(cluster, endpoint, status).Inc()
}

// RecordMessage records a message transmission.
func (c *ConnectionCollector) RecordMessage(cluster, endpoint, direction, messageType string) {
	if !c.enabled {
		return
	}

	c.messageThroughput.WithLabelValues(cluster, endpoint, direction, messageType).Inc()
}

// RecordHeartbeat records a heartbeat latency measurement.
func (c *ConnectionCollector) RecordHeartbeat(cluster, endpoint string, latency time.Duration) {
	if !c.enabled {
		return
	}

	c.heartbeatLatency.WithLabelValues(cluster, endpoint).Observe(latency.Seconds())
}

// SetConnectionDuration sets the duration of the current connection.
func (c *ConnectionCollector) SetConnectionDuration(cluster, endpoint string, duration time.Duration) {
	if !c.enabled {
		return
	}

	c.connectionDuration.WithLabelValues(cluster, endpoint).Set(duration.Seconds())
}

// RecordMessageError records a message transmission error.
func (c *ConnectionCollector) RecordMessageError(cluster, endpoint, errorType, direction string) {
	if !c.enabled {
		return
	}

	c.messageErrors.WithLabelValues(cluster, endpoint, errorType, direction).Inc()
}

// SetBandwidthUsage sets the current bandwidth usage.
func (c *ConnectionCollector) SetBandwidthUsage(cluster, endpoint, direction string, bytesPerSecond float64) {
	if !c.enabled {
		return
	}

	c.bandwidthUsage.WithLabelValues(cluster, endpoint, direction).Set(bytesPerSecond)
}

// SetActiveConnections sets the number of active connections.
func (c *ConnectionCollector) SetActiveConnections(endpoint, connectionType string, count float64) {
	if !c.enabled {
		return
	}

	c.activeConnections.WithLabelValues(endpoint, connectionType).Set(count)
}

// GetConnectionCollector returns a shared instance of the connection collector.
var (
	connectionCollectorInstance *ConnectionCollector
	connectionCollectorOnce     sync.Once
)

// GetConnectionCollector returns the global connection collector instance.
func GetConnectionCollector() *ConnectionCollector {
	connectionCollectorOnce.Do(func() {
		connectionCollectorInstance = NewConnectionCollector()
		// Register with global registry
		if err := metrics.GetRegistry().RegisterCollector(connectionCollectorInstance); err != nil {
			klog.Errorf("Failed to register connection collector: %v", err)
		}
	})
	return connectionCollectorInstance
}