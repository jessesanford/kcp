/*
Copyright The KCP Authors.

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

// Package aggregator implements status aggregation strategies for the TMC system.
package aggregator

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/unstructured"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/status/interfaces"
)

// Aggregator implements the StatusAggregator interface for combining
// status updates from multiple sources using configurable strategies
type Aggregator struct {
	// mu protects access to strategies
	mu sync.RWMutex

	// defaultStrategies maps resource types to their default aggregation strategies
	defaultStrategies map[schema.GroupVersionResource]interfaces.AggregationStrategy

	// sourcePriorities maps source names to their priorities (higher = more important)
	sourcePriorities map[string]int

	// merger handles field-level merging
	merger interfaces.StatusMerger

	// cache provides caching for aggregated results
	cache interfaces.StatusCache
}

// AggregatorConfig contains configuration for the status aggregator
type AggregatorConfig struct {
	// DefaultStrategy is used when no specific strategy is configured
	DefaultStrategy interfaces.AggregationStrategy

	// SourcePriorities defines priority for each source (higher = more important)
	SourcePriorities map[string]int

	// Merger handles field-level status merging
	Merger interfaces.StatusMerger

	// Cache provides result caching
	Cache interfaces.StatusCache
}

// NewAggregator creates a new status aggregator instance
func NewAggregator(config AggregatorConfig) *Aggregator {
	if config.DefaultStrategy == "" {
		config.DefaultStrategy = interfaces.AggregationStrategyLatestWins
	}

	return &Aggregator{
		defaultStrategies: make(map[schema.GroupVersionResource]interfaces.AggregationStrategy),
		sourcePriorities:  config.SourcePriorities,
		merger:            config.Merger,
		cache:             config.Cache,
	}
}

// AggregateStatus combines multiple status updates using the specified strategy
func (a *Aggregator) AggregateStatus(ctx context.Context, updates []*interfaces.StatusUpdate, strategy interfaces.AggregationStrategy) (*interfaces.AggregatedStatus, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("no status updates provided")
	}

	klog.V(4).InfoS("Aggregating status", 
		"strategy", string(strategy), 
		"updates", len(updates))

	// Create aggregated status result
	result := &interfaces.AggregatedStatus{
		AggregatedAt: time.Now(),
		Strategy:     strategy,
		Sources:      make([]string, 0, len(updates)),
	}

	// Collect all sources
	for _, update := range updates {
		result.Sources = append(result.Sources, update.Source)
	}

	var err error
	switch strategy {
	case interfaces.AggregationStrategyLatestWins:
		result.Status, err = a.aggregateLatestWins(ctx, updates)
	case interfaces.AggregationStrategyMergeAll:
		result.Status, result.Conflicts, err = a.aggregateMergeAll(ctx, updates)
	case interfaces.AggregationStrategyConflictDetection:
		result.Status, result.Conflicts, err = a.aggregateConflictDetection(ctx, updates)
	case interfaces.AggregationStrategySourcePriority:
		result.Status, err = a.aggregateSourcePriority(ctx, updates)
	default:
		return nil, fmt.Errorf("unknown aggregation strategy: %s", strategy)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to aggregate status using strategy %s: %w", strategy, err)
	}

	klog.V(4).InfoS("Status aggregation completed", 
		"strategy", string(strategy),
		"sources", len(result.Sources),
		"conflicts", len(result.Conflicts))

	return result, nil
}

// SetDefaultStrategy sets the default aggregation strategy for a resource type
func (a *Aggregator) SetDefaultStrategy(gvr schema.GroupVersionResource, strategy interfaces.AggregationStrategy) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.defaultStrategies[gvr] = strategy
}

// GetDefaultStrategy returns the default aggregation strategy for a resource type
func (a *Aggregator) GetDefaultStrategy(gvr schema.GroupVersionResource) interfaces.AggregationStrategy {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	if strategy, exists := a.defaultStrategies[gvr]; exists {
		return strategy
	}
	
	return interfaces.AggregationStrategyLatestWins
}

// aggregateLatestWins uses the most recent status update
func (a *Aggregator) aggregateLatestWins(ctx context.Context, updates []*interfaces.StatusUpdate) (*unstructured.Unstructured, error) {
	if len(updates) == 1 {
		return updates[0].Status.DeepCopy(), nil
	}

	// Sort by timestamp (most recent first)
	sortedUpdates := make([]*interfaces.StatusUpdate, len(updates))
	copy(sortedUpdates, updates)
	sort.Slice(sortedUpdates, func(i, j int) bool {
		return sortedUpdates[i].Timestamp.After(sortedUpdates[j].Timestamp)
	})

	latest := sortedUpdates[0].Status.DeepCopy()
	
	klog.V(4).InfoS("Latest wins aggregation", 
		"selectedSource", sortedUpdates[0].Source,
		"timestamp", sortedUpdates[0].Timestamp.Format(time.RFC3339))

	return latest, nil
}

// aggregateMergeAll attempts to merge all status updates
func (a *Aggregator) aggregateMergeAll(ctx context.Context, updates []*interfaces.StatusUpdate) (*unstructured.Unstructured, []interfaces.StatusConflict, error) {
	if a.merger == nil {
		return nil, nil, fmt.Errorf("merger is required for merge-all strategy")
	}

	// Use merger to combine all updates
	config := interfaces.MergeConfig{
		DefaultStrategy:  interfaces.FieldMergeLatest,
		ConflictBehavior: interfaces.ConflictBehaviorLog,
	}

	merged, err := a.merger.MergeFields(ctx, updates, config)
	if err != nil {
		return nil, nil, err
	}

	// For now, we don't track detailed conflicts in merge-all
	// This could be enhanced to capture conflicts detected during merging
	return merged, nil, nil
}

// aggregateConflictDetection detects and reports conflicts without resolution
func (a *Aggregator) aggregateConflictDetection(ctx context.Context, updates []*interfaces.StatusUpdate) (*unstructured.Unstructured, []interfaces.StatusConflict, error) {
	if len(updates) <= 1 {
		if len(updates) == 1 {
			return updates[0].Status.DeepCopy(), nil, nil
		}
		return nil, nil, nil
	}

	// Detect conflicts by comparing all pairs of updates
	conflicts := a.detectConflicts(updates)

	// Use the first update as the base result
	result := updates[0].Status.DeepCopy()

	klog.V(3).InfoS("Conflict detection completed", 
		"conflicts", len(conflicts),
		"updates", len(updates))

	return result, conflicts, nil
}

// aggregateSourcePriority uses predefined source priorities
func (a *Aggregator) aggregateSourcePriority(ctx context.Context, updates []*interfaces.StatusUpdate) (*unstructured.Unstructured, error) {
	if len(updates) == 1 {
		return updates[0].Status.DeepCopy(), nil
	}

	// Sort by source priority (highest first)
	sortedUpdates := make([]*interfaces.StatusUpdate, len(updates))
	copy(sortedUpdates, updates)
	sort.Slice(sortedUpdates, func(i, j int) bool {
		priorityI := a.getSourcePriority(sortedUpdates[i].Source)
		priorityJ := a.getSourcePriority(sortedUpdates[j].Source)
		
		// If priorities are equal, use timestamp as tiebreaker
		if priorityI == priorityJ {
			return sortedUpdates[i].Timestamp.After(sortedUpdates[j].Timestamp)
		}
		
		return priorityI > priorityJ
	})

	highest := sortedUpdates[0]
	
	klog.V(4).InfoS("Source priority aggregation", 
		"selectedSource", highest.Source,
		"priority", a.getSourcePriority(highest.Source),
		"timestamp", highest.Timestamp.Format(time.RFC3339))

	return highest.Status.DeepCopy(), nil
}

// getSourcePriority returns the priority for a source (default: 0)
func (a *Aggregator) getSourcePriority(source string) int {
	if a.sourcePriorities == nil {
		return 0
	}
	
	priority, exists := a.sourcePriorities[source]
	if !exists {
		return 0
	}
	
	return priority
}

// detectConflicts compares status updates to find conflicting field values
func (a *Aggregator) detectConflicts(updates []*interfaces.StatusUpdate) []interfaces.StatusConflict {
	var conflicts []interfaces.StatusConflict

	// Simple conflict detection - compare key status fields
	// In a real implementation, this would be more sophisticated
	statusFields := []string{"conditions", "phase", "replicas", "readyReplicas"}

	for _, fieldPath := range statusFields {
		fieldConflicts := a.detectFieldConflicts(updates, []string{"status", fieldPath})
		conflicts = append(conflicts, fieldConflicts...)
	}

	return conflicts
}

// detectFieldConflicts detects conflicts in a specific field across updates
func (a *Aggregator) detectFieldConflicts(updates []*interfaces.StatusUpdate, fieldPath []string) []interfaces.StatusConflict {
	// Extract field values from each update
	values := make(map[string]interface{})
	sources := make(map[string][]string)

	for _, update := range updates {
		if update.Status == nil {
			continue
		}

		value, found, err := unstructured.NestedFieldNoCopy(update.Status.Object, fieldPath...)
		if err != nil || !found {
			continue
		}

		// Convert value to string for comparison
		valueStr := fmt.Sprintf("%v", value)
		
		if existingSources, exists := sources[valueStr]; exists {
			sources[valueStr] = append(existingSources, update.Source)
		} else {
			values[valueStr] = value
			sources[valueStr] = []string{update.Source}
		}
	}

	// If we have more than one unique value, we have a conflict
	if len(values) <= 1 {
		return nil
	}

	fieldPathStr := fmt.Sprintf("%s", fieldPath)
	conflict := interfaces.StatusConflict{
		FieldPath:          fieldPathStr,
		ConflictingSources: []string{},
		Values:             make(map[string]interface{}),
		Resolution:         "no automatic resolution applied",
	}

	for valueStr, sourcesWithValue := range sources {
		conflict.ConflictingSources = append(conflict.ConflictingSources, sourcesWithValue...)
		for _, source := range sourcesWithValue {
			conflict.Values[source] = values[valueStr]
		}
	}

	return []interfaces.StatusConflict{conflict}
}