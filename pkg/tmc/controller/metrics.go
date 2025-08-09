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

import "github.com/prometheus/client_golang/prometheus"

// ManagerMetrics defines the prometheus metrics used by the TMC controller manager.
// These metrics follow KCP patterns for observability and provide essential insights
// into controller health, performance, and operational status.
type ManagerMetrics struct {
	// controllersTotal tracks the total number of controllers managed by this manager
	controllersTotal prometheus.Gauge

	// controllersHealthy tracks the number of healthy controllers currently running
	controllersHealthy prometheus.Gauge

	// reconcileTotal tracks the total number of reconciliation attempts by result
	// Labels: controller (controller name), result (success/error)
	reconcileTotal *prometheus.CounterVec
}

// NewManagerMetrics creates and registers a new set of manager metrics.
// The metrics are registered with the default prometheus registry following
// KCP naming conventions for controller observability.
func NewManagerMetrics() *ManagerMetrics {
	metrics := &ManagerMetrics{
		controllersTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "tmc_controllers_total",
				Help: "Total number of TMC controllers managed by this manager",
			},
		),
		controllersHealthy: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "tmc_controllers_healthy",
				Help: "Number of healthy TMC controllers currently running",
			},
		),
		reconcileTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tmc_reconcile_total",
				Help: "Total number of TMC reconciliation attempts by controller and result",
			},
			[]string{"controller", "result"},
		),
	}

	// Register all metrics with the default registry
	prometheus.MustRegister(
		metrics.controllersTotal,
		metrics.controllersHealthy,
		metrics.reconcileTotal,
	)

	return metrics
}

// RecordControllerStart records that a controller has started successfully.
func (m *ManagerMetrics) RecordControllerStart(controllerName string) {
	// Controllers are counted at the manager level, not individually
	// This is handled by the manager's lifecycle methods
}

// RecordReconcileSuccess records a successful reconciliation attempt.
func (m *ManagerMetrics) RecordReconcileSuccess(controllerName string) {
	m.reconcileTotal.WithLabelValues(controllerName, "success").Inc()
}

// RecordReconcileError records a failed reconciliation attempt.
func (m *ManagerMetrics) RecordReconcileError(controllerName string) {
	m.reconcileTotal.WithLabelValues(controllerName, "error").Inc()
}