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

package upstream

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AggregationStrategy defines different ways to aggregate status from multiple clusters
type AggregationStrategy int

const (
	// Latest uses the most recently updated status (default for single cluster)
	AggregationStrategyLatest AggregationStrategy = iota
)

// StatusAggregator handles aggregating status from multiple downstream clusters
type StatusAggregator struct {
	// Default strategy for single cluster scenarios
	defaultStrategy AggregationStrategy
}

// NewStatusAggregator creates a new status aggregator
func NewStatusAggregator() *StatusAggregator {
	return &StatusAggregator{
		defaultStrategy: AggregationStrategyLatest,
	}
}

// AggregateStatus aggregates status from multiple downstream clusters
// For now, simple implementation that returns the first status (single cluster)
func (a *StatusAggregator) AggregateStatus(gvr schema.GroupVersionResource, statuses []interface{}) (interface{}, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	
	// Simple implementation: return first status
	// Future versions can implement more sophisticated aggregation
	return statuses[0], nil
}