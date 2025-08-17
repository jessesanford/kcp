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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// EventAggregator groups similar events together to reduce noise
// and provide a cleaner view of what's happening in the cluster.
type EventAggregator struct {
	config EventSyncConfig

	// Aggregated events indexed by aggregation key
	events map[AggregationKey]*AggregatedEventGroup
	mu     sync.RWMutex

	// Flush callback
	flushCallback func(context.Context, []*corev1.Event)
}

// AggregatedEventGroup represents a group of similar events
type AggregatedEventGroup struct {
	FirstEvent  *corev1.Event
	LastEvent   *corev1.Event
	Count       int32
	FirstTime   time.Time
	LastTime    time.Time
	Events      []*corev1.Event
	Flushed     bool
}

// NewEventAggregator creates a new event aggregator
func NewEventAggregator(config EventSyncConfig) *EventAggregator {
	return &EventAggregator{
		config: config,
		events: make(map[AggregationKey]*AggregatedEventGroup),
	}
}

// ShouldAggregate determines if an event should be aggregated
// based on its characteristics and current aggregation state.
func (a *EventAggregator) ShouldAggregate(event *corev1.Event) bool {
	// Don't aggregate high-severity events
	if event.Type == corev1.EventTypeWarning {
		return false
	}

	// Don't aggregate unique events
	if a.isUniqueEvent(event) {
		return false
	}

	return true
}

// AddEvent adds an event to the aggregation groups
func (a *EventAggregator) AddEvent(event *corev1.Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := a.getAggregationKey(event)
	group, exists := a.events[key]

	if !exists {
		// Create new aggregation group
		a.events[key] = &AggregatedEventGroup{
			FirstEvent: event,
			LastEvent:  event,
			Count:      1,
			FirstTime:  event.FirstTimestamp.Time,
			LastTime:   event.LastTimestamp.Time,
			Events:     []*corev1.Event{event},
			Flushed:    false,
		}
		return
	}

	// Update existing group
	group.LastEvent = event
	group.Count++
	group.LastTime = event.LastTimestamp.Time
	
	// Keep events if within limit
	if len(group.Events) < a.config.MaxAggregatedEvents {
		group.Events = append(group.Events, event)
	}
}

// StartFlushLoop starts the periodic flush of aggregated events
func (a *EventAggregator) StartFlushLoop(ctx context.Context, callback func(context.Context, []*corev1.Event)) {
	a.flushCallback = callback

	ticker := time.NewTicker(a.config.AggregationWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.flushEvents(ctx)
		}
	}
}

// flushEvents creates summarized events for aggregated groups
func (a *EventAggregator) flushEvents(ctx context.Context) {
	logger := klog.FromContext(ctx).WithName("event-aggregator")

	a.mu.Lock()
	toFlush := make([]*corev1.Event, 0)
	cutoff := time.Now().Add(-a.config.AggregationWindow)

	for key, group := range a.events {
		// Only flush groups that are old enough and haven't been flushed
		if group.LastTime.Before(cutoff) && !group.Flushed {
			summarized := a.createSummarizedEvent(group, key)
			toFlush = append(toFlush, summarized)
			group.Flushed = true
		}
	}

	// Clean up old groups
	for key, group := range a.events {
		if group.Flushed && group.LastTime.Before(cutoff.Add(-time.Hour)) {
			delete(a.events, key)
		}
	}
	a.mu.Unlock()

	if len(toFlush) > 0 {
		logger.V(4).Info("Flushing aggregated events", "count", len(toFlush))
		if a.flushCallback != nil {
			a.flushCallback(ctx, toFlush)
		}
	}
}

// createSummarizedEvent creates a single event that summarizes a group
func (a *EventAggregator) createSummarizedEvent(group *AggregatedEventGroup, key AggregationKey) *corev1.Event {
	summarized := group.FirstEvent.DeepCopy()

	// Update the message to show aggregation
	if group.Count > 1 {
		summarized.Message = fmt.Sprintf("%s (occurred %d times)", summarized.Message, group.Count)
		summarized.Count = group.Count
	}

	// Update timing information
	summarized.FirstTimestamp = metav1.Time{Time: group.FirstTime}
	summarized.LastTimestamp = metav1.Time{Time: group.LastTime}

	// Add aggregation metadata
	if summarized.Annotations == nil {
		summarized.Annotations = make(map[string]string)
	}
	summarized.Annotations["kcp.io/event-aggregated"] = "true"
	summarized.Annotations["kcp.io/event-count"] = fmt.Sprintf("%d", group.Count)
	summarized.Annotations["kcp.io/aggregation-window"] = a.config.AggregationWindow.String()

	// Update event name for uniqueness
	summarized.Name = fmt.Sprintf("%s-aggregated-%d", summarized.Name, group.Count)
	summarized.GenerateName = ""

	return summarized
}

// getAggregationKey creates a key for grouping similar events
func (a *EventAggregator) getAggregationKey(event *corev1.Event) AggregationKey {
	return AggregationKey{
		Namespace: event.Namespace,
		Name:      event.InvolvedObject.Name,
		Type:      event.Type,
		Reason:    event.Reason,
		Message:   a.normalizeMessage(event.Message),
	}
}

// normalizeMessage normalizes event messages for better grouping
func (a *EventAggregator) normalizeMessage(message string) string {
	// Remove timestamps and other variable elements to improve grouping
	// This is a simplified implementation - in practice, you'd use regex
	// to handle more complex patterns
	
	// For now, return first 100 characters to group by message prefix
	if len(message) > 100 {
		return message[:100]
	}
	return message
}

// isUniqueEvent determines if an event should not be aggregated
func (a *EventAggregator) isUniqueEvent(event *corev1.Event) bool {
	uniqueReasons := []string{
		"Started", "Killing", "Pulled",
		"Created", "Scheduled",
	}

	for _, reason := range uniqueReasons {
		if event.Reason == reason {
			return true
		}
	}

	return false
}

// GetAggregationStats returns statistics about current aggregation state
func (a *EventAggregator) GetAggregationStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["active_groups"] = len(a.events)
	
	totalEvents := int32(0)
	for _, group := range a.events {
		totalEvents += group.Count
	}
	stats["total_aggregated_events"] = totalEvents

	return stats
}