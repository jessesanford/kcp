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

package workqueue

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestTMCWorkqueue_AddAndGetWithWorkspace(t *testing.T) {
	q := NewDefaultTMCWorkqueue("test-queue")
	defer q.ShutDown()

	workspace := logicalcluster.Name("root:test")
	name := "test-object"

	// Add item to workqueue
	q.AddWorkspace(workspace, name)

	// Get item from workqueue
	gotWorkspace, gotName, shutdown := q.GetWithWorkspace()
	require.False(t, shutdown, "workqueue should not be shutdown")
	assert.Equal(t, workspace, gotWorkspace, "workspace should match")
	assert.Equal(t, name, gotName, "name should match")

	// Mark item as done
	q.Done(makeWorkspaceKey(workspace, name))

	// Verify queue is empty
	assert.Equal(t, 0, q.Len(), "queue should be empty after processing")
}

func TestTMCWorkqueue_Name(t *testing.T) {
	queueName := "test-named-queue"
	q := NewDefaultTMCWorkqueue(queueName)
	defer q.ShutDown()

	assert.Equal(t, queueName, q.Name(), "queue name should match")
}

func TestTMCWorkqueue_Metrics(t *testing.T) {
	q := NewDefaultTMCWorkqueue("test-metrics-queue")
	defer q.ShutDown()

	workspace := logicalcluster.Name("root:metrics")
	name := "metrics-object"

	// Initially empty
	metrics := q.Metrics()
	assert.Equal(t, 0, metrics.Depth, "queue depth should be 0 initially")

	// Add item and check depth
	q.AddWorkspace(workspace, name)
	metrics = q.Metrics()
	assert.Equal(t, 1, metrics.Depth, "queue depth should be 1 after adding item")

	// Get item and mark done
	gotWorkspace, gotName, shutdown := q.GetWithWorkspace()
	require.False(t, shutdown)
	assert.Equal(t, workspace, gotWorkspace)
	assert.Equal(t, name, gotName)
	q.Done(makeWorkspaceKey(workspace, name))

	// Verify queue is empty
	metrics = q.Metrics()
	assert.Equal(t, 0, metrics.Depth, "queue depth should be 0 after processing")
}

func TestMakeAndParseWorkspaceKey(t *testing.T) {
	tests := map[string]struct {
		workspace     logicalcluster.Name
		name          string
		expectedKey   string
		shouldSucceed bool
	}{
		"valid workspace and name": {
			workspace:     logicalcluster.Name("root:test"),
			name:          "test-object",
			expectedKey:   "root:test|test-object",
			shouldSucceed: true,
		},
		"workspace with special chars": {
			workspace:     logicalcluster.Name("root:test:sub"),
			name:          "object-with-dash",
			expectedKey:   "root:test:sub|object-with-dash",
			shouldSucceed: true,
		},
		"empty workspace": {
			workspace:     logicalcluster.Name(""),
			name:          "object",
			expectedKey:   "|object",
			shouldSucceed: false,
		},
		"empty name": {
			workspace:     logicalcluster.Name("root:test"),
			name:          "",
			expectedKey:   "root:test|",
			shouldSucceed: false,
		},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			// Test key creation
			key := makeWorkspaceKey(tc.workspace, tc.name)
			assert.Equal(t, tc.expectedKey, key, "generated key should match expected")

			// Test key parsing
			parsedWorkspace, parsedName, err := parseWorkspaceKey(key)
			if tc.shouldSucceed {
				require.NoError(t, err, "parsing should succeed for valid key")
				assert.Equal(t, tc.workspace, parsedWorkspace, "parsed workspace should match")
				assert.Equal(t, tc.name, parsedName, "parsed name should match")
			} else {
				require.Error(t, err, "parsing should fail for invalid key")
			}
		})
	}
}

func TestWorkerPool_ProcessItems(t *testing.T) {
	q := NewDefaultTMCWorkqueue("test-worker-pool")
	defer q.ShutDown()

	// Track processed items
	var processedItems []string
	var mu sync.Mutex

	handler := WorkerHandlerFunc(func(ctx context.Context, workspace logicalcluster.Name, name string) error {
		mu.Lock()
		defer mu.Unlock()
		processedItems = append(processedItems, fmt.Sprintf("%s|%s", workspace, name))
		return nil
	})

	wp := NewWorkerPool("test-pool", q, handler, 2)

	// Add test items
	items := []struct {
		workspace logicalcluster.Name
		name      string
	}{
		{logicalcluster.Name("root:test1"), "object1"},
		{logicalcluster.Name("root:test2"), "object2"},
		{logicalcluster.Name("root:test3"), "object3"},
	}

	for _, item := range items {
		q.AddWorkspace(item.workspace, item.name)
	}

	// Start worker pool with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- wp.Start(ctx)
	}()

	// Wait for items to be processed
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(processedItems) == len(items)
	}, 3*time.Second, 100*time.Millisecond, "all items should be processed")

	// Cancel context to stop workers
	cancel()

	// Wait for worker pool to finish
	select {
	case err := <-done:
		assert.NoError(t, err, "worker pool should shut down cleanly")
	case <-time.After(2 * time.Second):
		t.Fatal("worker pool did not shut down in time")
	}

	// Verify all items were processed
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, processedItems, len(items), "all items should be processed")

	expectedItems := make([]string, len(items))
	for i, item := range items {
		expectedItems[i] = fmt.Sprintf("%s|%s", item.workspace, item.name)
	}

	assert.ElementsMatch(t, expectedItems, processedItems, "processed items should match expected")
}

func TestWorkerPool_ErrorHandling(t *testing.T) {
	q := NewDefaultTMCWorkqueue("test-error-pool")
	defer q.ShutDown()

	// Handler that returns error for specific item
	var processCount int
	var mu sync.Mutex

	handler := WorkerHandlerFunc(func(ctx context.Context, workspace logicalcluster.Name, name string) error {
		mu.Lock()
		defer mu.Unlock()
		processCount++
		
		if name == "error-object" {
			return fmt.Errorf("test error")
		}
		return nil
	})

	wp := NewWorkerPool("test-error-pool", q, handler, 1)

	// Add items including one that will error
	q.AddWorkspace(logicalcluster.Name("root:test"), "good-object")
	q.AddWorkspace(logicalcluster.Name("root:test"), "error-object")

	// Start worker pool with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- wp.Start(ctx)
	}()

	// Wait for processing to complete (good item + several retries of error item)
	time.Sleep(1 * time.Second)

	// Cancel to stop workers
	cancel()

	// Wait for shutdown
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker pool did not shut down in time")
	}

	// Verify error item was retried multiple times
	mu.Lock()
	defer mu.Unlock()
	assert.Greater(t, processCount, 2, "error item should have been retried")
}

func TestWorkerPool_Shutdown(t *testing.T) {
	q := NewDefaultTMCWorkqueue("test-shutdown-pool")
	defer q.ShutDown()

	handler := WorkerHandlerFunc(func(ctx context.Context, workspace logicalcluster.Name, name string) error {
		// Simulate work
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	wp := NewWorkerPool("test-shutdown-pool", q, handler, 1)

	// Initially not shutdown
	assert.False(t, wp.IsShutdown(), "worker pool should not be shutdown initially")

	// Start and immediately cancel
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := wp.Start(ctx)
	assert.NoError(t, err, "shutdown should not return error")
	assert.True(t, wp.IsShutdown(), "worker pool should be shutdown after context cancel")
}