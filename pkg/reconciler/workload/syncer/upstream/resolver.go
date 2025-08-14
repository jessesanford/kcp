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
	"time"
	
	"k8s.io/klog/v2"
	
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// resolverImpl implements ConflictResolver
type resolverImpl struct {
	strategy workloadv1alpha1.ConflictStrategy
}

// NewResolver creates a new conflict resolver
func NewResolver(strategy workloadv1alpha1.ConflictStrategy) ConflictResolver {
	if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncConflictResolution) {
		return &noopResolver{}
	}
	
	return &resolverImpl{
		strategy: strategy,
	}
}

// Resolve attempts to resolve a conflict
func (r *resolverImpl) Resolve(ctx context.Context, conflict Conflict) (*Resolution, error) {
	if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSyncConflictResolution) {
		return nil, fmt.Errorf("conflict resolution is disabled")
	}
	
	klog.V(4).Infof("Resolving conflict for %s", conflict.ResourceKey)
	
	if len(conflict.Statuses) == 0 {
		return nil, fmt.Errorf("no statuses to resolve")
	}
	
	// Use newest resource by default
	newest := conflict.Statuses[0]
	for _, status := range conflict.Statuses[1:] {
		if status.LastUpdated.After(newest.LastUpdated) {
			newest = status
		}
	}
	
	resolution := &Resolution{
		Conflict:       conflict,
		ResolvedStatus: &newest,
		Strategy:       string(r.strategy),
		Timestamp:      time.Now(),
	}
	
	return resolution, nil
}

// SetStrategy updates the resolution strategy
func (r *resolverImpl) SetStrategy(strategy workloadv1alpha1.ConflictStrategy) {
	r.strategy = strategy
}

// CanAutoResolve checks if conflict can be automatically resolved
func (r *resolverImpl) CanAutoResolve(conflict Conflict) bool {
	return conflict.Severity != ConflictSeverityHigh && len(conflict.Statuses) > 0
}

// noopResolver when feature is disabled
type noopResolver struct{}

func (n *noopResolver) Resolve(ctx context.Context, conflict Conflict) (*Resolution, error) {
	return nil, fmt.Errorf("resolver disabled")
}
func (n *noopResolver) SetStrategy(strategy workloadv1alpha1.ConflictStrategy) {}
func (n *noopResolver) CanAutoResolve(conflict Conflict) bool { return false }