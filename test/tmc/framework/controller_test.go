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

package framework

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/kcp-dev/logicalcluster/v3"
)

// TestMetrics provides test-specific metrics for TMC controllers
type TestMetrics struct {
	controllersTotal   prometheus.Gauge
	controllersHealthy prometheus.Gauge
	reconcileTotal     *prometheus.CounterVec
	reconcileDuration  *prometheus.HistogramVec
}

// NewTestMetrics creates metrics for testing TMC controllers
func NewTestMetrics() *TestMetrics {
	return &TestMetrics{
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
		reconcileDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "test_tmc_reconcile_duration_seconds",
				Help: "Time spent in reconciliation",
			},
			[]string{"controller"},
		),
	}
}

// TestControllerConfig holds configuration for testing TMC controllers
type TestControllerConfig struct {
	Name      string
	Workspace logicalcluster.Name
	Queue     workqueue.RateLimitingInterface
	Metrics   *TestMetrics
	Context   context.Context
}

// NewTestControllerConfig creates a standard test configuration
func NewTestControllerConfig(name string) *TestControllerConfig {
	return &TestControllerConfig{
		Name:      name,
		Workspace: logicalcluster.Name("root:test"),
		Queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		Metrics:   NewTestMetrics(),
		Context:   context.Background(),
	}
}

// ValidateControllerBasics performs common validation for TMC controllers
func ValidateControllerBasics(t *testing.T, name string, workspace logicalcluster.Name) {
	t.Helper()
	
	assert.NotEmpty(t, name, "controller name should not be empty")
	assert.NotEmpty(t, workspace.String(), "workspace should not be empty")
	assert.NotEqual(t, logicalcluster.Name(""), workspace, "workspace should be valid")
}

// ValidateWorkspaceIsolation verifies controller properly isolates workspaces
func ValidateWorkspaceIsolation(t *testing.T, expectedWorkspace logicalcluster.Name, actualWorkspace logicalcluster.Name) {
	t.Helper()
	
	assert.Equal(t, expectedWorkspace, actualWorkspace,
		"controller should maintain workspace isolation")
}

// TestReconcilerMock provides a mock reconciler for testing
type TestReconcilerMock struct {
	ReconcileFn func(ctx context.Context, key string) error
	CallCount   int
	LastKey     string
}

// Reconcile implements the basic reconciler pattern
func (m *TestReconcilerMock) Reconcile(ctx context.Context, key string) error {
	m.CallCount++
	m.LastKey = key
	if m.ReconcileFn != nil {
		return m.ReconcileFn(ctx, key)
	}
	return nil
}

// AssertReconcilerCalled verifies the reconciler was called as expected
func (m *TestReconcilerMock) AssertReconcilerCalled(t *testing.T, expectedCount int, expectedKey string) {
	t.Helper()
	
	assert.Equal(t, expectedCount, m.CallCount, "reconciler call count mismatch")
	if expectedKey != "" {
		assert.Equal(t, expectedKey, m.LastKey, "reconciler key mismatch")
	}
}

// WaitForCacheSync waits for cache synchronization in tests
func WaitForCacheSync(ctx context.Context, t *testing.T, name string, informer cache.SharedIndexInformer) {
	t.Helper()
	
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		t.Fatalf("failed to wait for %s cache sync", name)
	}
}