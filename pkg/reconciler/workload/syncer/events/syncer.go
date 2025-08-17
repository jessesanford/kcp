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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	kcpclientset "github.com/kcp-dev/client-go/kubernetes"
	"github.com/kcp-dev/logicalcluster/v3"
)

// EventSyncer synchronizes events from downstream physical clusters to KCP.
// It provides filtering, aggregation, and deduplication of events to reduce
// noise while ensuring important events are visible in KCP workspaces.
type EventSyncer struct {
	kcpClient        kcpclientset.ClusterInterface
	downstreamClient kubernetes.Interface

	// Sync target information
	syncTargetName string
	workspace      logicalcluster.Name

	// Event processing
	filter     *EventFilter
	aggregator *EventAggregator
	config     EventSyncConfig

	// Deduplication tracking
	seenEvents map[string]time.Time
	mu         sync.RWMutex

	// Internal state
	stopCh chan struct{}
}

// NewEventSyncer creates a new event syncer for the given sync target.
// It configures the syncer to watch events from the downstream cluster
// and sync them to the appropriate workspace in KCP.
func NewEventSyncer(
	kcpClient kcpclientset.ClusterInterface,
	downstreamClient kubernetes.Interface,
	syncTargetName string,
	workspace logicalcluster.Name,
	config EventSyncConfig,
) *EventSyncer {
	return &EventSyncer{
		kcpClient:        kcpClient,
		downstreamClient: downstreamClient,
		syncTargetName:   syncTargetName,
		workspace:        workspace,
		filter:           NewEventFilter(config),
		aggregator:       NewEventAggregator(config),
		config:           config,
		seenEvents:       make(map[string]time.Time),
		stopCh:           make(chan struct{}),
	}
}

// Start begins watching and syncing events from the downstream cluster.
// It sets up event informers and starts the processing loops.
func (s *EventSyncer) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithName("event-syncer").WithValues(
		"syncTarget", s.syncTargetName,
		"workspace", s.workspace,
	)
	logger.Info("Starting event syncer")

	// Create event informer for all namespaces
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return s.downstreamClient.CoreV1().Events(metav1.NamespaceAll).List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return s.downstreamClient.CoreV1().Events(metav1.NamespaceAll).Watch(ctx, options)
			},
		},
		&corev1.Event{},
		30*time.Second, // Resync period
	)

	// Add event handler that processes new and updated events
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if event, ok := obj.(*corev1.Event); ok {
				s.handleEvent(ctx, event)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			if event, ok := new.(*corev1.Event); ok {
				s.handleEvent(ctx, event)
			}
		},
		// Note: We don't handle delete events as events are time-based
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	// Start the informer
	go informer.Run(ctx.Done())

	// Wait for informer cache to sync
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("failed to sync event informer cache")
	}

	// Start the cleanup loop for old seen events
	go s.cleanupLoop(ctx)

	// Start the aggregator flush loop
	go s.aggregator.StartFlushLoop(ctx, s.syncAggregatedEvents)

	logger.Info("Event syncer started successfully")
	return nil
}

// Stop gracefully stops the event syncer
func (s *EventSyncer) Stop() {
	close(s.stopCh)
}

// handleEvent processes a single event from the downstream cluster.
// It applies filtering, deduplication, and aggregation before syncing to KCP.
func (s *EventSyncer) handleEvent(ctx context.Context, event *corev1.Event) {
	logger := klog.FromContext(ctx).WithName("event-handler").WithValues(
		"event", event.Name,
		"namespace", event.Namespace,
		"reason", event.Reason,
	)

	// Apply filtering to reduce noise
	if !s.filter.ShouldSync(event) {
		logger.V(6).Info("Event filtered out")
		return
	}

	// Check for duplicate events
	if s.isDuplicate(event) {
		logger.V(6).Info("Duplicate event skipped")
		return
	}

	// Transform the event for KCP
	transformed := s.transformEvent(event)

	// Check if event should be aggregated
	if s.aggregator.ShouldAggregate(transformed) {
		logger.V(5).Info("Event added to aggregation")
		s.aggregator.AddEvent(transformed)
		return
	}

	// Sync individual event to KCP
	if err := s.syncToKCP(ctx, transformed); err != nil {
		logger.Error(err, "Failed to sync event to KCP")
		return
	}

	logger.V(4).Info("Event synced successfully")
}

// transformEvent transforms a downstream event for storage in KCP.
// It adds necessary metadata and updates namespace references.
func (s *EventSyncer) transformEvent(event *corev1.Event) *corev1.Event {
	transformed := event.DeepCopy()

	// Update namespace to match KCP workspace structure
	if transformed.Namespace != "" {
		transformed.Namespace = s.reverseNamespaceTransform(transformed.Namespace)
	}

	// Update involved object namespace
	if transformed.InvolvedObject.Namespace != "" {
		transformed.InvolvedObject.Namespace = s.reverseNamespaceTransform(transformed.InvolvedObject.Namespace)
	}

	// Add sync target labels and annotations
	if transformed.Labels == nil {
		transformed.Labels = make(map[string]string)
	}
	transformed.Labels["kcp.io/sync-target"] = s.syncTargetName
	transformed.Labels["kcp.io/workspace"] = s.workspace.String()

	if transformed.Annotations == nil {
		transformed.Annotations = make(map[string]string)
	}
	transformed.Annotations["kcp.io/source-cluster"] = s.syncTargetName
	transformed.Annotations["kcp.io/synced-at"] = time.Now().Format(time.RFC3339)

	// Add configured labels and annotations
	for k, v := range s.config.AddLabels {
		transformed.Labels[k] = v
	}
	for k, v := range s.config.AddAnnotations {
		transformed.Annotations[k] = v
	}

	// Update event name to avoid conflicts across clusters
	transformed.Name = fmt.Sprintf("%s-%s", s.syncTargetName, transformed.Name)
	transformed.GenerateName = ""

	return transformed
}

// syncToKCP creates the transformed event in the KCP workspace
func (s *EventSyncer) syncToKCP(ctx context.Context, event *corev1.Event) error {
	// Clear fields that should not be set on creation
	event.ResourceVersion = ""
	event.UID = ""

	_, err := s.kcpClient.
		Cluster(s.workspace.Path()).
		CoreV1().
		Events(event.Namespace).
		Create(ctx, event, metav1.CreateOptions{})

	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Event already exists, this is fine
			return nil
		}
		return fmt.Errorf("failed to create event in KCP: %w", err)
	}

	// Mark event as seen to prevent duplicates
	s.markSeen(event)
	return nil
}

// syncAggregatedEvents handles syncing of aggregated events to KCP
func (s *EventSyncer) syncAggregatedEvents(ctx context.Context, events []*corev1.Event) {
	logger := klog.FromContext(ctx).WithName("aggregated-sync")

	for _, event := range events {
		if err := s.syncToKCP(ctx, event); err != nil {
			logger.Error(err, "Failed to sync aggregated event", "event", event.Name)
		}
	}

	logger.V(4).Info("Synced aggregated events", "count", len(events))
}

// isDuplicate checks if we've already processed this event recently
func (s *EventSyncer) isDuplicate(event *corev1.Event) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.getEventKey(event)
	lastSeen, exists := s.seenEvents[key]
	if !exists {
		return false
	}

	// Consider it duplicate if seen within the last 5 minutes
	return time.Since(lastSeen) < 5*time.Minute
}

// markSeen records that we've processed this event
func (s *EventSyncer) markSeen(event *corev1.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getEventKey(event)
	s.seenEvents[key] = time.Now()
}

// getEventKey creates a unique key for event deduplication
func (s *EventSyncer) getEventKey(event *corev1.Event) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s",
		event.Namespace,
		event.InvolvedObject.Kind,
		event.InvolvedObject.Name,
		event.Reason,
		event.Message,
	)
}

// reverseNamespaceTransform converts a downstream namespace to KCP namespace
func (s *EventSyncer) reverseNamespaceTransform(namespace string) string {
	// This would typically involve removing sync-target specific prefixes
	// For now, return as-is since we don't know the exact transformation rules
	return namespace
}

// cleanupLoop periodically removes old entries from the seen events map
func (s *EventSyncer) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanupSeenEvents()
		}
	}
}

// cleanupSeenEvents removes old entries from the seen events map
func (s *EventSyncer) cleanupSeenEvents() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-30 * time.Minute)
	for key, timestamp := range s.seenEvents {
		if timestamp.Before(cutoff) {
			delete(s.seenEvents, key)
		}
	}
}