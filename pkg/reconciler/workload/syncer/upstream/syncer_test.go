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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	"github.com/kcp-dev/logicalcluster/v3"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	kcpfakeclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/fake"
	workloadv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/workload/v1alpha1"
)

func TestNewUpstreamSyncer(t *testing.T) {
	tests := map[string]struct {
		syncInterval      time.Duration
		expectedInterval  time.Duration
		expectedError     bool
	}{
		"default interval": {
			syncInterval:     0,
			expectedInterval: DefaultSyncInterval,
			expectedError:    false,
		},
		"custom interval": {
			syncInterval:     60 * time.Second,
			expectedInterval: 60 * time.Second,
			expectedError:    false,
		},
		"negative interval": {
			syncInterval:     -10 * time.Second,
			expectedInterval: DefaultSyncInterval,
			expectedError:    false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create fake clients and informers
			kcpClient := kcpfakeclientset.NewSimpleClientset()
			informer := workloadv1alpha1informers.NewSyncTargetInformer(
				kcpClient.WorkloadV1alpha1().SyncTargets(),
				0,
				cache.Indexers{},
			)

			// Create upstream syncer
			syncer, err := NewUpstreamSyncer(kcpClient, informer, tc.syncInterval)

			if tc.expectedError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if syncer == nil {
				t.Fatal("syncer should not be nil")
			}

			if syncer.syncInterval != tc.expectedInterval {
				t.Errorf("expected sync interval %v, got %v", tc.expectedInterval, syncer.syncInterval)
			}

			if syncer.discoveryManager == nil {
				t.Error("discovery manager should not be nil")
			}

			if syncer.conflictResolver == nil {
				t.Error("conflict resolver should not be nil")
			}

			if syncer.statusAggregator == nil {
				t.Error("status aggregator should not be nil")
			}
		})
	}
}

func TestUpstreamSyncer_isSyncTargetReady(t *testing.T) {
	tests := map[string]struct {
		syncTarget    *workloadv1alpha1.SyncTarget
		expectedReady bool
	}{
		"ready sync target": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Status: workloadv1alpha1.SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{
						{
							Type:   workloadv1alpha1.SyncTargetReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expectedReady: true,
		},
		"not ready sync target": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Status: workloadv1alpha1.SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{
						{
							Type:   workloadv1alpha1.SyncTargetReady,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			expectedReady: false,
		},
		"no conditions": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Status: workloadv1alpha1.SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{},
				},
			},
			expectedReady: false,
		},
		"wrong condition type": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Status: workloadv1alpha1.SyncTargetStatus{
					Conditions: conditionsv1alpha1.Conditions{
						{
							Type:   workloadv1alpha1.SyncTargetSyncerReady,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			expectedReady: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			syncer := &UpstreamSyncer{}
			ready := syncer.isSyncTargetReady(tc.syncTarget)

			if ready != tc.expectedReady {
				t.Errorf("expected ready=%v, got ready=%v", tc.expectedReady, ready)
			}
		})
	}
}

func TestResourceTransformer_transformToKCP(t *testing.T) {
	tests := map[string]struct {
		physicalResource *unstructured.Unstructured
		syncTarget      *workloadv1alpha1.SyncTarget
		workspace       logicalcluster.Name
		gvr             schema.GroupVersionResource
		validateResult  func(t *testing.T, result *unstructured.Unstructured)
	}{
		"transform pod resource": {
			physicalResource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":            "test-pod",
						"namespace":       "default",
						"uid":             "physical-uid-123",
						"resourceVersion": "12345",
						"generation":      int64(1),
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test-container",
								"image": "nginx",
							},
						},
					},
					"status": map[string]interface{}{
						"phase": "Running",
					},
				},
			},
			syncTarget: &workloadv1alpha1.SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-target",
				},
				Spec: workloadv1alpha1.SyncTargetSpec{
					Location: "us-west-2",
				},
			},
			workspace: logicalcluster.Name("root:test"),
			gvr:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			validateResult: func(t *testing.T, result *unstructured.Unstructured) {
				// Check that cluster-specific metadata was cleared
				if result.GetUID() != "" {
					t.Error("UID should be cleared")
				}
				if result.GetResourceVersion() != "" {
					t.Error("ResourceVersion should be cleared")  
				}
				if result.GetGeneration() != 0 {
					t.Error("Generation should be cleared")
				}

				// Check workspace annotations
				annotations := result.GetAnnotations()
				if annotations == nil {
					t.Fatal("annotations should not be nil")
				}

				if annotations[WorkspaceAnnotation] != "root:test" {
					t.Errorf("expected workspace annotation 'root:test', got %s", annotations[WorkspaceAnnotation])
				}

				if annotations[SyncTargetAnnotation] != "test-target" {
					t.Errorf("expected sync target annotation 'test-target', got %s", annotations[SyncTargetAnnotation])
				}

				if annotations[SourceClusterAnnotation] != "us-west-2" {
					t.Errorf("expected source cluster annotation 'us-west-2', got %s", annotations[SourceClusterAnnotation])
				}

				if annotations[PhysicalUIDAnnotation] != "physical-uid-123" {
					t.Errorf("expected physical UID annotation 'physical-uid-123', got %s", annotations[PhysicalUIDAnnotation])
				}

				// Check sync labels
				labels := result.GetLabels()
				if labels == nil {
					t.Fatal("labels should not be nil")
				}

				if labels[UpstreamSyncLabel] != "true" {
					t.Errorf("expected upstream sync label 'true', got %s", labels[UpstreamSyncLabel])
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			transformer := newResourceTransformer(tc.syncTarget, tc.workspace)
			ctx := context.Background()

			result, err := transformer.transformToKCP(ctx, tc.physicalResource, tc.gvr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result should not be nil")
			}

			tc.validateResult(t, result)
		})
	}
}

func TestConflictResolver_detectConflictType(t *testing.T) {
	tests := map[string]struct {
		kcpResource      *unstructured.Unstructured
		physicalResource *unstructured.Unstructured
		expectedType     string
		expectedReason   string
	}{
		"no conflict": {
			kcpResource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			physicalResource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			expectedType:   "",
			expectedReason: "",
		},
		"generation mismatch": {
			kcpResource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": int64(2),
					},
				},
			},
			physicalResource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
				},
			},
			expectedType:   "generation-mismatch",
			expectedReason: "KCP generation: 2, Physical generation: 1",
		},
		"spec divergence": {
			kcpResource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			physicalResource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generation": int64(1),
					},
					"spec": map[string]interface{}{
						"replicas": int64(5),
					},
				},
			},
			expectedType:   "spec-divergence",
			expectedReason: "Spec fields differ between KCP and physical cluster",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resolver := newConflictResolver()
			
			conflictType, conflictReason := resolver.detectConflictType(tc.kcpResource, tc.physicalResource)
			
			if conflictType != tc.expectedType {
				t.Errorf("expected conflict type %q, got %q", tc.expectedType, conflictType)
			}
			
			if conflictReason != tc.expectedReason {
				t.Errorf("expected conflict reason %q, got %q", tc.expectedReason, conflictReason)
			}
		})
	}
}

func TestStatusAggregator_determinePodHealth(t *testing.T) {
	tests := map[string]struct {
		pod            unstructured.Unstructured
		expectedHealth ResourceHealth
	}{
		"running pod with ready containers": {
			pod: unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "Running",
						"containerStatuses": []interface{}{
							map[string]interface{}{
								"ready": true,
							},
							map[string]interface{}{
								"ready": true,
							},
						},
					},
				},
			},
			expectedHealth: HealthHealthy,
		},
		"running pod with some containers not ready": {
			pod: unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "Running",
						"containerStatuses": []interface{}{
							map[string]interface{}{
								"ready": true,
							},
							map[string]interface{}{
								"ready": false,
							},
						},
					},
				},
			},
			expectedHealth: HealthPending,
		},
		"succeeded pod": {
			pod: unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "Succeeded",
					},
				},
			},
			expectedHealth: HealthHealthy,
		},
		"failed pod": {
			pod: unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "Failed",
					},
				},
			},
			expectedHealth: HealthUnhealthy,
		},
		"pending pod": {
			pod: unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "Pending",
					},
				},
			},
			expectedHealth: HealthPending,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			aggregator := newStatusAggregator()
			
			health := aggregator.determinePodHealth(tc.pod)
			
			if health != tc.expectedHealth {
				t.Errorf("expected health %s, got %s", tc.expectedHealth, health)
			}
		})
	}
}

func TestDiscoveryManager_filterResourcesForSync(t *testing.T) {
	tests := map[string]struct {
		syncTarget *workloadv1alpha1.SyncTarget
		resources  map[schema.GroupVersionResource]*discoveredResource
		expected   int
	}{
		"no filter - all resources": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Spec: workloadv1alpha1.SyncTargetSpec{
					SupportedResourceTypes: []string{},
				},
			},
			resources: map[schema.GroupVersionResource]*discoveredResource{
				{Resource: "pods"}:        {Verbs: testVerbs()},
				{Resource: "services"}:    {Verbs: testVerbs()},
				{Resource: "deployments"}: {Verbs: testVerbs()},
			},
			expected: 3,
		},
		"filter by supported types": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Spec: workloadv1alpha1.SyncTargetSpec{
					SupportedResourceTypes: []string{"pods", "services"},
				},
			},
			resources: map[schema.GroupVersionResource]*discoveredResource{
				{Resource: "pods"}:        {Verbs: testVerbs()},
				{Resource: "services"}:    {Verbs: testVerbs()},
				{Resource: "deployments"}: {Verbs: testVerbs()},
			},
			expected: 2,
		},
		"filter by missing verbs": {
			syncTarget: &workloadv1alpha1.SyncTarget{
				Spec: workloadv1alpha1.SyncTargetSpec{
					SupportedResourceTypes: []string{},
				},
			},
			resources: map[schema.GroupVersionResource]*discoveredResource{
				{Resource: "pods"}:     {Verbs: testVerbs()},
				{Resource: "services"}: {Verbs: testVerbsLimited()}, // Missing required verbs
			},
			expected: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dm := &discoveryManager{}
			
			filtered := dm.filterResourcesForSync(tc.syncTarget, tc.resources)
			
			if len(filtered) != tc.expected {
				t.Errorf("expected %d filtered resources, got %d", tc.expected, len(filtered))
			}
		})
	}
}

// Helper functions for tests

func testVerbs() sets.Set[string] {
	return sets.New("get", "list", "watch", "create", "update", "patch", "delete")
}

func testVerbsLimited() sets.Set[string] {
	return sets.New("get", "list")
}