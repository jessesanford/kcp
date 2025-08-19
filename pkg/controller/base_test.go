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
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewBaseController(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		// Create test-specific metrics with unique registry
		testRegistry := prometheus.NewRegistry()
		testMetrics := createTestMetrics(testRegistry)

		config := &BaseControllerConfig{
			Name:         "test-controller",
			Workspace:    logicalcluster.Name("test-workspace"),
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  5,
			Reconciler:   &testReconciler{},
			Metrics:      testMetrics,
			// KcpClusterClient can be nil for tests
		}

		controller := NewBaseController(config)
		assert.NotNil(t, controller)
		assert.Equal(t, "test-controller", controller.Name())
		// Note: IsHealthy() returns false until the controller is started

		// Test KCP-specific functionality
		impl := controller.(*baseControllerImpl)
		assert.Equal(t, logicalcluster.Name("test-workspace"), impl.GetWorkspace())
		assert.Equal(t, config.Reconciler, impl.GetReconciler())
	})

	t.Run("nil config panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewBaseController(nil)
		})
	})

	t.Run("empty workspace panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewBaseController(&BaseControllerConfig{
				Name:      "test",
				Workspace: logicalcluster.Name(""),
			})
		})
	})

	t.Run("nil reconciler panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewBaseController(&BaseControllerConfig{
				Name:       "test",
				Workspace:  logicalcluster.Name("test"),
				Reconciler: nil,
			})
		})
	})
}

// createTestMetrics creates a test metrics instance with proper registration
func createTestMetrics(registry *prometheus.Registry) *ManagerMetrics {
	metrics := &ManagerMetrics{
		controllersTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "test_controllers_total",
				Help: "Total number of test controllers",
			},
		),
		controllersHealthy: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "test_controllers_healthy",
				Help: "Number of healthy test controllers",
			},
		),
		reconcileTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_reconcile_total",
				Help: "Total number of test reconciliation attempts",
			},
			[]string{"controller", "result"},
		),
	}
	registry.MustRegister(
		metrics.controllersTotal,
		metrics.controllersHealthy,
		metrics.reconcileTotal,
	)
	return metrics
}

// testReconciler is a minimal test implementation of Reconciler
type testReconciler struct{}

func (r *testReconciler) Reconcile(ctx context.Context, key string) error {
	return nil
}

// testReconcilerWithCommitter implements ReconcilerWithCommit for testing
type testReconcilerWithCommitter struct {
	testReconciler
}

func (r *testReconcilerWithCommitter) GetCommitFunc() interface{} {
	return func(ctx context.Context, old, new interface{}) error {
		return nil
	}
}

func TestCommitterPatternSupport(t *testing.T) {
	testRegistry := prometheus.NewRegistry()
	testMetrics := createTestMetrics(testRegistry)

	t.Run("reconciler without committer pattern", func(t *testing.T) {
		config := &BaseControllerConfig{
			Name:       "test-controller",
			Workspace:  logicalcluster.Name("test-workspace"),
			Reconciler: &testReconciler{},
			Metrics:    testMetrics,
		}

		controller := NewBaseController(config)
		impl := controller.(*baseControllerImpl)

		assert.False(t, impl.SupportsCommitterPattern())
	})

	t.Run("reconciler with committer pattern", func(t *testing.T) {
		config := &BaseControllerConfig{
			Name:       "test-controller-commit",
			Workspace:  logicalcluster.Name("test-workspace"),
			Reconciler: &testReconcilerWithCommitter{},
			Metrics:    testMetrics,
		}

		controller := NewBaseController(config)
		impl := controller.(*baseControllerImpl)

		assert.True(t, impl.SupportsCommitterPattern())
	})
}

func TestWorkspaceKeyParsing(t *testing.T) {
	tests := map[string]struct {
		key               string
		expectedCluster   logicalcluster.Name
		expectedNamespace string
		expectedName      string
		expectError       bool
	}{
		"cluster-scoped resource": {
			key:               "root:org:workspace|resource-name",
			expectedCluster:   logicalcluster.Name("root:org:workspace"),
			expectedNamespace: "",
			expectedName:      "resource-name",
			expectError:       false,
		},
		"namespaced resource": {
			key:               "root:org:workspace|default/resource-name",
			expectedCluster:   logicalcluster.Name("root:org:workspace"),
			expectedNamespace: "default",
			expectedName:      "resource-name",
			expectError:       false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cluster, namespace, resourceName, err := ParseWorkspaceKey(tc.key)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedCluster, cluster)
				assert.Equal(t, tc.expectedNamespace, namespace)
				assert.Equal(t, tc.expectedName, resourceName)
			}
		})
	}
}

func TestWorkspaceKeyGeneration(t *testing.T) {
	cluster := logicalcluster.Name("root:org:workspace")

	t.Run("cluster-scoped key", func(t *testing.T) {
		key := MakeClusterScopedKey(cluster, "resource-name")
		expected := "root:org:workspace|resource-name"
		assert.Equal(t, expected, key)
	})

	t.Run("namespaced key", func(t *testing.T) {
		key := MakeNamespacedKey(cluster, "default", "resource-name")
		expected := "root:org:workspace|default/resource-name"
		assert.Equal(t, expected, key)
	})

	t.Run("generic workspace key - cluster scoped", func(t *testing.T) {
		key := MakeWorkspaceKey(cluster, "", "resource-name")
		expected := "root:org:workspace|resource-name"
		assert.Equal(t, expected, key)
	})

	t.Run("generic workspace key - namespaced", func(t *testing.T) {
		key := MakeWorkspaceKey(cluster, "kube-system", "resource-name")
		expected := "root:org:workspace|kube-system/resource-name"
		assert.Equal(t, expected, key)
	})
}

func TestBaseControllerHealthCheck(t *testing.T) {
	testRegistry := prometheus.NewRegistry()
	testMetrics := createTestMetrics(testRegistry)

	config := &BaseControllerConfig{
		Name:       "health-test-controller",
		Workspace:  logicalcluster.Name("test-workspace"),
		Reconciler: &testReconciler{},
		Metrics:    testMetrics,
	}

	controller := NewBaseController(config)

	// Before starting, controller should not be healthy
	assert.False(t, controller.IsHealthy())

	// Test that we can get access to internal methods
	impl := controller.(*baseControllerImpl)
	assert.NotNil(t, impl.GetQueue())
}
