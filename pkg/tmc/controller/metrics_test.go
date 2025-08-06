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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManagerMetrics(t *testing.T) {
	t.Run("creates all metrics", func(t *testing.T) {
		metrics := NewManagerMetrics()
		require.NotNil(t, metrics)
		assert.NotNil(t, metrics.controllersTotal)
		assert.NotNil(t, metrics.controllersHealthy)
		assert.NotNil(t, metrics.reconcileTotal)
	})
}

func TestManagerMetrics_MustRegister(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		// Create a custom registry to avoid conflicts
		registry := prometheus.NewRegistry()
		
		// Create metrics with unique names
		metrics := &ManagerMetrics{
			controllersTotal: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "test_metrics_register_controllers_total",
					Help: "Total number of TMC controllers",
				},
			),
			controllersHealthy: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "test_metrics_register_controllers_healthy", 
					Help: "Number of healthy TMC controllers",
				},
			),
			reconcileTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "test_metrics_register_reconcile_total",
					Help: "Total number of reconciliation attempts",
				},
				[]string{"controller", "result"},
			),
		}

		// Should not panic
		assert.NotPanics(t, func() {
			registry.MustRegister(
				metrics.controllersTotal,
				metrics.controllersHealthy,
				metrics.reconcileTotal,
			)
		})
	})
}

func TestNewMetricsRecorder(t *testing.T) {
	t.Run("creates recorder", func(t *testing.T) {
		metrics := NewManagerMetrics()
		recorder := NewMetricsRecorder(metrics)
		assert.NotNil(t, recorder)
		assert.IsType(t, &metricsRecorder{}, recorder)
	})
}

func TestMetricsRecorder_RecordReconcile(t *testing.T) {
	t.Run("records successful reconcile", func(t *testing.T) {
		// Create isolated registry
		registry := prometheus.NewRegistry()
		
		reconcileCounter := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_reconcile_success_total",
				Help: "Total number of reconciliation attempts",
			},
			[]string{"controller", "result"},
		)
		registry.MustRegister(reconcileCounter)
		
		metrics := &ManagerMetrics{
			reconcileTotal: reconcileCounter,
		}
		recorder := NewMetricsRecorder(metrics)

		// Record successful reconcile
		recorder.RecordReconcile("test-controller", "success")

		// Verify metric was incremented
		metricValue := testutil.ToFloat64(reconcileCounter.WithLabelValues("test-controller", "success"))
		assert.Equal(t, 1.0, metricValue)
	})

	t.Run("records failed reconcile", func(t *testing.T) {
		// Create isolated registry
		registry := prometheus.NewRegistry()
		
		reconcileCounter := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_reconcile_error_total",
				Help: "Total number of reconciliation attempts",
			},
			[]string{"controller", "result"},
		)
		registry.MustRegister(reconcileCounter)
		
		metrics := &ManagerMetrics{
			reconcileTotal: reconcileCounter,
		}
		recorder := NewMetricsRecorder(metrics)

		// Record failed reconcile
		recorder.RecordReconcile("test-controller", "error")

		// Verify metric was incremented
		metricValue := testutil.ToFloat64(reconcileCounter.WithLabelValues("test-controller", "error"))
		assert.Equal(t, 1.0, metricValue)
	})

	t.Run("records multiple reconciles", func(t *testing.T) {
		// Create isolated registry
		registry := prometheus.NewRegistry()
		
		reconcileCounter := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_reconcile_multiple_total",
				Help: "Total number of reconciliation attempts",
			},
			[]string{"controller", "result"},
		)
		registry.MustRegister(reconcileCounter)
		
		metrics := &ManagerMetrics{
			reconcileTotal: reconcileCounter,
		}
		recorder := NewMetricsRecorder(metrics)

		// Record multiple reconciles
		for i := 0; i < 5; i++ {
			recorder.RecordReconcile("test-controller", "success")
		}
		for i := 0; i < 2; i++ {
			recorder.RecordReconcile("test-controller", "error")
		}

		// Verify metrics were incremented correctly
		successValue := testutil.ToFloat64(reconcileCounter.WithLabelValues("test-controller", "success"))
		errorValue := testutil.ToFloat64(reconcileCounter.WithLabelValues("test-controller", "error"))
		
		assert.Equal(t, 5.0, successValue)
		assert.Equal(t, 2.0, errorValue)
	})
}

func TestNewControllerHealthChecker(t *testing.T) {
	t.Run("creates health checker", func(t *testing.T) {
		maxErrors := 5
		checker := NewControllerHealthChecker(maxErrors)
		
		assert.NotNil(t, checker)
		assert.Equal(t, maxErrors, checker.maxErrors)
		assert.Equal(t, 0, checker.errorCount)
		assert.True(t, time.Since(checker.lastReconcile) < time.Second) // Should be recent
	})
}

func TestControllerHealthChecker_RecordReconcile(t *testing.T) {
	t.Run("records successful reconcile", func(t *testing.T) {
		checker := NewControllerHealthChecker(5)
		
		// Set up some errors first
		checker.errorCount = 3
		oldTime := checker.lastReconcile
		
		// Wait a bit to ensure time changes
		time.Sleep(10 * time.Millisecond)
		
		// Record reconcile
		checker.RecordReconcile()
		
		// Should reset error count and update time
		assert.Equal(t, 0, checker.errorCount)
		assert.True(t, checker.lastReconcile.After(oldTime))
	})
}

func TestControllerHealthChecker_RecordError(t *testing.T) {
	t.Run("increments error count", func(t *testing.T) {
		checker := NewControllerHealthChecker(5)
		
		assert.Equal(t, 0, checker.errorCount)
		
		checker.RecordError()
		assert.Equal(t, 1, checker.errorCount)
		
		checker.RecordError()
		assert.Equal(t, 2, checker.errorCount)
	})
}

func TestControllerHealthChecker_IsHealthy(t *testing.T) {
	tests := map[string]struct {
		maxErrors      int
		currentErrors  int
		timeSince      time.Duration
		expectedHealthy bool
	}{
		"healthy with no errors": {
			maxErrors:       5,
			currentErrors:   0,
			timeSince:       1 * time.Minute,
			expectedHealthy: true,
		},
		"healthy with few errors": {
			maxErrors:       5,
			currentErrors:   3,
			timeSince:       1 * time.Minute,
			expectedHealthy: true,
		},
		"unhealthy with too many errors": {
			maxErrors:       5,
			currentErrors:   5,
			timeSince:       1 * time.Minute,
			expectedHealthy: false,
		},
		"unhealthy with no recent reconciliation": {
			maxErrors:       5,
			currentErrors:   2,
			timeSince:       10 * time.Minute,
			expectedHealthy: false,
		},
		"unhealthy with both issues": {
			maxErrors:       3,
			currentErrors:   5,
			timeSince:       10 * time.Minute,
			expectedHealthy: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			checker := NewControllerHealthChecker(tc.maxErrors)
			checker.errorCount = tc.currentErrors
			checker.lastReconcile = time.Now().Add(-tc.timeSince)
			
			healthy := checker.IsHealthy()
			assert.Equal(t, tc.expectedHealthy, healthy)
		})
	}
}

func TestControllerHealthChecker_Integration(t *testing.T) {
	t.Run("realistic health checking scenario", func(t *testing.T) {
		checker := NewControllerHealthChecker(3)
		
		// Initially healthy
		assert.True(t, checker.IsHealthy())
		
		// Record some successful reconciles
		checker.RecordReconcile()
		time.Sleep(10 * time.Millisecond)
		checker.RecordReconcile()
		assert.True(t, checker.IsHealthy())
		
		// Record some errors, but not too many
		checker.RecordError()
		checker.RecordError()
		assert.True(t, checker.IsHealthy())
		
		// One more error should make it unhealthy
		checker.RecordError()
		assert.False(t, checker.IsHealthy())
		
		// A successful reconcile should restore health
		checker.RecordReconcile()
		assert.True(t, checker.IsHealthy())
	})
}

func TestManagerMetrics_Integration(t *testing.T) {
	t.Run("full metrics workflow", func(t *testing.T) {
		// Create isolated registry
		registry := prometheus.NewRegistry()
		
		// Create unique metrics
		metrics := &ManagerMetrics{
			controllersTotal: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "test_integration_controllers_total",
					Help: "Total number of TMC controllers",
				},
			),
			controllersHealthy: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "test_integration_controllers_healthy",
					Help: "Number of healthy TMC controllers",
				},
			),
			reconcileTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "test_integration_reconcile_total",
					Help: "Total number of reconciliation attempts",
				},
				[]string{"controller", "result"},
			),
		}
		
		// Register metrics
		registry.MustRegister(
			metrics.controllersTotal,
			metrics.controllersHealthy, 
			metrics.reconcileTotal,
		)

		// Create recorder
		recorder := NewMetricsRecorder(metrics)
		
		// Simulate controller lifecycle
		metrics.controllersTotal.Set(2)
		metrics.controllersHealthy.Set(2)
		
		// Simulate reconciliation activity
		recorder.RecordReconcile("controller-1", "success")
		recorder.RecordReconcile("controller-2", "success")
		recorder.RecordReconcile("controller-1", "error")
		recorder.RecordReconcile("controller-2", "success")
		
		// One controller becomes unhealthy
		metrics.controllersHealthy.Set(1)
		
		// Verify final state
		assert.Equal(t, 2.0, testutil.ToFloat64(metrics.controllersTotal))
		assert.Equal(t, 1.0, testutil.ToFloat64(metrics.controllersHealthy))
		assert.Equal(t, 1.0, testutil.ToFloat64(metrics.reconcileTotal.WithLabelValues("controller-1", "success")))
		assert.Equal(t, 2.0, testutil.ToFloat64(metrics.reconcileTotal.WithLabelValues("controller-2", "success")))
		assert.Equal(t, 1.0, testutil.ToFloat64(metrics.reconcileTotal.WithLabelValues("controller-1", "error")))
	})
}