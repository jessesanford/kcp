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
	"github.com/stretchr/testify/assert"
)

func TestNewBaseController(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		// Create test-specific metrics with unique registry
		testRegistry := prometheus.NewRegistry()
		testMetrics := &ManagerMetrics{
			controllersTotal: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "test_tmc_controllers_total",
					Help: "Total number of TMC controllers",
				},
			),
			controllersHealthy: prometheus.NewGauge(
				prometheus.GaugeOpts{
					Name: "test_tmc_controllers_healthy",
					Help: "Number of healthy TMC controllers",
				},
			),
			reconcileTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "test_tmc_reconcile_total",
					Help: "Total number of reconciliation attempts",
				},
				[]string{"controller", "result"},
			),
		}
		testRegistry.MustRegister(
			testMetrics.controllersTotal,
			testMetrics.controllersHealthy,
			testMetrics.reconcileTotal,
		)

		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  5,
			Metrics:      testMetrics,
		}

		controller := NewBaseController(config)
		assert.NotNil(t, controller)
		assert.Equal(t, "test-controller", controller.Name())
		// Note: IsHealthy() returns false until the controller is started
	})

	t.Run("nil config panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewBaseController(nil)
		})
	})
}
