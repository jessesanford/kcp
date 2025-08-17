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
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// EventFilter provides event filtering capabilities to reduce noise
// and ensure only relevant events are synced to KCP.
type EventFilter struct {
	config EventSyncConfig

	// Rate limiting per event source
	rateLimiter map[string]*rateLimitInfo
	mu          sync.RWMutex

	// Filter sets for efficient lookup
	includeTypes sets.Set[string]
	excludeTypes sets.Set[string]
}

type rateLimitInfo struct {
	count     int
	window    time.Time
	lastEvent time.Time
}

// NewEventFilter creates a new event filter with the given configuration
func NewEventFilter(config EventSyncConfig) *EventFilter {
	filter := &EventFilter{
		config:      config,
		rateLimiter: make(map[string]*rateLimitInfo),
		includeTypes: sets.New(config.IncludeTypes...),
		excludeTypes: sets.New(config.ExcludeTypes...),
	}

	return filter
}

// ShouldSync determines if an event should be synchronized to KCP
// based on the configured filtering rules.
func (f *EventFilter) ShouldSync(event *corev1.Event) bool {
	// Apply type filtering
	if !f.isTypeAllowed(event.Type) {
		return false
	}

	// Apply severity filtering
	if !f.isSeverityAllowed(event.Type, event.Reason) {
		return false
	}

	// Apply reason filtering
	if !f.isReasonAllowed(event.Reason) {
		return false
	}

	// Apply rate limiting
	if !f.checkRateLimit(event) {
		return false
	}

	return true
}

// isTypeAllowed checks if the event type should be synced
func (f *EventFilter) isTypeAllowed(eventType string) bool {
	// If include list is specified, event must be in it
	if f.includeTypes.Len() > 0 {
		return f.includeTypes.Has(eventType)
	}

	// If exclude list is specified, event must not be in it
	if f.excludeTypes.Len() > 0 {
		return !f.excludeTypes.Has(eventType)
	}

	// Default: allow all types
	return true
}

// isSeverityAllowed checks if the event meets the minimum severity threshold
func (f *EventFilter) isSeverityAllowed(eventType, reason string) bool {
	if f.config.MinimumSeverity == "" {
		return true
	}

	severity := f.getEventSeverity(eventType, reason)
	minSeverity := f.getSeverityLevel(f.config.MinimumSeverity)

	return severity >= minSeverity
}

// isReasonAllowed checks if the event reason should be synced
func (f *EventFilter) isReasonAllowed(reason string) bool {
	// Filter out common noisy reasons
	noisyReasons := []string{
		"Pulled", "Created", "Started",
		"SuccessfulMount", "SuccessfulAttach",
		"Sync", "SandboxChanged",
	}

	for _, noisy := range noisyReasons {
		if strings.Contains(reason, noisy) {
			return false
		}
	}

	return true
}

// checkRateLimit applies rate limiting to prevent event storms
func (f *EventFilter) checkRateLimit(event *corev1.Event) bool {
	if f.config.MaxEventsPerMinute <= 0 {
		return true // Rate limiting disabled
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	key := f.getRateLimitKey(event)
	info, exists := f.rateLimiter[key]

	now := time.Now()
	if !exists || now.Sub(info.window) > time.Minute {
		// New window or first event for this key
		f.rateLimiter[key] = &rateLimitInfo{
			count:     1,
			window:    now,
			lastEvent: now,
		}
		return true
	}

	// Check if we're within rate limit
	if info.count >= f.config.MaxEventsPerMinute {
		return false
	}

	// Update rate limit info
	info.count++
	info.lastEvent = now
	return true
}

// getRateLimitKey creates a key for rate limiting based on event characteristics
func (f *EventFilter) getRateLimitKey(event *corev1.Event) string {
	return event.InvolvedObject.Kind + "/" + event.InvolvedObject.Namespace + "/" + event.InvolvedObject.Name
}

// getEventSeverity determines the severity level of an event
func (f *EventFilter) getEventSeverity(eventType, reason string) int {
	// Higher numbers = higher severity
	
	if eventType == corev1.EventTypeWarning {
		return f.getWarningSeverity(reason)
	}

	if eventType == corev1.EventTypeNormal {
		return f.getNormalSeverity(reason)
	}

	return 1 // Default low severity
}

// getWarningSeverity returns severity for warning events
func (f *EventFilter) getWarningSeverity(reason string) int {
	highSeverityReasons := []string{
		"Failed", "Error", "FailedMount", "FailedAttach",
		"Unhealthy", "BackOff", "FailedScheduling",
		"FailedBinding", "FailedValidation",
	}

	for _, high := range highSeverityReasons {
		if strings.Contains(reason, high) {
			return 5 // High severity
		}
	}

	return 3 // Medium severity for other warnings
}

// getNormalSeverity returns severity for normal events
func (f *EventFilter) getNormalSeverity(reason string) int {
	importantReasons := []string{
		"Scheduled", "Killing", "Preempting",
		"NodeReady", "NodeNotReady",
	}

	for _, important := range importantReasons {
		if strings.Contains(reason, important) {
			return 2 // Medium-low severity
		}
	}

	return 1 // Low severity for other normal events
}

// getSeverityLevel converts severity string to numeric level
func (f *EventFilter) getSeverityLevel(severity string) int {
	switch strings.ToLower(severity) {
	case "high":
		return 5
	case "medium":
		return 3
	case "low":
		return 1
	default:
		return 1
	}
}

// CleanupRateLimits removes old rate limit entries
func (f *EventFilter) CleanupRateLimits() {
	f.mu.Lock()
	defer f.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute)
	for key, info := range f.rateLimiter {
		if info.lastEvent.Before(cutoff) {
			delete(f.rateLimiter, key)
		}
	}
}