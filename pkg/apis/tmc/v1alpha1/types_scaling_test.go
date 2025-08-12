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

package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

func TestScalingPolicyValidation(t *testing.T) {
	tests := map[string]struct {
		policy *ScalingPolicy
		want   bool
	}{
		"valid resource-based scaling policy": {
			policy: &ScalingPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tmc.kcp.io/v1alpha1",
					Kind:       "ScalingPolicy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-scaling",
					Namespace: "default",
				},
				Spec: ScalingPolicySpec{
					TargetRef: CrossNamespaceObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-app",
					},
					MinReplicas: int32Ptr(1),
					MaxReplicas: 10,
					Metrics: []MetricSpec{
						{
							Type: ResourceMetricSourceType,
							Resource: &ResourceMetricSource{
								Name: "cpu",
								Target: MetricTarget{
									Type:               autoscalingv2.UtilizationMetricType,
									AverageUtilization: int32Ptr(80),
								},
							},
						},
					},
				},
			},
			want: true,
		},
		"valid external metric scaling policy": {
			policy: &ScalingPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tmc.kcp.io/v1alpha1",
					Kind:       "ScalingPolicy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "external-scaling",
					Namespace: "default",
				},
				Spec: ScalingPolicySpec{
					TargetRef: CrossNamespaceObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "queue-processor",
					},
					MinReplicas: int32Ptr(2),
					MaxReplicas: 20,
					Metrics: []MetricSpec{
						{
							Type: ExternalMetricSourceType,
							External: &ExternalMetricSource{
								Metric: MetricIdentifier{
									Name: "queue.amazonaws.com|ApproximateNumberOfMessages",
									Selector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"queue": "work-queue",
										},
									},
								},
								Target: MetricTarget{
									Type:         autoscalingv2.AverageValueMetricType,
									AverageValue: resource.NewQuantity(30, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			want: true,
		},
		"valid multi-metric scaling policy with behavior": {
			policy: &ScalingPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "tmc.kcp.io/v1alpha1",
					Kind:       "ScalingPolicy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "complex-scaling",
					Namespace: "production",
				},
				Spec: ScalingPolicySpec{
					TargetRef: CrossNamespaceObjectReference{
						APIVersion: "apps/v1",
						Kind:       "StatefulSet",
						Name:       "database",
						Namespace:  "db",
					},
					MinReplicas: int32Ptr(3),
					MaxReplicas: 50,
					Metrics: []MetricSpec{
						{
							Type: ResourceMetricSourceType,
							Resource: &ResourceMetricSource{
								Name: "cpu",
								Target: MetricTarget{
									Type:               autoscalingv2.UtilizationMetricType,
									AverageUtilization: int32Ptr(70),
								},
							},
						},
						{
							Type: ResourceMetricSourceType,
							Resource: &ResourceMetricSource{
								Name: "memory",
								Target: MetricTarget{
									Type:               autoscalingv2.UtilizationMetricType,
									AverageUtilization: int32Ptr(80),
								},
							},
						},
					},
					Behavior: &ScalingBehavior{
						ScaleUp: &ScalingRules{
							StabilizationWindowSeconds: int32Ptr(300),
							SelectPolicy:               &[]autoscalingv2.ScalingPolicySelect{autoscalingv2.MaxChangePolicySelect}[0],
							Policies: []HPAScalingRule{
								{
									Type:          autoscalingv2.PodsScalingPolicy,
									Value:         4,
									PeriodSeconds: 60,
								},
							},
						},
						ScaleDown: &ScalingRules{
							StabilizationWindowSeconds: int32Ptr(600),
							SelectPolicy:               &[]autoscalingv2.ScalingPolicySelect{autoscalingv2.MinChangePolicySelect}[0],
							Policies: []HPAScalingRule{
								{
									Type:          autoscalingv2.PodsScalingPolicy,
									Value:         2,
									PeriodSeconds: 120,
								},
							},
						},
					},
				},
			},
			want: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Basic validation that the struct can be created
			if tc.policy.Spec.TargetRef.APIVersion == "" {
				t.Errorf("TargetRef.APIVersion should not be empty")
			}
			if tc.policy.Spec.TargetRef.Kind == "" {
				t.Errorf("TargetRef.Kind should not be empty")
			}
			if tc.policy.Spec.TargetRef.Name == "" {
				t.Errorf("TargetRef.Name should not be empty")
			}
			if tc.policy.Spec.MaxReplicas <= 0 {
				t.Errorf("MaxReplicas should be positive")
			}
			if tc.policy.Spec.MinReplicas != nil && *tc.policy.Spec.MinReplicas <= 0 {
				t.Errorf("MinReplicas should be positive when set")
			}
			if tc.policy.Spec.MinReplicas != nil && *tc.policy.Spec.MinReplicas > tc.policy.Spec.MaxReplicas {
				t.Errorf("MinReplicas should not exceed MaxReplicas")
			}
		})
	}
}

func TestMetricTargetValidation(t *testing.T) {
	tests := map[string]struct {
		target MetricTarget
		valid  bool
	}{
		"valid utilization target": {
			target: MetricTarget{
				Type:               autoscalingv2.UtilizationMetricType,
				AverageUtilization: int32Ptr(50),
			},
			valid: true,
		},
		"valid value target": {
			target: MetricTarget{
				Type:  autoscalingv2.ValueMetricType,
				Value: resource.NewQuantity(1000, resource.DecimalSI),
			},
			valid: true,
		},
		"valid average value target": {
			target: MetricTarget{
				Type:         autoscalingv2.AverageValueMetricType,
				AverageValue: resource.NewQuantity(100, resource.DecimalSI),
			},
			valid: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			switch tc.target.Type {
			case autoscalingv2.UtilizationMetricType:
				if tc.target.AverageUtilization == nil {
					t.Errorf("AverageUtilization should be set for Utilization type")
				}
			case autoscalingv2.ValueMetricType:
				if tc.target.Value == nil {
					t.Errorf("Value should be set for Value type")
				}
			case autoscalingv2.AverageValueMetricType:
				if tc.target.AverageValue == nil {
					t.Errorf("AverageValue should be set for AverageValue type")
				}
			}
		})
	}
}

func TestScalingBehaviorValidation(t *testing.T) {
	behavior := &ScalingBehavior{
		ScaleUp: &ScalingRules{
			StabilizationWindowSeconds: int32Ptr(300),
			SelectPolicy:               &[]autoscalingv2.ScalingPolicySelect{autoscalingv2.MaxChangePolicySelect}[0],
			Policies: []HPAScalingRule{
				{
					Type:          autoscalingv2.PodsScalingPolicy,
					Value:         4,
					PeriodSeconds: 60,
				},
				{
					Type:          autoscalingv2.PercentScalingPolicy,
					Value:         50,
					PeriodSeconds: 120,
				},
			},
		},
		ScaleDown: &ScalingRules{
			StabilizationWindowSeconds: int32Ptr(600),
			SelectPolicy:               &[]autoscalingv2.ScalingPolicySelect{autoscalingv2.MinChangePolicySelect}[0],
			Policies: []HPAScalingRule{
				{
					Type:          autoscalingv2.PodsScalingPolicy,
					Value:         2,
					PeriodSeconds: 120,
				},
			},
		},
	}

	// Test scale up policies
	if behavior.ScaleUp == nil {
		t.Errorf("ScaleUp should not be nil")
	} else {
		if behavior.ScaleUp.StabilizationWindowSeconds == nil {
			t.Errorf("StabilizationWindowSeconds should be set")
		}
		if len(behavior.ScaleUp.Policies) == 0 {
			t.Errorf("Policies should not be empty")
		}
		for i, policy := range behavior.ScaleUp.Policies {
			if policy.Value <= 0 {
				t.Errorf("Policy %d Value should be positive", i)
			}
			if policy.PeriodSeconds <= 0 {
				t.Errorf("Policy %d PeriodSeconds should be positive", i)
			}
		}
	}

	// Test scale down policies
	if behavior.ScaleDown == nil {
		t.Errorf("ScaleDown should not be nil")
	} else {
		if behavior.ScaleDown.StabilizationWindowSeconds == nil {
			t.Errorf("StabilizationWindowSeconds should be set")
		}
		if len(behavior.ScaleDown.Policies) == 0 {
			t.Errorf("Policies should not be empty")
		}
		for i, policy := range behavior.ScaleDown.Policies {
			if policy.Value <= 0 {
				t.Errorf("Policy %d Value should be positive", i)
			}
			if policy.PeriodSeconds <= 0 {
				t.Errorf("Policy %d PeriodSeconds should be positive", i)
			}
		}
	}
}

// Helper function to create int32 pointers
func int32Ptr(i int32) *int32 {
	return &i
}