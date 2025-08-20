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
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"
)

// ProcessorInterface defines the interface for processing work items from a queue.
// This follows KCP patterns for workspace-aware processing.
type ProcessorInterface interface {
	// ProcessItem handles processing a single work item.
	// The key follows KCP format: cluster|namespace/name or cluster|name
	ProcessItem(ctx context.Context, key string) error
}

// WorkerConfig contains configuration for workqueue management.
type WorkerConfig struct {
	// Name identifies this workqueue for logging and metrics
	Name string
	
	// Workspace provides workspace isolation
	Workspace logicalcluster.Name
	
	// WorkerCount controls concurrent processing goroutines
	WorkerCount int
	
	// Processor handles the actual work item processing
	Processor ProcessorInterface
	
	// RateLimiter controls retry behavior (optional - defaults to exponential backoff)
	RateLimiter workqueue.TypedRateLimiter[string]
}

// Manager manages a typed workqueue with KCP patterns.
// It provides worker pool management, error handling with exponential backoff,
// and proper shutdown coordination for TMC controllers.
type Manager struct {
	// Configuration
	name        string
	workspace   logicalcluster.Name
	workerCount int
	processor   ProcessorInterface
	
	// Queue management - uses KCP typed workqueue for type safety
	queue workqueue.TypedRateLimitingInterface[string]
	
	// Worker lifecycle
	mu       sync.RWMutex
	started  bool
	stopping bool
}

// NewManager creates a new workqueue manager with the given configuration.
// This provides KCP-compliant typed workqueue management with workspace isolation
// and proper error handling patterns.
func NewManager(config *WorkerConfig) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("WorkerConfig cannot be nil")
	}
	
	if config.Name == "" {
		return nil, fmt.Errorf("Name is required for workqueue identification")
	}
	
	if config.Workspace.Empty() {
		return nil, fmt.Errorf("Workspace cannot be empty - workspace isolation required")
	}
	
	if config.Processor == nil {
		return nil, fmt.Errorf("Processor cannot be nil - work item processing required")
	}
	
	if config.WorkerCount <= 0 {
		config.WorkerCount = 1 // Default to single worker
	}
	
	// Set up rate limiter with exponential backoff if not provided
	rateLimiter := config.RateLimiter
	if rateLimiter == nil {
		rateLimiter = workqueue.DefaultTypedControllerRateLimiter[string]()
	}
	
	// Create typed rate limiting queue
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		rateLimiter,
		workqueue.TypedRateLimitingQueueConfig[string]{
			Name: config.Name,
		},
	)
	
	return &Manager{
		name:        config.Name,
		workspace:   config.Workspace,
		workerCount: config.WorkerCount,
		processor:   config.Processor,
		queue:       queue,
	}, nil
}

// Start starts the workqueue manager and its worker goroutines.
// This method blocks until the context is cancelled.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("workqueue manager %s already started", m.name)
	}
	m.started = true
	m.mu.Unlock()
	
	defer utilruntime.HandleCrash()
	defer m.queue.ShutDown()
	
	klog.InfoS("Starting workqueue manager",
		"name", m.name,
		"workspace", m.workspace,
		"workers", m.workerCount)
	
	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < m.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			m.runWorker(ctx, workerID)
		}(i)
	}
	
	klog.InfoS("Workqueue manager started",
		"name", m.name,
		"workers", m.workerCount)
	
	// Block until context is cancelled
	<-ctx.Done()
	
	klog.InfoS("Shutting down workqueue manager", "name", m.name)
	
	// Mark as stopping
	m.mu.Lock()
	m.stopping = true
	m.mu.Unlock()
	
	// Wait for workers to finish
	wg.Wait()
	
	klog.InfoS("Workqueue manager stopped", "name", m.name)
	return nil
}

// Shutdown gracefully shuts down the workqueue manager.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	if !m.started || m.stopping {
		m.mu.Unlock()
		return
	}
	m.stopping = true
	m.mu.Unlock()
	
	klog.InfoS("Gracefully shutting down workqueue manager", "name", m.name)
	m.queue.ShutDown()
}

// Add adds a work item to the queue.
func (m *Manager) Add(key string) {
	m.queue.Add(key)
}

// AddAfter adds a work item to the queue after the specified delay.
func (m *Manager) AddAfter(key string, delay time.Duration) {
	m.queue.AddAfter(key, delay)
}

// AddRateLimited adds a work item with rate limiting.
func (m *Manager) AddRateLimited(key string) {
	m.queue.AddRateLimited(key)
}

// AddObject adds an object to the work queue using KCP key function.
// This ensures proper workspace isolation in the queue key.
func (m *Manager) AddObject(obj interface{}) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	m.queue.Add(key)
}

// Len returns the current length of the work queue.
func (m *Manager) Len() int {
	return m.queue.Len()
}

// IsShuttingDown returns true if the queue is shutting down.
func (m *Manager) IsShuttingDown() bool {
	return m.queue.ShuttingDown()
}

// runWorker processes work items from the queue until the context is done.
func (m *Manager) runWorker(ctx context.Context, workerID int) {
	klog.V(4).InfoS("Starting workqueue worker",
		"manager", m.name,
		"workspace", m.workspace,
		"worker", workerID)
	
	defer klog.V(4).InfoS("Stopping workqueue worker",
		"manager", m.name,
		"worker", workerID)
	
	for m.processNextWorkItem(ctx) {
		// Check if we should stop
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// processNextWorkItem processes a single work item from the queue.
// This implements the standard KCP workqueue processing pattern with
// typed queue support and proper error handling.
func (m *Manager) processNextWorkItem(ctx context.Context) bool {
	key, quit := m.queue.Get()
	if quit {
		return false
	}
	defer m.queue.Done(key)
	
	// Process the work item with workspace context
	err := m.processor.ProcessItem(ctx, key)
	
	if err == nil {
		// Success - forget the item and stop retrying
		m.queue.Forget(key)
		klog.V(6).InfoS("Successfully processed work item",
			"manager", m.name,
			"workspace", m.workspace,
			"key", key)
		return true
	}
	
	// Handle error with exponential backoff
	m.handleError(err, key)
	return true
}

// handleError handles processing errors with exponential backoff.
// This follows KCP patterns for error handling with workspace context.
func (m *Manager) handleError(err error, key string) {
	// Implement exponential backoff with workspace context logging
	if m.queue.NumRequeues(key) < 10 {
		klog.V(4).InfoS("Error processing work item, retrying",
			"manager", m.name,
			"workspace", m.workspace,
			"key", key,
			"error", err,
			"retries", m.queue.NumRequeues(key))
		
		m.queue.AddRateLimited(key)
		return
	}
	
	// Too many retries - drop the item
	klog.ErrorS(err, "Dropping work item after too many retries",
		"manager", m.name,
		"workspace", m.workspace,
		"key", key,
		"retries", m.queue.NumRequeues(key))
	
	m.queue.Forget(key)
	utilruntime.HandleError(err)
}