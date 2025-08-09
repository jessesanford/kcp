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
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// eventNotifier implements EventNotifier for sending event notifications.
type eventNotifier struct {
	listeners map[string]EventListener
	timeout   time.Duration
	mu        sync.RWMutex
}

// newEventNotifier creates a new event notifier.
func newEventNotifier(timeout time.Duration) (EventNotifier, error) {
	return &eventNotifier{
		listeners: make(map[string]EventListener),
		timeout:   timeout,
	}, nil
}

// NotifyEvent sends notifications for significant TMC events.
func (n *eventNotifier) NotifyEvent(ctx context.Context, event *TMCEvent) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Find matching listeners
	var matchingListeners []EventListener
	for _, listener := range n.listeners {
		if n.listenerMatches(listener, event) {
			matchingListeners = append(matchingListeners, listener)
		}
	}

	if len(matchingListeners) == 0 {
		klog.V(8).Infof("No listeners found for event type %s", event.Type)
		return nil
	}

	// Send notifications concurrently
	var wg sync.WaitGroup
	errors := make(chan error, len(matchingListeners))

	for _, listener := range matchingListeners {
		wg.Add(1)
		go func(l EventListener) {
			defer wg.Done()
			if err := n.notifyListener(ctx, l, event); err != nil {
				errors <- fmt.Errorf("listener %s failed: %w", l.GetID(), err)
			}
		}(listener)
	}

	// Wait for all notifications to complete with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	ctx, cancel := context.WithTimeout(ctx, n.timeout)
	defer cancel()

	select {
	case <-done:
		// All notifications completed
	case <-ctx.Done():
		return fmt.Errorf("notification timeout after %v", n.timeout)
	}

	// Collect any errors
	close(errors)
	var allErrors []error
	for err := range errors {
		allErrors = append(allErrors, err)
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("notification errors: %v", allErrors)
	}

	klog.V(6).Infof("Successfully notified %d listeners for event %s", 
		len(matchingListeners), event.Type)

	return nil
}

// AddListener adds an event listener for specific event types.
func (n *eventNotifier) AddListener(eventTypes []EventType, listener EventListener) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if listener == nil {
		return fmt.Errorf("listener cannot be nil")
	}

	id := listener.GetID()
	if id == "" {
		return fmt.Errorf("listener ID cannot be empty")
	}

	if _, exists := n.listeners[id]; exists {
		return fmt.Errorf("listener with ID %s already exists", id)
	}

	n.listeners[id] = listener
	klog.V(4).Infof("Added event listener %s for types %v", id, eventTypes)
	return nil
}

// RemoveListener removes an event listener.
func (n *eventNotifier) RemoveListener(listener EventListener) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if listener == nil {
		return fmt.Errorf("listener cannot be nil")
	}

	id := listener.GetID()
	if _, exists := n.listeners[id]; !exists {
		return fmt.Errorf("listener with ID %s not found", id)
	}

	delete(n.listeners, id)
	klog.V(4).Infof("Removed event listener %s", id)
	return nil
}

// notifyListener sends a notification to a specific listener with timeout protection.
func (n *eventNotifier) notifyListener(ctx context.Context, listener EventListener, event *TMCEvent) error {
	ctx, cancel := context.WithTimeout(ctx, n.timeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("listener panicked: %v", r)
			}
		}()
		done <- listener.OnEvent(ctx, event)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("listener timeout: %w", ctx.Err())
	}
}

// listenerMatches checks if a listener should receive the event.
func (n *eventNotifier) listenerMatches(listener EventListener, event *TMCEvent) bool {
	listenerTypes := listener.GetEventTypes()
	
	// If listener has no specific types, it receives all events
	if len(listenerTypes) == 0 {
		return true
	}

	// Check if event type matches any listener type
	for _, listenerType := range listenerTypes {
		if listenerType == event.Type {
			return true
		}
	}

	return false
}