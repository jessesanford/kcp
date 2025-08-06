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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/util/workqueue"
)

func TestNewBaseController(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		metrics := createTestMetrics("test_new_controller_valid")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  5,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		assert.NotNil(t, controller)
		assert.Equal(t, "test-controller", controller.Name())
		// Controller should be unhealthy before starting (not started yet)
		assert.False(t, controller.IsHealthy())
	})

	t.Run("nil config panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewBaseController(nil)
		})
	})
}

// createTestMetrics creates isolated test metrics to avoid conflicts
func createTestMetrics(testName string) *ManagerMetrics {
	return &ManagerMetrics{
		controllersTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: testName + "_tmc_controllers_total",
				Help: "Total number of TMC controllers",
			},
		),
		controllersHealthy: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: testName + "_tmc_controllers_healthy",
				Help: "Number of healthy TMC controllers",
			},
		),
		reconcileTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: testName + "_tmc_reconcile_total",
				Help: "Total number of reconciliation attempts",
			},
			[]string{"controller", "result"},
		),
	}
}

func TestBaseController_Start_Lifecycle(t *testing.T) {
	t.Run("lifecycle state management", func(t *testing.T) {
		metrics := createTestMetrics("test_lifecycle_state")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)
		require.NotNil(t, controller)

		// Should be unhealthy when not started
		assert.False(t, controller.IsHealthy())
		
		// Should not be started or stopping initially
		baseImpl.mu.RLock()
		assert.False(t, baseImpl.started)
		assert.False(t, baseImpl.stopping)
		baseImpl.mu.RUnlock()

		// Simulate started state
		baseImpl.mu.Lock()
		baseImpl.started = true
		baseImpl.mu.Unlock()
		
		// Should be healthy when started
		assert.True(t, controller.IsHealthy())

		// Simulate stopping state
		baseImpl.mu.Lock()
		baseImpl.stopping = true
		baseImpl.mu.Unlock()
		
		// Should be unhealthy when stopping
		assert.False(t, controller.IsHealthy())
	})

	t.Run("already started error", func(t *testing.T) {
		metrics := createTestMetrics("test_already_started_error")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		// Manually mark as started to test error condition
		baseImpl.mu.Lock()
		baseImpl.started = true
		baseImpl.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Try starting - should get error
		err := controller.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already started")
	})
}

func TestBaseController_HealthChecking(t *testing.T) {
	t.Run("unhealthy by default (not started)", func(t *testing.T) {
		metrics := createTestMetrics("test_health_default")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		// Should be unhealthy when not started
		assert.False(t, controller.IsHealthy())
	})

	t.Run("unhealthy when not started", func(t *testing.T) {
		metrics := createTestMetrics("test_health_not_started")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		// Mark as not started
		baseImpl.mu.Lock()
		baseImpl.started = false
		baseImpl.mu.Unlock()

		assert.False(t, controller.IsHealthy())
	})

	t.Run("unhealthy when stopping", func(t *testing.T) {
		metrics := createTestMetrics("test_health_stopping")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		// Mark as stopping
		baseImpl.mu.Lock()
		baseImpl.started = true
		baseImpl.stopping = true
		baseImpl.mu.Unlock()

		assert.False(t, controller.IsHealthy())
	})
}

func TestBaseController_QueueManagement(t *testing.T) {
	t.Run("enqueue key", func(t *testing.T) {
		metrics := createTestMetrics("test_queue_enqueue_key")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		// Test enqueuing a key
		baseImpl.EnqueueKey("test-key")
		assert.Equal(t, 1, baseImpl.queue.Len())
		
		// Verify we can get it back
		key, shutdown := baseImpl.queue.Get()
		assert.False(t, shutdown)
		assert.Equal(t, "test-key", key)
		baseImpl.queue.Done(key)
	})

	t.Run("enqueue after", func(t *testing.T) {
		metrics := createTestMetrics("test_queue_enqueue_after")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		// Test enqueuing with delay
		baseImpl.EnqueueAfter("delayed-key", 10*time.Millisecond)
		
		// Should not be immediately available
		assert.Equal(t, 0, baseImpl.queue.Len())
		
		// Wait for delay and check again
		time.Sleep(20 * time.Millisecond)
		assert.Equal(t, 1, baseImpl.queue.Len())
	})

	t.Run("get queue", func(t *testing.T) {
		metrics := createTestMetrics("test_queue_get_queue")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		queue := baseImpl.GetQueue()
		assert.NotNil(t, queue)
		assert.IsType(t, workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "test"), queue)
	})
}

func TestBaseController_Shutdown(t *testing.T) {
	t.Run("graceful shutdown", func(t *testing.T) {
		metrics := createTestMetrics("test_shutdown_graceful")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  2,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		// Mark as started to enable shutdown
		baseImpl.mu.Lock()
		baseImpl.started = true
		baseImpl.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := controller.Shutdown(ctx)
		assert.NoError(t, err)

		// Should be marked as stopping
		baseImpl.mu.RLock()
		assert.True(t, baseImpl.stopping)
		baseImpl.mu.RUnlock()
	})

	t.Run("shutdown not started controller", func(t *testing.T) {
		metrics := createTestMetrics("test_shutdown_not_started")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Should succeed without error even when not started
		err := controller.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("shutdown already stopping", func(t *testing.T) {
		metrics := createTestMetrics("test_shutdown_already_stopping")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		// Mark as started and stopping
		baseImpl.mu.Lock()
		baseImpl.started = true
		baseImpl.stopping = true
		baseImpl.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Should succeed without error
		err := controller.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

func TestBaseController_ErrorHandling(t *testing.T) {
	t.Run("handle error with retries", func(t *testing.T) {
		metrics := createTestMetrics("test_error_handle_retries")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		testError := errors.New("test error")
		testKey := "test-key"

		// Add item to queue
		baseImpl.queue.Add(testKey)

		// Handle error - should requeue
		baseImpl.handleError(testError, testKey)

		// Should still have item in queue (requeued)
		assert.Equal(t, 1, baseImpl.queue.Len())
		assert.Equal(t, 1, baseImpl.queue.NumRequeues(testKey))
	})

	t.Run("handle error too many retries", func(t *testing.T) {
		metrics := createTestMetrics("test_error_too_many_retries")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  1,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		testError := errors.New("persistent test error")
		testKey := "persistent-key"

		// Add item to queue and simulate many retries
		baseImpl.queue.Add(testKey)
		for i := 0; i < 10; i++ {
			baseImpl.queue.AddRateLimited(testKey)
		}

		// Handle error - should drop item and mark unhealthy
		baseImpl.handleError(testError, testKey)

		// Should be marked as unhealthy
		baseImpl.mu.RLock()
		isHealthy := baseImpl.healthy
		baseImpl.mu.RUnlock()
		assert.False(t, isHealthy)
	})
}

func TestBaseController_ConcurrentWorkers(t *testing.T) {
	t.Run("multiple workers configuration", func(t *testing.T) {
		metrics := createTestMetrics("test_concurrent_workers")
		config := &BaseControllerConfig{
			Name:         "test-controller",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  3,
			Metrics:      metrics,
		}

		controller := NewBaseController(config)
		baseImpl := controller.(*baseControllerImpl)

		// Verify worker count configuration
		assert.Equal(t, 3, baseImpl.workerCount)

		// Add items to queue to test queue management
		for i := 0; i < 5; i++ {
			baseImpl.EnqueueKey(fmt.Sprintf("item-%d", i))
		}

		// Verify items were added
		assert.Equal(t, 5, baseImpl.queue.Len())
		
		// Test queue operations
		key, shutdown := baseImpl.queue.Get()
		assert.False(t, shutdown)
		assert.Equal(t, "item-0", key)
		baseImpl.queue.Done(key)
		
		// Should have 4 items left
		assert.Equal(t, 4, baseImpl.queue.Len())
	})
}