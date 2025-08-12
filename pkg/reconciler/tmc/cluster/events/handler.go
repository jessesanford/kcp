/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// ClusterEventInfo represents information about a cluster event.
type ClusterEventInfo struct {
	// ClusterName is the name of the cluster the event relates to.
	ClusterName string

	// EventType is the type of the event.
	EventType EventType

	// Reason is the reason for the event.
	Reason EventReason

	// Message is the human-readable message.
	Message string

	// Object is the Kubernetes object associated with the event.
	Object runtime.Object

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// Workspace is the logical cluster context.
	Workspace logicalcluster.Name
}

// ClusterEventHandler is responsible for handling and processing cluster events.
// It provides a queue-based system for processing events asynchronously while
// maintaining proper workspace isolation and following KCP patterns.
type ClusterEventHandler struct {
	// workqueue is the queue for processing events.
	workqueue workqueue.RateLimitingInterface

	// recorder is the event recorder for recording processed events.
	recorder *ClusterEventRecorder

	// handlers is a map of event handlers by event type.
	handlers map[EventType][]EventHandlerFunc

	// workspace represents the logical cluster context.
	workspace logicalcluster.Name

	// logger provides structured logging for event handling operations.
	logger klog.Logger

	// mutex protects handlers map from concurrent access.
	mutex sync.RWMutex

	// shutdown is used to signal shutdown to the event processing loop.
	shutdown chan struct{}

	// shutdownOnce ensures shutdown is only called once.
	shutdownOnce sync.Once
}

// EventHandlerFunc is a function that handles a specific cluster event.
type EventHandlerFunc func(ctx context.Context, eventInfo *ClusterEventInfo) error

// NewClusterEventHandler creates a new ClusterEventHandler for the specified workspace.
//
// Parameters:
//   - recorder: ClusterEventRecorder for recording events
//   - workspace: Logical cluster name for workspace isolation
//   - logger: Structured logger for event handling operations
//
// Returns:
//   - *ClusterEventHandler: Configured event handler ready to start processing
func NewClusterEventHandler(
	recorder *ClusterEventRecorder,
	workspace logicalcluster.Name,
	logger klog.Logger,
) *ClusterEventHandler {
	return &ClusterEventHandler{
		workqueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"cluster-events",
		),
		recorder:  recorder,
		handlers:  make(map[EventType][]EventHandlerFunc),
		workspace: workspace,
		logger:    logger.WithName("cluster-event-handler").WithValues("workspace", workspace),
		shutdown:  make(chan struct{}),
	}
}

// RegisterEventHandler registers a handler function for a specific event type.
//
// Parameters:
//   - eventType: The type of event to handle
//   - handler: The handler function to register
func (h *ClusterEventHandler) RegisterEventHandler(eventType EventType, handler EventHandlerFunc) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.handlers[eventType] == nil {
		h.handlers[eventType] = make([]EventHandlerFunc, 0)
	}
	h.handlers[eventType] = append(h.handlers[eventType], handler)

	h.logger.V(4).Info("Registered event handler",
		"eventType", eventType,
		"totalHandlers", len(h.handlers[eventType]),
	)
}

// HandleClusterEvent queues a cluster event for processing.
//
// Parameters:
//   - ctx: Context for the operation
//   - clusterName: Name of the cluster the event relates to
//   - eventType: Type of the event
//   - reason: Reason for the event
//   - message: Human-readable message
//   - obj: Kubernetes object associated with the event
func (h *ClusterEventHandler) HandleClusterEvent(
	ctx context.Context,
	clusterName string,
	eventType EventType,
	reason EventReason,
	message string,
	obj runtime.Object,
) {
	eventInfo := &ClusterEventInfo{
		ClusterName: clusterName,
		EventType:   eventType,
		Reason:      reason,
		Message:     message,
		Object:      obj,
		Timestamp:   time.Now(),
		Workspace:   h.workspace,
	}

	// Generate a unique key for the event
	key := fmt.Sprintf("%s/%s/%s/%d", 
		h.workspace, clusterName, eventType, eventInfo.Timestamp.Unix())

	h.workqueue.Add(key)
	h.logger.V(4).Info("Queued cluster event for processing",
		"key", key,
		"clusterName", clusterName,
		"eventType", eventType,
		"reason", reason,
	)
}

// Start begins processing events from the queue.
//
// Parameters:
//   - ctx: Context for the operation
//   - workers: Number of worker goroutines to start
func (h *ClusterEventHandler) Start(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer h.workqueue.ShutDown()

	h.logger.Info("Starting cluster event handler", "workers", workers)

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, h.runWorker, time.Second)
	}

	h.logger.Info("Started cluster event handler workers")
	<-ctx.Done()
	h.logger.Info("Shutting down cluster event handler")
}

// Shutdown gracefully shuts down the event handler.
func (h *ClusterEventHandler) Shutdown() {
	h.shutdownOnce.Do(func() {
		close(h.shutdown)
		h.workqueue.ShutDown()
		h.logger.Info("Cluster event handler shutdown completed")
	})
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the workqueue.
func (h *ClusterEventHandler) runWorker(ctx context.Context) {
	for h.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it by calling the processEvent method.
func (h *ClusterEventHandler) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := h.workqueue.Get()
	if shutdown {
		return false
	}

	defer h.workqueue.Done(obj)

	key, ok := obj.(string)
	if !ok {
		h.workqueue.Forget(obj)
		utilruntime.HandleError(fmt.Errorf("expected string key, got %T", obj))
		return true
	}

	if err := h.processEventKey(ctx, key); err != nil {
		// Put the item back on the workqueue to handle any transient errors.
		h.workqueue.AddRateLimited(key)
		utilruntime.HandleError(fmt.Errorf("error processing key '%s': %w", key, err))
		return true
	}

	// Finally, if no error occurs we Forget this item so it does not
	// get queued again until another change happens.
	h.workqueue.Forget(obj)
	return true
}

// processEventKey processes a single event key from the workqueue.
func (h *ClusterEventHandler) processEventKey(ctx context.Context, key string) error {
	h.logger.V(4).Info("Processing event key", "key", key)
	// Simplified processing for now
	return nil
}

// getEventHandlers returns the registered handlers for a given event type.
func (h *ClusterEventHandler) getEventHandlers(eventType EventType) []EventHandlerFunc {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	handlers := h.handlers[eventType]
	if handlers == nil {
		return []EventHandlerFunc{}
	}
	return handlers
}

// GetQueueLength returns the current length of the event processing queue.
func (h *ClusterEventHandler) GetQueueLength() int {
	return h.workqueue.Len()
}

// GetWorkspace returns the workspace this event handler is associated with.
func (h *ClusterEventHandler) GetWorkspace() logicalcluster.Name {
	return h.workspace
}

// WithLogger returns a new ClusterEventHandler with the specified logger.
func (h *ClusterEventHandler) WithLogger(logger klog.Logger) *ClusterEventHandler {
	return &ClusterEventHandler{
		workqueue: h.workqueue,
		recorder:  h.recorder,
		handlers:  h.handlers,
		workspace: h.workspace,
		logger:    logger.WithName("cluster-event-handler").WithValues("workspace", h.workspace),
		shutdown:  h.shutdown,
		mutex:     h.mutex,
	}
}