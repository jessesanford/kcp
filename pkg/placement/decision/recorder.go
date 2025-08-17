/*
Copyright 2023 The KCP Authors.

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

package decision

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// DecisionRecorder provides the interface for recording placement decisions and events.
type DecisionRecorder interface {
	// RecordDecision records a placement decision for audit and debugging
	RecordDecision(ctx context.Context, decision *PlacementDecision) error
	
	// RecordEvent records a decision-related event
	RecordEvent(ctx context.Context, decisionID string, event DecisionEvent) error
	
	// GetDecisionHistory returns the decision history for a specific placement
	GetDecisionHistory(ctx context.Context, placementID string) ([]*DecisionRecord, error)
	
	// GetDecision retrieves a specific decision by ID
	GetDecision(ctx context.Context, decisionID string) (*DecisionRecord, error)
	
	// ListDecisions lists decisions with optional filtering
	ListDecisions(ctx context.Context, filter DecisionFilter) ([]*DecisionRecord, error)
	
	// PruneHistory removes old decision records based on retention policy
	PruneHistory(ctx context.Context, retentionPolicy RetentionPolicy) error
}

// DecisionFilter provides filtering options for listing decisions.
type DecisionFilter struct {
	// PlacementID filters by placement request ID
	PlacementID string
	
	// Status filters by decision status
	Status DecisionStatus
	
	// CreatedAfter filters decisions created after this time
	CreatedAfter *time.Time
	
	// CreatedBefore filters decisions created before this time
	CreatedBefore *time.Time
	
	// Limit limits the number of results (0 = no limit)
	Limit int
}

// RetentionPolicy defines how long to retain decision records.
type RetentionPolicy struct {
	// MaxAge is the maximum age of records to retain
	MaxAge time.Duration
	
	// MaxRecords is the maximum number of records to retain (0 = no limit)
	MaxRecords int
	
	// PreserveErrors indicates whether to preserve error records longer
	PreserveErrors bool
	
	// ErrorRetentionMultiplier multiplies MaxAge for error records
	ErrorRetentionMultiplier float64
}

// inMemoryDecisionRecorder implements DecisionRecorder using in-memory storage.
// This is suitable for development and testing. Production systems should use
// a persistent storage implementation.
type inMemoryDecisionRecorder struct {
	mu       sync.RWMutex
	records  map[string]*DecisionRecord  // decisionID -> record
	byPlacement map[string][]*DecisionRecord // placementID -> records
	events   map[string][]DecisionEvent  // decisionID -> events
}

// NewInMemoryDecisionRecorder creates a new in-memory decision recorder.
func NewInMemoryDecisionRecorder() DecisionRecorder {
	return &inMemoryDecisionRecorder{
		records:     make(map[string]*DecisionRecord),
		byPlacement: make(map[string][]*DecisionRecord),
		events:      make(map[string][]DecisionEvent),
	}
}

// RecordDecision records a placement decision for audit and debugging.
func (r *inMemoryDecisionRecorder) RecordDecision(ctx context.Context, decision *PlacementDecision) error {
	if decision == nil {
		return fmt.Errorf("decision cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	klog.V(3).InfoS("Recording decision", "decisionID", decision.ID, "requestID", decision.RequestID)

	// Create or update the decision record
	record, exists := r.records[decision.ID]
	if !exists {
		record = &DecisionRecord{
			Decision:  decision,
			Timestamp: time.Now(),
			Version:   1,
			Events:    []DecisionEvent{},
		}
	} else {
		// Update existing record
		record.Decision = decision
		record.Timestamp = time.Now()
		record.Version++
	}

	// Add any events associated with this decision
	if events, hasEvents := r.events[decision.ID]; hasEvents {
		record.Events = events
	}

	// Store the record
	r.records[decision.ID] = record

	// Index by placement ID for efficient lookups
	if decision.RequestID != "" {
		r.byPlacement[decision.RequestID] = append(r.byPlacement[decision.RequestID], record)
		
		// Sort by timestamp (most recent first)
		sort.Slice(r.byPlacement[decision.RequestID], func(i, j int) bool {
			return r.byPlacement[decision.RequestID][i].Timestamp.After(r.byPlacement[decision.RequestID][j].Timestamp)
		})
	}

	klog.V(4).InfoS("Decision recorded successfully", 
		"decisionID", decision.ID, 
		"version", record.Version,
		"eventsCount", len(record.Events))

	return nil
}

// RecordEvent records a decision-related event.
func (r *inMemoryDecisionRecorder) RecordEvent(ctx context.Context, decisionID string, event DecisionEvent) error {
	if decisionID == "" {
		return fmt.Errorf("decision ID cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	klog.V(4).InfoS("Recording event", "decisionID", decisionID, "eventType", event.Type)

	// Add event to the events map
	r.events[decisionID] = append(r.events[decisionID], event)

	// If we have a decision record, update it as well
	if record, exists := r.records[decisionID]; exists {
		record.Events = append(record.Events, event)
		record.Timestamp = time.Now() // Update record timestamp
		record.Version++
	}

	return nil
}

// GetDecisionHistory returns the decision history for a specific placement.
func (r *inMemoryDecisionRecorder) GetDecisionHistory(ctx context.Context, placementID string) ([]*DecisionRecord, error) {
	if placementID == "" {
		return nil, fmt.Errorf("placement ID cannot be empty")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	klog.V(3).InfoS("Getting decision history", "placementID", placementID)

	records, exists := r.byPlacement[placementID]
	if !exists {
		return []*DecisionRecord{}, nil
	}

	// Return a copy to prevent external modification
	result := make([]*DecisionRecord, len(records))
	copy(result, records)

	klog.V(4).InfoS("Retrieved decision history", "placementID", placementID, "recordCount", len(result))

	return result, nil
}

// GetDecision retrieves a specific decision by ID.
func (r *inMemoryDecisionRecorder) GetDecision(ctx context.Context, decisionID string) (*DecisionRecord, error) {
	if decisionID == "" {
		return nil, fmt.Errorf("decision ID cannot be empty")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	record, exists := r.records[decisionID]
	if !exists {
		return nil, fmt.Errorf("decision not found: %s", decisionID)
	}

	klog.V(4).InfoS("Retrieved decision", "decisionID", decisionID, "version", record.Version)

	// Return a copy to prevent external modification
	recordCopy := *record
	return &recordCopy, nil
}

// ListDecisions lists decisions with optional filtering.
func (r *inMemoryDecisionRecorder) ListDecisions(ctx context.Context, filter DecisionFilter) ([]*DecisionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	klog.V(3).InfoS("Listing decisions", "filter", fmt.Sprintf("%+v", filter))

	var results []*DecisionRecord

	// If filtering by placement ID, use the index
	if filter.PlacementID != "" {
		if records, exists := r.byPlacement[filter.PlacementID]; exists {
			results = make([]*DecisionRecord, len(records))
			copy(results, records)
		}
	} else {
		// Iterate through all records
		results = make([]*DecisionRecord, 0, len(r.records))
		for _, record := range r.records {
			results = append(results, record)
		}
		
		// Sort by timestamp (most recent first)
		sort.Slice(results, func(i, j int) bool {
			return results[i].Timestamp.After(results[j].Timestamp)
		})
	}

	// Apply additional filters
	filteredResults := []*DecisionRecord{}
	for _, record := range results {
		if r.matchesFilter(record, filter) {
			filteredResults = append(filteredResults, record)
		}
	}

	// Apply limit
	if filter.Limit > 0 && len(filteredResults) > filter.Limit {
		filteredResults = filteredResults[:filter.Limit]
	}

	klog.V(4).InfoS("Listed decisions", "totalRecords", len(r.records), "filteredCount", len(filteredResults))

	return filteredResults, nil
}

// matchesFilter checks if a decision record matches the given filter criteria.
func (r *inMemoryDecisionRecorder) matchesFilter(record *DecisionRecord, filter DecisionFilter) bool {
	// Check status filter
	if filter.Status != "" && record.Decision.Status != filter.Status {
		return false
	}

	// Check created after filter
	if filter.CreatedAfter != nil && record.Timestamp.Before(*filter.CreatedAfter) {
		return false
	}

	// Check created before filter
	if filter.CreatedBefore != nil && record.Timestamp.After(*filter.CreatedBefore) {
		return false
	}

	return true
}

// PruneHistory removes old decision records based on retention policy.
func (r *inMemoryDecisionRecorder) PruneHistory(ctx context.Context, retentionPolicy RetentionPolicy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	klog.V(2).InfoS("Pruning decision history", "policy", fmt.Sprintf("%+v", retentionPolicy))

	now := time.Now()
	pruneThreshold := now.Add(-retentionPolicy.MaxAge)
	errorPruneThreshold := now.Add(-time.Duration(float64(retentionPolicy.MaxAge) * retentionPolicy.ErrorRetentionMultiplier))

	prunedCount := 0
	preservedErrorCount := 0

	// Collect records to prune
	recordsToPrune := []string{}

	for decisionID, record := range r.records {
		shouldPrune := false

		// Check age-based pruning
		if record.Decision.Status == DecisionStatusError && retentionPolicy.PreserveErrors {
			if record.Timestamp.Before(errorPruneThreshold) {
				shouldPrune = true
			} else {
				preservedErrorCount++
			}
		} else {
			if record.Timestamp.Before(pruneThreshold) {
				shouldPrune = true
			}
		}

		if shouldPrune {
			recordsToPrune = append(recordsToPrune, decisionID)
		}
	}

	// Apply record count limit if specified
	if retentionPolicy.MaxRecords > 0 && len(r.records)-len(recordsToPrune) > retentionPolicy.MaxRecords {
		// Sort remaining records by timestamp (oldest first)
		type recordInfo struct {
			id        string
			timestamp time.Time
		}

		remainingRecords := []recordInfo{}
		for decisionID, record := range r.records {
			found := false
			for _, pruneID := range recordsToPrune {
				if decisionID == pruneID {
					found = true
					break
				}
			}
			if !found {
				remainingRecords = append(remainingRecords, recordInfo{
					id:        decisionID,
					timestamp: record.Timestamp,
				})
			}
		}

		sort.Slice(remainingRecords, func(i, j int) bool {
			return remainingRecords[i].timestamp.Before(remainingRecords[j].timestamp)
		})

		// Add oldest records to prune list
		excessCount := len(remainingRecords) - retentionPolicy.MaxRecords
		for i := 0; i < excessCount; i++ {
			recordsToPrune = append(recordsToPrune, remainingRecords[i].id)
		}
	}

	// Actually prune the records
	for _, decisionID := range recordsToPrune {
		record := r.records[decisionID]
		
		// Remove from main records map
		delete(r.records, decisionID)
		
		// Remove from placement index
		if record.Decision.RequestID != "" {
			placementRecords := r.byPlacement[record.Decision.RequestID]
			for i, pr := range placementRecords {
				if pr.Decision.ID == decisionID {
					// Remove from slice
					r.byPlacement[record.Decision.RequestID] = append(placementRecords[:i], placementRecords[i+1:]...)
					break
				}
			}
			
			// Clean up empty placement entries
			if len(r.byPlacement[record.Decision.RequestID]) == 0 {
				delete(r.byPlacement, record.Decision.RequestID)
			}
		}
		
		// Remove events
		delete(r.events, decisionID)
		
		prunedCount++
	}

	klog.V(2).InfoS("Decision history pruning completed",
		"prunedCount", prunedCount,
		"preservedErrorCount", preservedErrorCount,
		"remainingRecords", len(r.records))

	return nil
}

// GetStats returns statistics about the recorded decisions.
func (r *inMemoryDecisionRecorder) GetStats() DecisionStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := DecisionStats{
		TotalDecisions:    len(r.records),
		TotalPlacements:   len(r.byPlacement),
		StatusCounts:      make(map[DecisionStatus]int),
		OldestRecordTime:  time.Time{},
		NewestRecordTime:  time.Time{},
	}

	for _, record := range r.records {
		// Count by status
		stats.StatusCounts[record.Decision.Status]++

		// Track oldest and newest
		if stats.OldestRecordTime.IsZero() || record.Timestamp.Before(stats.OldestRecordTime) {
			stats.OldestRecordTime = record.Timestamp
		}
		if stats.NewestRecordTime.IsZero() || record.Timestamp.After(stats.NewestRecordTime) {
			stats.NewestRecordTime = record.Timestamp
		}
	}

	return stats
}

// DecisionStats provides statistics about recorded decisions.
type DecisionStats struct {
	// TotalDecisions is the total number of decision records
	TotalDecisions int
	
	// TotalPlacements is the total number of unique placements
	TotalPlacements int
	
	// StatusCounts counts decisions by status
	StatusCounts map[DecisionStatus]int
	
	// OldestRecordTime is the timestamp of the oldest record
	OldestRecordTime time.Time
	
	// NewestRecordTime is the timestamp of the newest record
	NewestRecordTime time.Time
}