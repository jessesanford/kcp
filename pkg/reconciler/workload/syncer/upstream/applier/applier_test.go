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

package applier

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"

	"github.com/kcp-dev/kcp/pkg/reconciler/workload/syncer/upstream"
	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
)

func TestNewApplier(t *testing.T) {
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

			client := fake.NewSimpleDynamicClient(runtime.NewScheme())
			applier := NewApplier(client)

			if tc.expectNoop {
				if _, ok := applier.(*noopApplier); !ok {
					t.Errorf("expected noop applier when feature disabled")
				}
			} else {
				if _, ok := applier.(*applierImpl); !ok {
					t.Errorf("expected real applier when feature enabled")
				}
			}
		})
	}
}

func TestSetDryRun(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, kcpfeatures.UpstreamSyncAggregation, true)()

	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	applier := NewApplier(client).(*applierImpl)

	if applier.dryRun {
		t.Errorf("expected dry-run to be false by default")
	}

	applier.SetDryRun(true)
	if !applier.dryRun {
		t.Errorf("expected dry-run to be true after setting")
	}

	applier.SetDryRun(false)
	if applier.dryRun {
		t.Errorf("expected dry-run to be false after unsetting")
	}
}

func TestGetAppliedCount(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, kcpfeatures.UpstreamSyncAggregation, true)()

	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	applier := NewApplier(client).(*applierImpl)

	if count := applier.GetAppliedCount(); count != 0 {
		t.Errorf("expected applied count to be 0, got %d", count)
	}
}

func TestApplyBatch(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, kcpfeatures.UpstreamSync, true)()
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, kcpfeatures.UpstreamSyncAggregation, true)()

	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	applier := NewApplier(client).(*applierImpl)

	updates := []*upstream.Update{
		{
			Type:     upstream.UpdateTypeCreate,
			Resource: newTestResource(),
			Strategy: upstream.ApplyStrategyClientSide,
		},
	}

	ctx := context.Background()
	err := applier.ApplyBatch(ctx, updates)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func newTestResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
			},
		},
	}
}