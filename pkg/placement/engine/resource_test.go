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

func TestResourceAwareEngine_SelectClusters(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		workload         *tmcv1alpha1.WorkloadPlacement
		clusters         []*tmcv1alpha1.ClusterRegistration
		expectedCount    int
		expectedOrder    []string // Expected cluster selection order
		wantError        bool
	}{
		"least loaded strategy - single cluster": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					NumberOfClusters: int32Ptr(1),
					ClusterSelector:  tmcv1alpha1.ClusterSelector{},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				createClusterWithResources("cluster-high-load", 1000, 1000, 10, 800, 800, 9), // 80% utilization
				createClusterWithResources("cluster-low-load", 1000, 1000, 10, 200, 200, 2),  // 20% utilization
			},
			expectedCount: 1,
			expectedOrder: []string{"cluster-low-load"}, // Should pick least loaded
			wantError:     false,
		},
		"least loaded strategy - multiple clusters": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					NumberOfClusters: int32Ptr(2),
					ClusterSelector:  tmcv1alpha1.ClusterSelector{},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				createClusterWithResources("cluster-high-load", 1000, 1000, 10, 900, 900, 9),   // 90% utilization
				createClusterWithResources("cluster-medium-load", 1000, 1000, 10, 500, 500, 5), // 50% utilization
				createClusterWithResources("cluster-low-load", 1000, 1000, 10, 100, 100, 1),    // 10% utilization
			},
			expectedCount: 2,
			expectedOrder: []string{"cluster-low-load", "cluster-medium-load"}, // Sorted by utilization
			wantError:     false,
		},
		"clusters without capacity information": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					NumberOfClusters: int32Ptr(1),
					ClusterSelector:  tmcv1alpha1.ClusterSelector{},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster-no-info"},
					Spec:       tmcv1alpha1.ClusterRegistrationSpec{Location: "us-west"},
				},
			},
			expectedCount: 1,
			expectedOrder: []string{"cluster-no-info"}, // Should include cluster without capacity info
			wantError:     false,
		},
		"resource filtering - over capacity clusters excluded": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					NumberOfClusters: int32Ptr(1),
					ClusterSelector:  tmcv1alpha1.ClusterSelector{},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				createClusterWithResources("cluster-overloaded", 1000, 1000, 10, 950, 950, 10), // >90% utilization
				createClusterWithResources("cluster-available", 1000, 1000, 10, 300, 300, 3),   // 30% utilization
			},
			expectedCount: 1,
			expectedOrder: []string{"cluster-available"}, // Overloaded cluster should be filtered out
			wantError:     false,
		},
		"no available clusters after resource filtering": {
			workload: &tmcv1alpha1.WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: tmcv1alpha1.WorkloadPlacementSpec{
					NumberOfClusters: int32Ptr(1),
					ClusterSelector:  tmcv1alpha1.ClusterSelector{},
				},
			},
			clusters: []*tmcv1alpha1.ClusterRegistration{
				createClusterWithResources("cluster-1", 1000, 1000, 10, 950, 950, 10), // >90% utilization
				createClusterWithResources("cluster-2", 1000, 1000, 10, 960, 960, 10), // >90% utilization
			},
			wantError: true, // Should fail when no clusters have available resources
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			engine := NewResourceAwareEngine()
			
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

			// Verify expected cluster selection order
			for i, expectedCluster := range tc.expectedOrder {
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
				// Resource-aware decisions should include utilization information
				if len(decision.Reason) < 20 { // Reasonable minimum for detailed reason
					t.Errorf("Decision %d has suspiciously short reason: %s", i, decision.Reason)
				}
			}
		})
	}
}

func TestResourceAwareEngine_Strategies(t *testing.T) {
	ctx := context.Background()

	clusters := []*tmcv1alpha1.ClusterRegistration{
		createClusterWithResources("cluster-low", 1000, 1000, 10, 100, 100, 1),    // 10% utilization
		createClusterWithResources("cluster-medium", 1000, 1000, 10, 500, 500, 5), // 50% utilization
		createClusterWithResources("cluster-high", 1000, 1000, 10, 800, 800, 8),   // 80% utilization
	}

	workload := &tmcv1alpha1.WorkloadPlacement{
		ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
		Spec: tmcv1alpha1.WorkloadPlacementSpec{
			NumberOfClusters: int32Ptr(2),
			ClusterSelector:  tmcv1alpha1.ClusterSelector{},
		},
	}

	tests := map[string]struct {
		strategy      ResourceStrategy
		expectedFirst string // Expected first cluster selection
	}{
		"least loaded strategy": {
			strategy:      LeastLoadedStrategy,
			expectedFirst: "cluster-low", // Should pick least loaded first
		},
		"best fit strategy": {
			strategy:      BestFitStrategy,
			expectedFirst: "cluster-low", // Currently falls back to least loaded
		},
		"balanced strategy": {
			strategy:      BalancedStrategy,
			expectedFirst: "cluster-medium", // Balanced strategy prefers moderate utilization
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			engine := NewResourceAwareEngineWithStrategy(tc.strategy)
			
			decisions, err := engine.SelectClusters(ctx, workload, clusters)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(decisions) < 1 {
				t.Errorf("Expected at least 1 decision")
				return
			}

			if decisions[0].ClusterName != tc.expectedFirst {
				t.Errorf("Expected first cluster %s, got %s", tc.expectedFirst, decisions[0].ClusterName)
			}

			// Verify strategy-specific reason format
			reason := decisions[0].Reason
			switch tc.strategy {
			case LeastLoadedStrategy:
				if !contains(reason, "LeastLoaded strategy") {
					t.Errorf("Expected LeastLoaded strategy reason, got: %s", reason)
				}
			case BestFitStrategy:
				if !contains(reason, "BestFit strategy") {
					t.Errorf("Expected BestFit strategy reason, got: %s", reason)
				}
			case BalancedStrategy:
				if !contains(reason, "Balanced strategy") {
					t.Errorf("Expected Balanced strategy reason, got: %s", reason)
				}
			}
		})
	}
}

func TestResourceAwareEngine_UtilizationCalculation(t *testing.T) {
	engine := NewResourceAwareEngine()

	tests := map[string]struct {
		cluster              *tmcv1alpha1.ClusterRegistration
		expectedUtilization  float64
		tolerancePercentage  float64 // Allow some tolerance in calculation
	}{
		"no resource info": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster-no-info"},
			},
			expectedUtilization: 0.1, // Default 10%
			tolerancePercentage: 0.01,
		},
		"mixed resource utilization": {
			cluster:             createClusterWithResources("mixed-cluster", 1000, 2000, 20, 300, 800, 5),
			expectedUtilization: (0.3 + 0.4 + 0.25) / 3, // 30% CPU, 40% memory, 25% pods = 31.7% average
			tolerancePercentage: 0.02,
		},
		"high utilization": {
			cluster:             createClusterWithResources("high-cluster", 1000, 1000, 10, 900, 950, 9),
			expectedUtilization: (0.9 + 0.95 + 0.9) / 3, // 90% CPU, 95% memory, 90% pods = 91.7% average
			tolerancePercentage: 0.02,
		},
		"partial resource tracking": {
			cluster: &tmcv1alpha1.ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "partial-cluster"},
				Spec: tmcv1alpha1.ClusterRegistrationSpec{
					Location: "us-west",
					Capacity: tmcv1alpha1.ClusterCapacity{
						CPU: int64Ptr(1000),
						// Memory not specified
					},
				},
				Status: tmcv1alpha1.ClusterRegistrationStatus{
					AllocatedResources: &tmcv1alpha1.ClusterResourceUsage{
						CPU: int64Ptr(500),
						// Memory allocation not tracked
					},
				},
			},
			expectedUtilization: 0.5, // 50% CPU utilization, only resource tracked
			tolerancePercentage: 0.01,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			utilization := engine.calculateUtilization(tc.cluster)
			
			diff := abs(utilization - tc.expectedUtilization)
			if diff > tc.tolerancePercentage {
				t.Errorf("Expected utilization %.3f, got %.3f (difference %.3f exceeds tolerance %.3f)",
					tc.expectedUtilization, utilization, diff, tc.tolerancePercentage)
			}
		})
	}
}

func TestResourceAwareEngine_ResourceAvailabilityFiltering(t *testing.T) {
	engine := NewResourceAwareEngine()

	tests := map[string]struct {
		clusters         []*tmcv1alpha1.ClusterRegistration
		expectedClusters []string
	}{
		"mixed availability": {
			clusters: []*tmcv1alpha1.ClusterRegistration{
				createClusterWithResources("available", 1000, 1000, 10, 300, 300, 3),    // 30% utilization - available
				createClusterWithResources("overloaded", 1000, 1000, 10, 950, 950, 10),  // 95% utilization - filtered out
				createClusterWithResources("borderline", 1000, 1000, 10, 890, 890, 8),   // 89% utilization - available
				createClusterWithResources("cpu-full", 1000, 1000, 10, 910, 100, 2),     // 91% CPU - filtered out
				createClusterWithResources("memory-full", 1000, 1000, 10, 100, 910, 2),  // 91% memory - filtered out
				createClusterWithResources("pods-full", 1000, 1000, 10, 100, 100, 10),   // 100% pods - filtered out
			},
			expectedClusters: []string{"available", "borderline"}, // Only clusters under 90% threshold
		},
		"no capacity info clusters": {
			clusters: []*tmcv1alpha1.ClusterRegistration{
				{ObjectMeta: metav1.ObjectMeta{Name: "no-info-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "no-info-2"}},
			},
			expectedClusters: []string{"no-info-1", "no-info-2"}, // All included when no capacity info
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			available := engine.filterByResourceAvailability(tc.clusters)
			
			if len(available) != len(tc.expectedClusters) {
				t.Errorf("Expected %d available clusters, got %d", len(tc.expectedClusters), len(available))
				return
			}

			// Check that expected clusters are present
			availableNames := make(map[string]bool)
			for _, cluster := range available {
				availableNames[cluster.Name] = true
			}

			for _, expectedName := range tc.expectedClusters {
				if !availableNames[expectedName] {
					t.Errorf("Expected cluster %s to be available but was filtered out", expectedName)
				}
			}
		})
	}
}

// Helper functions

func createClusterWithResources(name string, cpuCap, memCap int64, podCap int32, cpuUsed, memUsed int64, podsUsed int32) *tmcv1alpha1.ClusterRegistration {
	return &tmcv1alpha1.ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: tmcv1alpha1.ClusterRegistrationSpec{
			Location: "us-west-2",
			Capacity: tmcv1alpha1.ClusterCapacity{
				CPU:     &cpuCap,
				Memory:  &memCap,
				MaxPods: &podCap,
			},
		},
		Status: tmcv1alpha1.ClusterRegistrationStatus{
			AllocatedResources: &tmcv1alpha1.ClusterResourceUsage{
				CPU:    &cpuUsed,
				Memory: &memUsed,
				Pods:   &podsUsed,
			},
		},
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}