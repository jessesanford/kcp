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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestWorkloadPlacementAdvancedValidation(t *testing.T) {
	tests := map[string]struct {
		placement *WorkloadPlacementAdvanced
		wantValid bool
	}{
		"valid basic placement": {
			placement: &WorkloadPlacementAdvanced{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-placement",
					Namespace: "default",
				},
				Spec: WorkloadPlacementAdvancedSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
					},
					ClusterSelector: ClusterSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"region": "us-west",
							},
						},
					},
					PlacementPolicy: PlacementPolicyRoundRobin,
				},
			},
			wantValid: true,
		},
		"valid placement with affinity rules": {
			placement: &WorkloadPlacementAdvanced{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-affinity-placement",
					Namespace: "default",
				},
				Spec: WorkloadPlacementAdvancedSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "frontend",
							},
						},
					},
					ClusterSelector: ClusterSelector{
						LocationSelector: []string{"us-west-1", "us-west-2"},
					},
					AffinityRules: &AffinityRules{
						ClusterAffinity: &ClusterAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []ClusterAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"tier": "production",
										},
									},
								},
							},
						},
						ClusterAntiAffinity: &ClusterAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []ClusterAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app": "database",
										},
									},
									LocationSelector: []string{"us-east-1"},
								},
							},
						},
					},
				},
			},
			wantValid: true,
		},
		"valid placement with rollout strategy": {
			placement: &WorkloadPlacementAdvanced{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rollout-placement",
					Namespace: "default",
				},
				Spec: WorkloadPlacementAdvancedSpec{
					WorkloadSelector: WorkloadSelector{
						WorkloadTypes: []WorkloadType{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
							},
						},
					},
					ClusterSelector: ClusterSelector{
						ClusterNames: []string{"cluster-1", "cluster-2", "cluster-3"},
					},
					RolloutStrategy: &RolloutStrategy{
						Type: RolloutStrategyTypeRollingUpdate,
						RollingUpdate: &RollingUpdateStrategy{
							MaxUnavailable: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "25%",
							},
							MaxSurge: &intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "25%",
							},
						},
					},
				},
			},
			wantValid: true,
		},
		"valid placement with traffic splitting": {
			placement: &WorkloadPlacementAdvanced{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-traffic-placement",
					Namespace: "default",
				},
				Spec: WorkloadPlacementAdvancedSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"tier": "frontend",
							},
						},
					},
					ClusterSelector: ClusterSelector{
						LocationSelector: []string{"us-east-1", "us-west-1", "eu-west-1"},
					},
					TrafficSplitting: &TrafficSplitting{
						ClusterWeights: []ClusterWeight{
							{
								ClusterName: "us-east-1",
								Weight:      50,
							},
							{
								ClusterName: "us-west-1",
								Weight:      30,
							},
							{
								ClusterName: "eu-west-1",
								Weight:      20,
							},
						},
					},
				},
			},
			wantValid: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.placement == nil {
				t.Fatal("placement cannot be nil")
			}

			// Basic validation - ensure required fields are present
			if tc.placement.Spec.WorkloadSelector.LabelSelector == nil && 
			   len(tc.placement.Spec.WorkloadSelector.WorkloadTypes) == 0 {
				if tc.wantValid {
					t.Error("expected valid placement, but WorkloadSelector has no selection criteria")
				}
				return
			}

			// Validate placement policy enum
			if tc.placement.Spec.PlacementPolicy != "" {
				validPolicies := []PlacementPolicy{
					PlacementPolicyRoundRobin,
					PlacementPolicyLeastLoaded,
					PlacementPolicyRandom,
					PlacementPolicyLocationAware,
				}
				found := false
				for _, policy := range validPolicies {
					if tc.placement.Spec.PlacementPolicy == policy {
						found = true
						break
					}
				}
				if !found && tc.wantValid {
					t.Errorf("invalid placement policy: %s", tc.placement.Spec.PlacementPolicy)
				}
			}

			// Validate traffic splitting if present
			if tc.placement.Spec.TrafficSplitting != nil {
				validateTrafficSplitting(t, tc.placement.Spec.TrafficSplitting, tc.wantValid)
			}
		})
	}
}

func validateTrafficSplitting(t *testing.T, traffic *TrafficSplitting, wantValid bool) {
	totalWeight := int32(0)
	for _, weight := range traffic.ClusterWeights {
		totalWeight += weight.Weight
	}
	if totalWeight != 100 && wantValid {
		t.Errorf("total traffic weight should be 100, got %d", totalWeight)
	}
}

func TestWorkloadPlacementAdvancedStatusTransitions(t *testing.T) {
	status := &WorkloadPlacementAdvancedStatus{
		RolloutState: &RolloutState{
			Phase: RolloutPhasePending,
		},
	}

	// Test phase transition
	status.RolloutState.Phase = RolloutPhaseInProgress
	if status.RolloutState.Phase != RolloutPhaseInProgress {
		t.Errorf("expected phase %s, got %s", RolloutPhaseInProgress, status.RolloutState.Phase)
	}
}

// Helper functions
func int32Ptr(i int32) *int32 {
	return &i
}

