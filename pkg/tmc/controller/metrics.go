// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ManagerMetrics provides basic metrics for the controller manager.
type ManagerMetrics struct {
	controllersTotal   prometheus.Gauge
	controllersHealthy prometheus.Gauge
	reconcileTotal     *prometheus.CounterVec
}

// MetricsRecorder provides basic interface for recording controller metrics.
type MetricsRecorder interface {
	// RecordReconcile records a reconciliation attempt with its result
	RecordReconcile(controller, result string)
}

// metricsRecorder implements MetricsRecorder using the ManagerMetrics
type metricsRecorder struct {
	metrics *ManagerMetrics
}

// NewMetricsRecorder creates a new metrics recorder.
func NewMetricsRecorder(metrics *ManagerMetrics) MetricsRecorder {
	return &metricsRecorder{
		metrics: metrics,
	}
}

// RecordReconcile implements MetricsRecorder.RecordReconcile
func (m *metricsRecorder) RecordReconcile(controller, result string) {
	m.metrics.reconcileTotal.WithLabelValues(controller, result).Inc()
}

// ControllerHealthChecker provides basic health checking for controllers.
type ControllerHealthChecker struct {
	lastReconcile time.Time
	errorCount    int
	maxErrors     int
}

// NewControllerHealthChecker creates a new health checker.
func NewControllerHealthChecker(maxErrors int) *ControllerHealthChecker {
	return &ControllerHealthChecker{
		lastReconcile: time.Now(),
		maxErrors:     maxErrors,
	}
}

// RecordReconcile records a successful reconciliation.
func (c *ControllerHealthChecker) RecordReconcile() {
	c.lastReconcile = time.Now()
	c.errorCount = 0
}

// RecordError increments the error count.
func (c *ControllerHealthChecker) RecordError() {
	c.errorCount++
}

// IsHealthy returns true if the controller is healthy.
func (c *ControllerHealthChecker) IsHealthy() bool {
	// Unhealthy if too many errors or no recent reconciliation
	return c.errorCount < c.maxErrors && time.Since(c.lastReconcile) < 5*time.Minute
}