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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
)

func TestNewAggregator(t *testing.T) {
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
			defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, kcpfeatures.UpstreamSyncAggregation, tc.featureEnabled)()

			resolver := &mockResolver{}
			aggregator := NewAggregator(workloadv1alpha1.ConflictStrategyUseNewest, resolver)

			if tc.expectNoop {
				if _, ok := aggregator.(*noopAggregator); !ok {
					t.Errorf("expected noop aggregator when feature disabled")
				}
			} else {
				if _, ok := aggregator.(*aggregatorImpl); !ok {
					t.Errorf("expected real aggregator when feature enabled")
				}
			}
		})
	}
}

func TestAggregateStatus(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, kcpfeatures.UpstreamSyncAggregation, true)()

	tests := map[string]struct {
		resources     []ResourceStatus
		strategy      workloadv1alpha1.ConflictStrategy
		expectError   bool
		expectCached  bool
	}{
		"single resource": {
			resources: []ResourceStatus{
				{
					ClusterName: "cluster1",
					Resource:    newTestResource("test", "v1", "ConfigMap", "default", "test-config"),
					LastUpdated: time.Now(),
				},
			},
			strategy:    workloadv1alpha1.ConflictStrategyUseNewest,
			expectError: false,
		},
		"multiple resources newest": {
			resources: []ResourceStatus{
				{
					ClusterName: "cluster1",
					Resource:    newTestResource("test", "v1", "ConfigMap", "default", "test-config"),
					LastUpdated: time.Now().Add(-time.Hour),
				},
				{
					ClusterName: "cluster2",
					Resource:    newTestResource("test", "v1", "ConfigMap", "default", "test-config"),
					LastUpdated: time.Now(),
				},
			},
			strategy:    workloadv1alpha1.ConflictStrategyUseNewest,
			expectError: false,
		},
		"no resources": {
			resources:   []ResourceStatus{},
			strategy:    workloadv1alpha1.ConflictStrategyUseNewest,
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resolver := &mockResolver{}
			aggregator := NewAggregator(tc.strategy, resolver).(*aggregatorImpl)

			ctx := context.Background()
			result, err := aggregator.AggregateStatus(ctx, tc.resources)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
				return
			}

			if len(result.SourceStatuses) != len(tc.resources) {
				t.Errorf("expected %d source statuses, got %d", len(tc.resources), len(result.SourceStatuses))
			}
		})
	}
}

func newTestResource(apiVersion, version, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion + "/" + version,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

type mockResolver struct {
	resolveFunc func(context.Context, Conflict) (*Resolution, error)
}

func (m *mockResolver) Resolve(ctx context.Context, conflict Conflict) (*Resolution, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, conflict)
	}
	return &Resolution{
		Conflict:       conflict,
		ResolvedStatus: &conflict.Statuses[0],
		Strategy:       "mock",
		Timestamp:      time.Now(),
	}, nil
}

func (m *mockResolver) SetStrategy(strategy workloadv1alpha1.ConflictStrategy) {}
func (m *mockResolver) CanAutoResolve(conflict Conflict) bool                { return true }