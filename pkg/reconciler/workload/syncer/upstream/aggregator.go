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
	"context"
	"fmt"
	"sync"
	"time"
	
	"k8s.io/klog/v2"
	
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// aggregatorImpl implements StatusAggregator
type aggregatorImpl struct {
	strategy workloadv1alpha1.ConflictStrategy
	resolver ConflictResolver
	mu       sync.RWMutex
	lastAggregation *AggregatedStatus
}

// NewAggregator creates a new status aggregator
func NewAggregator(strategy workloadv1alpha1.ConflictStrategy, resolver ConflictResolver) StatusAggregator {
	if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncAggregation) {
		return &noopAggregator{}
	}
	
	return &aggregatorImpl{
		strategy: strategy,
		resolver: resolver,
	}
}

// AggregateStatus combines status from multiple clusters
func (a *aggregatorImpl) AggregateStatus(ctx context.Context, resources []ResourceStatus) (*AggregatedStatus, error) {
	if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncAggregation) {
		return nil, nil
	}
	
	if len(resources) == 0 {
		return nil, fmt.Errorf("no resources to aggregate")
	}
	
	klog.V(4).Infof("Aggregating status from %d resources", len(resources))
	
	// Use newest resource by default
	chosen := &resources[0]
	for i := range resources {
		if resources[i].LastUpdated.After(chosen.LastUpdated) {
			chosen = &resources[i]
		}
	}
	
	aggregated := &AggregatedStatus{
		ResourceKey:       fmt.Sprintf("%s/%s", chosen.Resource.GetNamespace(), chosen.Resource.GetName()),
		CombinedStatus:    chosen.Resource.DeepCopy(),
		SourceStatuses:    resources,
		AggregationTime:   time.Now(),
		ConflictsResolved: len(resources) - 1,
	}
	
	a.mu.Lock()
	a.lastAggregation = aggregated
	a.mu.Unlock()
	
	return aggregated, nil
}

// ResolveConflicts handles conflicting status
func (a *aggregatorImpl) ResolveConflicts(ctx context.Context, conflicts []Conflict) (*Resolution, error) {
	if len(conflicts) == 0 {
		return nil, fmt.Errorf("no conflicts to resolve")
	}
	
	return a.resolver.Resolve(ctx, conflicts[0])
}

// SetStrategy updates the conflict resolution strategy
func (a *aggregatorImpl) SetStrategy(strategy workloadv1alpha1.ConflictStrategy) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.strategy = strategy
}

// GetLastAggregation returns the last aggregation result
func (a *aggregatorImpl) GetLastAggregation() *AggregatedStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastAggregation
}

// noopAggregator when feature is disabled
type noopAggregator struct{}

func (n *noopAggregator) AggregateStatus(ctx context.Context, resources []ResourceStatus) (*AggregatedStatus, error) {
	return nil, nil
}
func (n *noopAggregator) ResolveConflicts(ctx context.Context, conflicts []Conflict) (*Resolution, error) {
	return nil, nil
}
func (n *noopAggregator) SetStrategy(strategy workloadv1alpha1.ConflictStrategy) {}
func (n *noopAggregator) GetLastAggregation() *AggregatedStatus { return nil }