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

package priority

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPriorityQueue(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &PriorityConfig{
			MaxRetries: 5,
			RetryDelay: 2 * time.Second,
		}
		
		queue := NewPriorityQueue("test-queue", config)
		require.NotNil(t, queue)
		assert.Equal(t, 0, queue.Len())
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		queue := NewPriorityQueue("test-queue", nil)
		require.NotNil(t, queue)
		assert.Equal(t, 0, queue.Len())
	})
}

func TestPriorityQueueAddWithPriority(t *testing.T) {
	queue := NewPriorityQueue("test-queue", nil)
	defer queue.ShutDown()

	// Add items with different priorities
	queue.AddWithPriority("low-item", Low)
	queue.AddWithPriority("high-item", High)
	queue.AddWithPriority("critical-item", Critical)

	assert.Equal(t, 3, queue.Len())

	// Check priorities are stored correctly
	priority, exists := queue.GetPriority("low-item")
	assert.True(t, exists)
	assert.Equal(t, Low, priority)

	priority, exists = queue.GetPriority("high-item")
	assert.True(t, exists)
	assert.Equal(t, High, priority)

	priority, exists = queue.GetPriority("critical-item")
	assert.True(t, exists)
	assert.Equal(t, Critical, priority)
}

func TestPriorityQueueOrdering(t *testing.T) {
	queue := NewPriorityQueue("test-queue", nil)
	defer queue.ShutDown()

	// Add items in reverse priority order
	queue.AddWithPriority("low-item", Low)
	queue.AddWithPriority("normal-item", Normal)
	queue.AddWithPriority("high-item", High)
	queue.AddWithPriority("critical-item", Critical)

	// Items should come out in priority order (highest first)
	expectedOrder := []string{"critical-item", "high-item", "normal-item", "low-item"}
	
	for _, expectedKey := range expectedOrder {
		key, quit := queue.Get()
		require.False(t, quit)
		assert.Equal(t, expectedKey, key)
		queue.Done(key)
	}

	assert.Equal(t, 0, queue.Len())
}

func TestPriorityQueueBasicOperations(t *testing.T) {
	queue := NewPriorityQueue("test-queue", nil)
	defer queue.ShutDown()

	// Test basic priority operations
	queue.AddWithPriority("test-item", Low)
	
	priority, exists := queue.GetPriority("test-item")
	assert.True(t, exists)
	assert.Equal(t, Low, priority)

	// Update priority
	updated := queue.UpdatePriority("test-item", High)
	assert.True(t, updated)

	priority, exists = queue.GetPriority("test-item")
	assert.True(t, exists)
	assert.Equal(t, High, priority)

	assert.Equal(t, 1, queue.Len())
}

func TestPriorityHeap(t *testing.T) {
	h := &priorityHeap{}

	// Add items with different priorities
	items := []*PriorityItem{
		NewPriorityItem("low", Low),
		NewPriorityItem("high", High),
		NewPriorityItem("critical", Critical),
		NewPriorityItem("normal", Normal),
	}

	// Push items
	for _, item := range items {
		h.Push(item)
	}

	assert.Equal(t, 4, h.Len())

	// Items should come out in priority order (highest first)
	expectedPriorities := []Priority{Critical, High, Normal, Low}
	
	for i, expectedPriority := range expectedPriorities {
		item := h.Pop().(*PriorityItem)
		assert.Equal(t, expectedPriority, item.Priority, "Item %d should have priority %v", i, expectedPriority)
	}

	assert.Equal(t, 0, h.Len())
}

// Test edge case for empty queue
func TestPriorityQueueEmpty(t *testing.T) {
	queue := NewPriorityQueue("test-queue", nil)
	defer queue.ShutDown()

	// Get from empty queue should return quit=true
	_, quit := queue.Get()
	assert.True(t, quit)

	// Len should be 0
	assert.Equal(t, 0, queue.Len())
}