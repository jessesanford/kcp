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
	"testing"
	"time"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
)

func TestNewResolver(t *testing.T) {
	tests := map[string]struct {
		featureEnabled bool
		expectNoop     bool
	}{
		"feature enabled": {
			featureEnabled: true,
			expectNoop:     false,
		},
		"feature disabled": {
			featureEnabled: false,
			expectNoop:     true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, kcpfeatures.UpstreamSyncConflictResolution, tc.featureEnabled)()

			resolver := NewResolver(workloadv1alpha1.ConflictStrategyUseNewest)

			if tc.expectNoop {
				if _, ok := resolver.(*noopResolver); !ok {
					t.Errorf("expected noop resolver when feature disabled")
				}
			} else {
				if _, ok := resolver.(*resolverImpl); !ok {
					t.Errorf("expected real resolver when feature enabled")
				}
			}
		})
	}
}

func TestCanAutoResolve(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, kcpfeatures.UpstreamSyncConflictResolution, true)()

	tests := map[string]struct {
		conflict    Conflict
		strategy    workloadv1alpha1.ConflictStrategy
		canResolve  bool
	}{
		"high severity conflict": {
			conflict: Conflict{
				Severity: ConflictSeverityHigh,
				Type:     ConflictTypeStatus,
				Statuses: []ResourceStatus{{ClusterName: "test"}},
			},
			strategy:   workloadv1alpha1.ConflictStrategyUseNewest,
			canResolve: false,
		},
		"manual strategy": {
			conflict: Conflict{
				Severity: ConflictSeverityLow,
				Type:     ConflictTypeStatus,
				Statuses: []ResourceStatus{{ClusterName: "test"}},
			},
			strategy:   workloadv1alpha1.ConflictStrategyManual,
			canResolve: false,
		},
		"generation conflict with newest": {
			conflict: Conflict{
				Severity: ConflictSeverityMedium,
				Type:     ConflictTypeGeneration,
				Statuses: []ResourceStatus{{ClusterName: "test"}},
			},
			strategy:   workloadv1alpha1.ConflictStrategyUseNewest,
			canResolve: true,
		},
		"status conflict": {
			conflict: Conflict{
				Severity: ConflictSeverityLow,
				Type:     ConflictTypeStatus,
				Statuses: []ResourceStatus{{ClusterName: "test"}},
			},
			strategy:   workloadv1alpha1.ConflictStrategyUseNewest,
			canResolve: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resolver := NewResolver(tc.strategy).(*resolverImpl)
			canResolve := resolver.CanAutoResolve(tc.conflict)

			if canResolve != tc.canResolve {
				t.Errorf("expected CanAutoResolve to return %v, got %v", tc.canResolve, canResolve)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, kcpfeatures.UpstreamSyncConflictResolution, true)()

	now := time.Now()
	conflict := Conflict{
		ResourceKey: "test-resource",
		Statuses: []ResourceStatus{
			{ClusterName: "cluster1", LastUpdated: now.Add(-time.Hour)},
			{ClusterName: "cluster2", LastUpdated: now},
		},
		Type:     ConflictTypeStatus,
		Severity: ConflictSeverityLow,
	}

	resolver := NewResolver(workloadv1alpha1.ConflictStrategyUseNewest).(*resolverImpl)
	ctx := context.Background()

	resolution, err := resolver.Resolve(ctx, conflict)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if resolution == nil {
		t.Errorf("expected resolution but got nil")
		return
	}

	if resolution.ResolvedStatus.ClusterName != "cluster2" {
		t.Errorf("expected cluster2 to be selected as newest, got %s", resolution.ResolvedStatus.ClusterName)
	}
}