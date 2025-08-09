/*
Copyright 2025 The KCP Authors.

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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
)

// eventStore provides in-memory storage and retrieval of TMC events.
type eventStore struct {
	events            map[string]*TMCEvent
	eventsByWorkspace map[string][]*TMCEvent
	eventsByType      map[EventType][]*TMCEvent
	maxAge            time.Duration
	mu                sync.RWMutex
	lastCleanup       time.Time
}

// newEventStore creates a new event store.
func newEventStore(maxAge time.Duration) *eventStore {
	return &eventStore{
		events:            make(map[string]*TMCEvent),
		eventsByWorkspace: make(map[string][]*TMCEvent),
		eventsByType:      make(map[EventType][]*TMCEvent),
		maxAge:            maxAge,
		lastCleanup:       time.Now(),
	}
}

// addEvent adds an event to the store.
func (s *eventStore) addEvent(event *TMCEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.generateEventKey(event)

	// Check if we already have this event (for deduplication)
	if existing, exists := s.events[key]; exists {
		existing.Count++
		existing.Timestamp = event.Timestamp
		existing.Message = event.Message
		return
	}

	// Add new event
	s.events[key] = event

	// Update workspace index
	workspaceKey := event.Workspace.String()
	s.eventsByWorkspace[workspaceKey] = append(s.eventsByWorkspace[workspaceKey], event)

	// Update type index
	s.eventsByType[event.Type] = append(s.eventsByType[event.Type], event)

	// Periodically cleanup old events
	if time.Since(s.lastCleanup) > time.Hour {
		s.cleanupOldEvents()
		s.lastCleanup = time.Now()
	}
}

// getEventsForWorkspace retrieves all events for a workspace.
func (s *eventStore) getEventsForWorkspace(workspace logicalcluster.Name) []*TMCEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceKey := workspace.String()
	events := s.eventsByWorkspace[workspaceKey]

	// Sort by timestamp (newest first)
	result := make([]*TMCEvent, len(events))
	copy(result, events)

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result
}

// getEventsByType retrieves events by type.
func (s *eventStore) getEventsByType(eventType EventType) []*TMCEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.eventsByType[eventType]
	result := make([]*TMCEvent, len(events))
	copy(result, events)

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result
}

// cleanupOldEvents removes events older than maxAge.
func (s *eventStore) cleanupOldEvents() {
	if s.maxAge <= 0 {
		return
	}

	cutoff := time.Now().Add(-s.maxAge)
	var toRemove []string

	for key, event := range s.events {
		if event.Timestamp.Before(cutoff) {
			toRemove = append(toRemove, key)
		}
	}

	for _, key := range toRemove {
		event := s.events[key]
		delete(s.events, key)

		// Remove from workspace index
		workspaceKey := event.Workspace.String()
		s.removeFromSlice(&s.eventsByWorkspace[workspaceKey], event)

		// Remove from type index
		s.removeFromSlice(&s.eventsByType[event.Type], event)
	}
}

// removeFromSlice removes an event from a slice.
func (s *eventStore) removeFromSlice(slice *[]*TMCEvent, event *TMCEvent) {
	for i, e := range *slice {
		if e == event {
			*slice = append((*slice)[:i], (*slice)[i+1:]...)
			break
		}
	}
}

// generateEventKey generates a unique key for an event for deduplication.
func (s *eventStore) generateEventKey(event *TMCEvent) string {
	objectKey := "<none>"
	if event.Object != nil {
		objectKey = getObjectKey(event.Object)
	}

	return strings.Join([]string{
		event.Workspace.String(),
		string(event.Type),
		string(event.Reason),
		objectKey,
		event.Source,
	}, "::")
}

// Helper function to get object key
func getObjectKey(obj interface{}) string {
	// Simplified object key generation
	return "<object>"
}