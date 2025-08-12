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
	"fmt"
	"time"

	"k8s.io/client-go/util/workqueue"
)

// Priority represents the priority level for workqueue items.
// Higher numerical values indicate higher priority.
type Priority int

const (
	// Critical priority - used for items that must be processed immediately
	// Examples: Security violations, system health emergencies
	Critical Priority = 1000
	
	// High priority - important operations that should be processed quickly
	// Examples: Resource cleanup, important status updates
	High Priority = 750
	
	// Normal priority - standard operations
	// Examples: Regular reconciliation, routine updates
	Normal Priority = 500
	
	// Low priority - background operations that can be delayed
	// Examples: Metrics collection, non-urgent cleanup
	Low Priority = 250
	
	// Background priority - lowest priority operations
	// Examples: Periodic maintenance, statistical analysis
	Background Priority = 100
)

// String returns the string representation of the priority level.
func (p Priority) String() string {
	switch p {
	case Critical:
		return "Critical"
	case High:
		return "High"
	case Normal:
		return "Normal"
	case Low:
		return "Low"
	case Background:
		return "Background"
	default:
		return fmt.Sprintf("Priority(%d)", int(p))
	}
}

// IsValid returns true if the priority is within valid bounds.
func (p Priority) IsValid() bool {
	return p >= Background && p <= Critical
}

// PriorityItem represents an item in the priority queue with its priority level.
type PriorityItem struct {
	// Key is the workqueue key for the item
	Key string
	
	// Priority is the priority level for this item
	Priority Priority
	
	// Timestamp when the item was added to the queue
	AddedAt time.Time
	
	// RetryCount tracks how many times this item has been retried
	RetryCount int
	
	// heapIndex is the index of the item in the heap (internal use only)
	heapIndex int
}

// NewPriorityItem creates a new priority item with the specified key and priority.
func NewPriorityItem(key string, priority Priority) *PriorityItem {
	return &PriorityItem{
		Key:        key,
		Priority:   priority,
		AddedAt:    time.Now(),
		RetryCount: 0,
	}
}

// Age returns how long this item has been in the queue.
func (pi *PriorityItem) Age() time.Duration {
	return time.Since(pi.AddedAt)
}

// EffectivePriority calculates the effective priority considering age and retry count.
// Items that have been waiting longer or have failed multiple times get priority boosts.
func (pi *PriorityItem) EffectivePriority() Priority {
	basePriority := int(pi.Priority)
	
	// Age boost: items waiting longer than 30 seconds get priority boost
	age := pi.Age()
	if age > 30*time.Second {
		ageBoost := int(age.Seconds() / 30)
		basePriority += ageBoost * 10 // 10 points per 30 second interval
	}
	
	// Retry boost: failed items get higher priority to prevent starvation
	retryBoost := pi.RetryCount * 25 // 25 points per retry
	basePriority += retryBoost
	
	effectivePriority := Priority(basePriority)
	
	// Cap at Critical priority
	if effectivePriority > Critical {
		effectivePriority = Critical
	}
	
	return effectivePriority
}

// PriorityConfig contains configuration for priority-based workqueue behavior.
type PriorityConfig struct {
	// MaxRetries is the maximum number of times to retry a failed item
	MaxRetries int
	
	// RetryDelay is the base delay for retrying failed items
	RetryDelay time.Duration
	
	// MaxDelay is the maximum delay for retrying failed items
	MaxDelay time.Duration
	
	// PriorityBoostInterval controls how often to boost priority for waiting items
	PriorityBoostInterval time.Duration
	
	// StarvationThreshold is the maximum time an item can wait before getting priority boost
	StarvationThreshold time.Duration
}

// DefaultPriorityConfig returns a default configuration for priority queues.
func DefaultPriorityConfig() *PriorityConfig {
	return &PriorityConfig{
		MaxRetries:            10,
		RetryDelay:            5 * time.Second,
		MaxDelay:              5 * time.Minute,
		PriorityBoostInterval: 30 * time.Second,
		StarvationThreshold:   2 * time.Minute,
	}
}

// PriorityQueue defines the interface for a priority-based workqueue
// that extends the standard KCP TypedRateLimitingInterface.
type PriorityQueue interface {
	workqueue.TypedRateLimitingInterface[string]
	
	// AddWithPriority adds an item to the queue with the specified priority
	AddWithPriority(key string, priority Priority)
	
	// GetPriority returns the current priority for an item in the queue
	GetPriority(key string) (Priority, bool)
	
	// UpdatePriority updates the priority of an item already in the queue
	UpdatePriority(key string, priority Priority) bool
	
	// Len returns the number of items in the queue
	Len() int
	
	// GetMetrics returns queue metrics for observability
	GetMetrics() PriorityQueueMetrics
}

// PriorityQueueMetrics contains metrics about the priority queue state.
type PriorityQueueMetrics struct {
	// TotalItems is the total number of items in the queue
	TotalItems int
	
	// ItemsByPriority breaks down items by priority level
	ItemsByPriority map[Priority]int
	
	// OldestItem is the age of the oldest item in the queue
	OldestItem time.Duration
	
	// AverageWaitTime is the average time items spend in the queue
	AverageWaitTime time.Duration
}