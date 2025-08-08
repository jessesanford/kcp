/*
Copyright 2025 The KCP Authors.

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

package placement

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

func TestDecisionEngine_MakeAdvancedPlacementDecisions(t *testing.T) {
	tests := map[string]struct {
		placement       *workloadv1alpha1.Placement
		locations       []*workloadv1alpha1.Location
		config          DecisionConfig
		expectedCount   int
		wantError       bool
	}{
		"successful placement with score strategy": {
			placement: &workloadv1alpha1.Placement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
				Spec: workloadv1alpha1.PlacementSpec{
					WorkloadReference: workloadv1alpha1.WorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-app",
					},
					NumberOfClusters: func() *int32 { n := int32(2); return &n }(),
				},
			},
			locations: []*workloadv1alpha1.Location{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "location-us-west",
						Labels: map[string]string{
							"topology.kubernetes.io/region": "us-west",
							"topology.kubernetes.io/zone":   "us-west-1a",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "location-us-east",
						Labels: map[string]string{
							"topology.kubernetes.io/region": "us-east",
							"topology.kubernetes.io/zone":   "us-east-1a",
						},
					},
				},
			},
			config: DecisionConfig{
				Strategy:       SelectionStrategyScore,
				ScoringWeights: DefaultScoringWeights(),
			},
			expectedCount: 2,
			wantError:     false,
		},
		"placement with location selector": {
			placement: &workloadv1alpha1.Placement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement-selector"},
				Spec: workloadv1alpha1.PlacementSpec{
					WorkloadReference: workloadv1alpha1.WorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-app",
					},
					LocationSelector: &workloadv1alpha1.LocationSelector{
						MatchLabels: map[string]string{
							"topology.kubernetes.io/region": "us-west",
						},
					},
					NumberOfClusters: func() *int32 { n := int32(1); return &n }(),
				},
			},
			locations: []*workloadv1alpha1.Location{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "location-us-west",
						Labels: map[string]string{
							"topology.kubernetes.io/region": "us-west",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "location-us-east",
						Labels: map[string]string{
							"topology.kubernetes.io/region": "us-east",
						},
					},
				},
			},
			config: DecisionConfig{
				Strategy:       SelectionStrategyScore,
				ScoringWeights: DefaultScoringWeights(),
			},
			expectedCount: 1,
			wantError:     false,
		},
		"no locations available": {
			placement: &workloadv1alpha1.Placement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement-empty"},
				Spec: workloadv1alpha1.PlacementSpec{
					WorkloadReference: workloadv1alpha1.WorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-app",
					},
				},
			},
			locations: []*workloadv1alpha1.Location{},
			config: DecisionConfig{
				Strategy:       SelectionStrategyScore,
				ScoringWeights: DefaultScoringWeights(),
			},
			expectedCount: 0,
			wantError:     false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			logger := klog.NewKlogr().WithName("test")
			engine := NewDecisionEngine(logger)

			decisions, err := engine.MakeAdvancedPlacementDecisions(ctx, tc.placement, tc.locations, tc.config)

			if tc.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(decisions) != tc.expectedCount {
				t.Errorf("expected %d decisions, got %d", tc.expectedCount, len(decisions))
			}

			// Verify decision properties
			for _, decision := range decisions {
				if decision.ClusterName == "" {
					t.Errorf("decision missing cluster name")
				}
				if decision.Location == "" {
					t.Errorf("decision missing location")
				}
				if decision.Reason == "" {
					t.Errorf("decision missing reason")
				}
				if decision.Score == nil {
					t.Errorf("decision missing score")
				}
			}
		})
	}
}

func TestDecisionEngine_SimpleSelection(t *testing.T) {
	locations := []*workloadv1alpha1.Location{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "location-us-west-1",
				Labels: map[string]string{
					"topology.kubernetes.io/region": "us-west",
					"topology.kubernetes.io/zone":   "us-west-1a",
				},
				Annotations: map[string]string{
					"workload.kcp.io/capacity": "high",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "location-us-east-1",
				Labels: map[string]string{
					"topology.kubernetes.io/region": "us-east",
					"topology.kubernetes.io/zone":   "us-east-1a",
				},
				Annotations: map[string]string{
					"workload.kcp.io/capacity": "low",
				},
			},
		},
	}

	placement := &workloadv1alpha1.Placement{
		ObjectMeta: metav1.ObjectMeta{Name: "test-placement"},
		Spec: workloadv1alpha1.PlacementSpec{
			WorkloadReference: workloadv1alpha1.WorkloadReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "test-app",
			},
			NumberOfClusters: func() *int32 { n := int32(1); return &n }(),
		},
	}

	ctx := context.Background()
	logger := klog.NewKlogr().WithName("test")
	engine := NewDecisionEngine(logger)

	config := DecisionConfig{
		Strategy:       SelectionStrategyScore,
		ScoringWeights: DefaultScoringWeights(),
	}

	decisions, err := engine.MakeAdvancedPlacementDecisions(ctx, placement, locations, config)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if len(decisions) != 1 {
		t.Errorf("expected 1 decision, got %d", len(decisions))
		return
	}

	// Should select the high-capacity location (higher score)
	if decisions[0].Location != "location-us-west-1" {
		t.Errorf("expected location-us-west-1 (high capacity) to be selected, got %s", decisions[0].Location)
	}
}

func TestDefaultScoringWeights(t *testing.T) {
	weights := DefaultScoringWeights()
	
	// Verify weights are reasonable
	if weights.LocationAffinity < 0 || weights.LocationAffinity > 100 {
		t.Errorf("LocationAffinity weight %d is out of range", weights.LocationAffinity)
	}
	if weights.ResourceCapacity < 0 || weights.ResourceCapacity > 100 {
		t.Errorf("ResourceCapacity weight %d is out of range", weights.ResourceCapacity)
	}
	if weights.WorkloadSpread < 0 || weights.WorkloadSpread > 100 {
		t.Errorf("WorkloadSpread weight %d is out of range", weights.WorkloadSpread)
	}
	if weights.NetworkLatency < 0 || weights.NetworkLatency > 100 {
		t.Errorf("NetworkLatency weight %d is out of range", weights.NetworkLatency)
	}
	
	// Verify total weights are reasonable
	total := weights.LocationAffinity + weights.ResourceCapacity + weights.WorkloadSpread + weights.NetworkLatency
	if total == 0 {
		t.Errorf("total weights should not be zero")
	}
}

func TestDecisionEngine_LabelSelectorRequirements(t *testing.T) {
	logger := klog.NewKlogr().WithName("test")
	engine := NewDecisionEngine(logger)

	locationLabels := map[string]string{
		"environment": "production",
		"tier":        "frontend",
		"region":      "us-west",
	}

	tests := map[string]struct {
		requirement metav1.LabelSelectorRequirement
		expected    bool
	}{
		"In operator - match": {
			requirement: metav1.LabelSelectorRequirement{
				Key:      "environment",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"production", "staging"},
			},
			expected: true,
		},
		"NotIn operator - match": {
			requirement: metav1.LabelSelectorRequirement{
				Key:      "environment",
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{"development", "staging"},
			},
			expected: true,
		},
		"Exists operator - match": {
			requirement: metav1.LabelSelectorRequirement{
				Key:      "environment",
				Operator: metav1.LabelSelectorOpExists,
			},
			expected: true,
		},
		"DoesNotExist operator - match": {
			requirement: metav1.LabelSelectorRequirement{
				Key:      "nonexistent",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			},
			expected: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := engine.evaluateLabelSelectorRequirement(locationLabels, tc.requirement)
			if result != tc.expected {
				t.Errorf("expected %v, got %v for requirement %+v", tc.expected, result, tc.requirement)
			}
		})
	}
}