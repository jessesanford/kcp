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

package engine

import (
	"context"
	"testing"

	tmcv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRoundRobinEngine_SelectClusters(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		workload         *tmcv1alpha1.WorkloadPlacement
		clusters         []*tmcv1alpha1.ClusterRegistration
		expectedCount    int
		expectedClusters []string
		wantError        bool
	}{
		"single cluster selection": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					NumberOfClusters: int32Ptr(1),
					ClusterSelector:  tmcv1alpha1.ClusterSelector{},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster-2"}},
			},
			expectedCount:    1,
			expectedClusters: []string{"cluster-1"}, // First in alphabetical order
			wantError:        false,
		},
		"multiple cluster selection": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					NumberOfClusters: int32Ptr(2),
					ClusterSelector:  tmcv1alpha1.ClusterSelector{},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster-a"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster-b"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "cluster-c"}},
			},
			expectedCount:    2,
			expectedClusters: []string{"cluster-a", "cluster-b"},
			wantError:        false,
		},
		"location selector filtering": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					NumberOfClusters: int32Ptr(1),
					ClusterSelector: tmcv1alpha1.ClusterSelector{
						LocationSelector: []string{"us-west"},
					},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-west"},
					Spec:       tmcv1alpha1.ClusterRegistrationSpec{Location: "us-west"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-east"},
					Spec:       tmcv1alpha1.ClusterRegistrationSpec{Location: "us-east"},
				},
			},
			expectedCount:    1,
			expectedClusters: []string{"cluster-west"},
			wantError:        false,
		},
		"explicit cluster names": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					NumberOfClusters: int32Ptr(1),
					ClusterSelector: tmcv1alpha1.ClusterSelector{
						ClusterNames: []string{"target-cluster"},
					},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				{ObjectMeta: metav1.ObjectMeta{Name: "other-cluster"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "target-cluster"}},
			},
			expectedCount:    1,
			expectedClusters: []string{"target-cluster"},
			wantError:        false,
		},
		"label selector filtering": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					NumberOfClusters: int32Ptr(1),
					ClusterSelector: tmcv1alpha1.ClusterSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"environment": "prod",
							},
						},
					},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "prod-cluster",
						Labels: map[string]string{"environment": "prod"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "dev-cluster",
						Labels: map[string]string{"environment": "dev"},
					},
				},
			},
			expectedCount:    1,
			expectedClusters: []string{"prod-cluster"},
			wantError:        false,
		},
		"no eligible clusters": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					ClusterSelector: tmcv1alpha1.ClusterSelector{
						LocationSelector: []string{"mars"},
					},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "earth-cluster"},
					Spec:       tmcv1alpha1.ClusterRegistrationSpec{Location: "earth"},
				},
			},
			wantError: true,
		},
		"nil workload": {
			workload:  nil,
			clusters:  []*tmcv1alpha1.ClusterRegistration{},
			wantError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			engine := NewRoundRobinEngine()
			
			decisions, err := engine.SelectClusters(ctx, tc.workload, tc.clusters)
			
			if tc.wantError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(decisions) != tc.expectedCount {
				t.Errorf("Expected %d decisions, got %d", tc.expectedCount, len(decisions))
				return
			}

			// Verify expected clusters are selected
			for i, expectedCluster := range tc.expectedClusters {
				if i >= len(decisions) {
					t.Errorf("Expected cluster %s not found in decisions", expectedCluster)
					continue
				}
				if decisions[i].ClusterName != expectedCluster {
					t.Errorf("Expected cluster %s at position %d, got %s",
						expectedCluster, i, decisions[i].ClusterName)
				}
			}

			// Verify decision properties
			for i, decision := range decisions {
				if decision.ClusterName == "" {
					t.Errorf("Decision %d has empty cluster name", i)
				}
				if decision.Score <= 0 {
					t.Errorf("Decision %d has invalid score: %d", i, decision.Score)
				}
				if decision.Reason == "" {
					t.Errorf("Decision %d has empty reason", i)
				}
			}
		})
	}
}

func TestRoundRobinEngine_RoundRobinDistribution(t *testing.T) {
	ctx := context.Background()
	engine := NewRoundRobinEngine()

	clusters := []*tmcv1alpha1.ClusterRegistration{
		{ObjectMeta: metav1.ObjectMeta{Name: "cluster-a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cluster-b"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cluster-c"}},
	}

	workload := &tmcv1alpha1.WorkloadPlacement{
		ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
		Spec: tmcv1alpha1.WorkloadPlacementSpec{
			NumberOfClusters: int32Ptr(1),
			ClusterSelector:  tmcv1alpha1.ClusterSelector{},
		},
	}

	// Track selections across multiple placement requests
	selections := make(map[string]int)
	expectedOrder := []string{"cluster-a", "cluster-b", "cluster-c"}

	// Run multiple selections to verify round-robin behavior
	for i := 0; i < 6; i++ {
		decisions, err := engine.SelectClusters(ctx, workload, clusters)
		if err != nil {
			t.Fatalf("Unexpected error on iteration %d: %v", i, err)
		}

		if len(decisions) != 1 {
			t.Fatalf("Expected 1 decision, got %d on iteration %d", len(decisions), i)
		}

		selected := decisions[0].ClusterName
		selections[selected]++

		// Verify round-robin order
		expectedCluster := expectedOrder[i%len(expectedOrder)]
		if selected != expectedCluster {
			t.Errorf("Iteration %d: expected %s, got %s", i, expectedCluster, selected)
		}
	}

	// Verify even distribution (each cluster selected twice in 6 iterations)
	for cluster, count := range selections {
		if count != 2 {
			t.Errorf("Cluster %s selected %d times, expected 2", cluster, count)
		}
	}
}

func TestRoundRobinEngine_SelectorKeyGeneration(t *testing.T) {
	engine := NewRoundRobinEngine()

	tests := map[string]struct {
		selector    *tmcv1alpha1.ClusterSelector
		expectedKey string
	}{
		"nil selector": {
			selector:    nil,
			expectedKey: "default",
		},
		"empty selector": {
			selector:    &tmcv1alpha1.ClusterSelector{},
			expectedKey: "default",
		},
		"location selector": {
			selector: &tmcv1alpha1.ClusterSelector{
				LocationSelector: []string{"us-west", "us-east"},
			},
			expectedKey: "|locations:[us-west us-east]",
		},
		"cluster names": {
			selector: &tmcv1alpha1.ClusterSelector{
				ClusterNames: []string{"cluster-1", "cluster-2"},
			},
			expectedKey: "|names:[cluster-1 cluster-2]",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			key := engine.generateSelectorKey(tc.selector)
			if key != tc.expectedKey {
				t.Errorf("Expected key %s, got %s", tc.expectedKey, key)
			}
		})
	}
}

// Helper function to create int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}