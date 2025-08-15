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

// testReconciler implements Reconciler for testing
type testReconciler struct {
	reconcileFunc func(ctx context.Context, key string) error
}

func (t *testReconciler) Reconcile(ctx context.Context, key string) error {
	if t.reconcileFunc != nil {
		return t.reconcileFunc(ctx, key)
	}
	return nil
}

// createTestMetrics creates test-specific metrics with unique registry
func createTestMetrics(registry *prometheus.Registry) *ManagerMetrics {
	metrics := NewManagerMetrics()
	registry.MustRegister(metrics.reconcileTotal)
	return metrics
}

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

func TestBaseController_Lifecycle(t *testing.T) {
	t.Run("controller starts and stops correctly", func(t *testing.T) {
		testRegistry := prometheus.NewRegistry()
		testMetrics := createTestMetrics(testRegistry)
		
		reconciled := make(chan string, 1)
		reconciler := &testReconciler{
			reconcileFunc: func(ctx context.Context, key string) error {
				reconciled <- key
				return nil
			},
		}

		config := &BaseControllerConfig{
			Name:        "test-controller",
			Workspace:   logicalcluster.Name("test-workspace"),
			WorkerCount: 1,
			Reconciler:  reconciler,
			Metrics:     testMetrics,
		}

		controller := NewBaseController(config)
		impl := controller.(*baseControllerImpl)
		
		// Test healthy state before start
		assert.True(t, impl.healthy)
		assert.False(t, impl.started)
		
		// Test enqueue functionality
		impl.EnqueueKey("test-key")
		assert.Equal(t, 1, impl.GetQueue().Len())

		// Controller should start and stop without error
		ctx, cancel := context.WithCancel(context.Background())
		
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		
		err := controller.Start(ctx)
		assert.NoError(t, err)
	})

	t.Run("cannot start already started controller", func(t *testing.T) {
		testRegistry := prometheus.NewRegistry()
		testMetrics := createTestMetrics(testRegistry)

		config := &BaseControllerConfig{
			Name:        "test-controller",
			Workspace:   logicalcluster.Name("test-workspace"),
			WorkerCount: 1,
			Reconciler:  &testReconciler{},
			Metrics:     testMetrics,
		}

		controller := NewBaseController(config)
		impl := controller.(*baseControllerImpl)
		
		// Mark as started
		impl.started = true
		
		err := controller.Start(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already started")
	})
}

func TestParseWorkspaceKey(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		wantCluster   logicalcluster.Name
		wantNamespace string
		wantName      string
		wantError     bool
	}{
		{
			name:          "cluster-scoped resource",
			key:           "test-cluster|resource-name",
			wantCluster:   logicalcluster.Name("test-cluster"),
			wantNamespace: "",
			wantName:      "resource-name",
			wantError:     false,
		},
		{
			name:          "namespaced resource",
			key:           "test-cluster|namespace/resource-name",
			wantCluster:   logicalcluster.Name("test-cluster"),
			wantNamespace: "namespace",
			wantName:      "resource-name",
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, namespace, name, err := ParseWorkspaceKey(tt.key)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			
			assert.NoError(t, err)
			assert.Equal(t, tt.wantCluster, cluster)
			assert.Equal(t, tt.wantNamespace, namespace)
			assert.Equal(t, tt.wantName, name)
		})
	}
}

func TestMakeWorkspaceKey(t *testing.T) {
	tests := []struct {
		name      string
		cluster   logicalcluster.Name
		namespace string
		resource  string
		wantKey   string
	}{
		{
			name:      "cluster-scoped key",
			cluster:   logicalcluster.Name("test-cluster"),
			namespace: "",
			resource:  "resource-name",
			wantKey:   "test-cluster|resource-name",
		},
		{
			name:      "namespaced key",
			cluster:   logicalcluster.Name("test-cluster"),
			namespace: "namespace",
			resource:  "resource-name",
			wantKey:   "test-cluster|namespace/resource-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := MakeWorkspaceKey(tt.cluster, tt.namespace, tt.resource)
			assert.Equal(t, tt.wantKey, key)
		})
	}
}