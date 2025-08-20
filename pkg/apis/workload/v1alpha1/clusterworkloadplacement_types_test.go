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

package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestClusterWorkloadPlacement_EvaluatePlacement(t *testing.T) {
	tests := map[string]struct {
		placement     *ClusterWorkloadPlacement
		target        *SyncTarget
		expectMatch   bool
		expectReason  string
	}{
		"nil target should reject": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{},
			},
			target:       nil,
			expectMatch:  false,
			expectReason: "target is nil",
		},
		"empty placement should accept any target": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{},
			},
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-target",
				},
				Spec: SyncTargetSpec{
					Location: "us-west-1",
				},
			},
			expectMatch:  true,
			expectReason: "target meets all placement criteria",
		},
		"namespace selector matching": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"env": "production",
						},
					},
				},
			},
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "production-target",
					Labels: map[string]string{
						"env": "production",
					},
				},
				Spec: SyncTargetSpec{
					Location: "us-west-2",
				},
			},
			expectMatch:  true,
			expectReason: "target meets all placement criteria",
		},
		"namespace selector not matching": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"env": "production",
						},
					},
				},
			},
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dev-target",
					Labels: map[string]string{
						"env": "development",
					},
				},
				Spec: SyncTargetSpec{
					Location: "us-east-1",
				},
			},
			expectMatch:  false,
			expectReason: "namespace selector does not match target labels",
		},
		"location selector matching required": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					LocationSelector: &LocationSelector{
						RequiredLocations: []string{"us-west-1", "us-west-2"},
					},
				},
			},
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "west-coast-target",
				},
				Spec: SyncTargetSpec{
					Location: "us-west-1",
				},
			},
			expectMatch:  true,
			expectReason: "target meets all placement criteria",
		},
		"location selector not matching required": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					LocationSelector: &LocationSelector{
						RequiredLocations: []string{"us-west-1", "us-west-2"},
					},
				},
			},
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "east-coast-target",
				},
				Spec: SyncTargetSpec{
					Location: "us-east-1",
				},
			},
			expectMatch:  false,
			expectReason: "location requirements not met",
		},
		"resource requirements basic check": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					ResourceRequirements: &ResourceRequirements{
						MinCPU:    "100m",
						MinMemory: "128Mi",
					},
				},
			},
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capable-target",
				},
				Status: SyncTargetStatus{
					Allocatable: ResourceCapacity{
						CPU:    resource.NewQuantity(1, resource.DecimalSI),
						Memory: resource.NewQuantity(1024*1024*1024, resource.BinarySI), // 1Gi
					},
				},
			},
			expectMatch:  true,
			expectReason: "target meets all placement criteria",
		},
		"multiple constraints all matching": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"tier": "frontend",
						},
					},
					LocationSelector: &LocationSelector{
						RequiredLocations: []string{"us-central-1"},
					},
					ResourceRequirements: &ResourceRequirements{
						MinCPU: "200m",
					},
				},
			},
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "frontend-target",
					Labels: map[string]string{
						"tier": "frontend",
					},
				},
				Spec: SyncTargetSpec{
					Location: "us-central-1",
				},
				Status: SyncTargetStatus{
					Allocatable: ResourceCapacity{
						CPU: resource.NewMilliQuantity(500, resource.DecimalSI),
					},
				},
			},
			expectMatch:  true,
			expectReason: "target meets all placement criteria",
		},
		"invalid namespace selector": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					NamespaceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "invalid",
								Operator: metav1.LabelSelectorOperator("InvalidOperator"),
								Values:   []string{"value"},
							},
						},
					},
				},
			},
			target: &SyncTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "any-target",
				},
			},
			expectMatch:  false,
			expectReason: "invalid namespace selector: \"InvalidOperator\" is not a valid label selector operator",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			match, reason := tc.placement.EvaluatePlacement(tc.target)
			
			require.Equal(t, tc.expectMatch, match, "placement match result should match expected")
			require.Contains(t, reason, tc.expectReason, "placement reason should contain expected text")
		})
	}
}

func TestClusterWorkloadPlacement_evaluateLocationSelector(t *testing.T) {
	tests := map[string]struct {
		placement    *ClusterWorkloadPlacement
		target       *SyncTarget
		expectResult bool
	}{
		"nil location selector always passes": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					LocationSelector: nil,
				},
			},
			target: &SyncTarget{
				Spec: SyncTargetSpec{Location: "any-location"},
			},
			expectResult: true,
		},
		"empty required locations always passes": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					LocationSelector: &LocationSelector{
						RequiredLocations: []string{},
					},
				},
			},
			target: &SyncTarget{
				Spec: SyncTargetSpec{Location: "any-location"},
			},
			expectResult: true,
		},
		"target with no location fails required check": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					LocationSelector: &LocationSelector{
						RequiredLocations: []string{"required-location"},
					},
				},
			},
			target: &SyncTarget{
				Spec: SyncTargetSpec{Location: ""}, // No location specified
			},
			expectResult: false,
		},
		"exact location match passes": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					LocationSelector: &LocationSelector{
						RequiredLocations: []string{"us-west-1"},
					},
				},
			},
			target: &SyncTarget{
				Spec: SyncTargetSpec{Location: "us-west-1"},
			},
			expectResult: true,
		},
		"one of multiple required locations matches": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					LocationSelector: &LocationSelector{
						RequiredLocations: []string{"us-west-1", "us-west-2", "us-central-1"},
					},
				},
			},
			target: &SyncTarget{
				Spec: SyncTargetSpec{Location: "us-central-1"},
			},
			expectResult: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.placement.evaluateLocationSelector(tc.target)
			require.Equal(t, tc.expectResult, result, "location selector evaluation should match expected")
		})
	}
}

func TestClusterWorkloadPlacement_evaluateResourceRequirements(t *testing.T) {
	tests := map[string]struct {
		placement    *ClusterWorkloadPlacement
		target       *SyncTarget
		expectResult bool
	}{
		"nil resource requirements always passes": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					ResourceRequirements: nil,
				},
			},
			target: &SyncTarget{},
			expectResult: true,
		},
		"target with CPU capacity passes": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					ResourceRequirements: &ResourceRequirements{
						MinCPU: "100m",
					},
				},
			},
			target: &SyncTarget{
				Status: SyncTargetStatus{
					Allocatable: ResourceCapacity{
						CPU: resource.NewMilliQuantity(500, resource.DecimalSI),
					},
				},
			},
			expectResult: true,
		},
		"target with memory capacity passes": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					ResourceRequirements: &ResourceRequirements{
						MinMemory: "256Mi",
					},
				},
			},
			target: &SyncTarget{
				Status: SyncTargetStatus{
					Allocatable: ResourceCapacity{
						Memory: resource.NewQuantity(1024*1024*1024, resource.BinarySI), // 1Gi
					},
				},
			},
			expectResult: true,
		},
		"target with no capacity information still passes": {
			placement: &ClusterWorkloadPlacement{
				Spec: ClusterWorkloadPlacementSpec{
					ResourceRequirements: &ResourceRequirements{
						MinCPU: "100m",
					},
				},
			},
			target: &SyncTarget{
				Status: SyncTargetStatus{
					Allocatable: ResourceCapacity{}, // No capacity info
				},
			},
			expectResult: true, // Current implementation is permissive
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.placement.evaluateResourceRequirements(tc.target)
			require.Equal(t, tc.expectResult, result, "resource requirements evaluation should match expected")
		})
	}
}

func TestClusterWorkloadPlacement_GetSetConditions(t *testing.T) {
	placement := &ClusterWorkloadPlacement{}
	
	// Initially no conditions
	conditions := placement.GetConditions()
	require.Empty(t, conditions)
	
	// Set some conditions
	testConditions := conditionsv1alpha1.Conditions{
		{
			Type:   PlacementReady,
			Status: corev1.ConditionTrue,
			Reason: PlacementSuccessReason,
		},
	}
	placement.SetConditions(testConditions)
	
	// Verify conditions were set
	retrievedConditions := placement.GetConditions()
	require.Len(t, retrievedConditions, 1)
	require.Equal(t, PlacementReady, retrievedConditions[0].Type)
	require.Equal(t, corev1.ConditionTrue, retrievedConditions[0].Status)
}

func TestClusterWorkloadPlacement_ValidationStructure(t *testing.T) {
	// Test that we can create a valid placement with all fields
	placement := &ClusterWorkloadPlacement{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-placement",
		},
		Spec: ClusterWorkloadPlacementSpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"env": "production",
				},
			},
			LocationSelector: &LocationSelector{
				RequiredLocations:  []string{"us-west-1"},
				PreferredLocations: []string{"us-west-2", "us-central-1"},
			},
			ResourceRequirements: &ResourceRequirements{
				MinCPU:    "500m",
				MinMemory: "1Gi",
			},
			MaxReplicas: &[]int32{5}[0],
			MinReplicas: &[]int32{2}[0],
		},
		Status: ClusterWorkloadPlacementStatus{
			SelectedTargets: 3,
			TargetSelections: []TargetSelection{
				{
					TargetName: "target-1",
					Workspace:  "root:production",
					Selected:   true,
					Reason:     "meets all criteria",
					Score:      100,
				},
			},
		},
	}
	
	require.NotNil(t, placement)
	require.Equal(t, "test-placement", placement.Name)
	require.NotNil(t, placement.Spec.NamespaceSelector)
	require.NotNil(t, placement.Spec.LocationSelector)
	require.NotNil(t, placement.Spec.ResourceRequirements)
	require.Equal(t, int32(5), *placement.Spec.MaxReplicas)
	require.Equal(t, int32(2), *placement.Spec.MinReplicas)
	require.Equal(t, int32(3), placement.Status.SelectedTargets)
	require.Len(t, placement.Status.TargetSelections, 1)
}