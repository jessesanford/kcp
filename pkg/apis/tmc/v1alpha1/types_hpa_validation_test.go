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
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestDefaultHorizontalPodAutoscalerPolicy(t *testing.T) {
	tests := map[string]struct {
		policy   *HorizontalPodAutoscalerPolicy
		validate func(*testing.T, *HorizontalPodAutoscalerPolicy)
	}{
		"sets default strategy": {
			policy: &HorizontalPodAutoscalerPolicy{
				Spec: HorizontalPodAutoscalerPolicySpec{
					MaxReplicas: 10,
					TargetRef: CrossClusterObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-app",
					},
				},
			},
			validate: func(t *testing.T, policy *HorizontalPodAutoscalerPolicy) {
				if policy.Spec.Strategy != DistributedAutoScaling {
					t.Errorf("expected strategy %q, got %q", DistributedAutoScaling, policy.Spec.Strategy)
				}
			},
		},
		"sets default min replicas": {
			policy: &HorizontalPodAutoscalerPolicy{
				Spec: HorizontalPodAutoscalerPolicySpec{
					MaxReplicas: 10,
					TargetRef: CrossClusterObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-app",
					},
				},
			},
			validate: func(t *testing.T, policy *HorizontalPodAutoscalerPolicy) {
				if policy.Spec.MinReplicas == nil || *policy.Spec.MinReplicas != 1 {
					t.Errorf("expected minReplicas to be 1, got %v", policy.Spec.MinReplicas)
				}
			},
		},
		"sets default scaling policies": {
			policy: &HorizontalPodAutoscalerPolicy{
				Spec: HorizontalPodAutoscalerPolicySpec{
					MaxReplicas: 10,
					TargetRef: CrossClusterObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-app",
					},
				},
			},
			validate: func(t *testing.T, policy *HorizontalPodAutoscalerPolicy) {
				if policy.Spec.ScaleDownPolicy == nil || *policy.Spec.ScaleDownPolicy != BalancedScaleDown {
					t.Errorf("expected scaleDownPolicy %q, got %v", BalancedScaleDown, policy.Spec.ScaleDownPolicy)
				}
				if policy.Spec.ScaleUpPolicy == nil || *policy.Spec.ScaleUpPolicy != LoadAwareScaleUp {
					t.Errorf("expected scaleUpPolicy %q, got %v", LoadAwareScaleUp, policy.Spec.ScaleUpPolicy)
				}
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			DefaultHorizontalPodAutoscalerPolicy(tc.policy)
			tc.validate(t, tc.policy)
		})
	}
}

func TestValidateHorizontalPodAutoscalerPolicy(t *testing.T) {
	validPolicy := &HorizontalPodAutoscalerPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: HorizontalPodAutoscalerPolicySpec{
			Strategy:    DistributedAutoScaling,
			MinReplicas: int32Ptr(1),
			MaxReplicas: 10,
			TargetRef: CrossClusterObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "test-app",
			},
			Metrics: []MetricSpec{
				{
					Type: ResourceMetricSourceType,
					Resource: &ResourceMetricSource{
						Name: "cpu",
						Target: MetricTarget{
							Type:               UtilizationMetricType,
							AverageUtilization: int32Ptr(70),
						},
					},
				},
			},
		},
	}

	tests := map[string]struct {
		policy      *HorizontalPodAutoscalerPolicy
		expectError bool
	}{
		"valid policy": {
			policy:      validPolicy,
			expectError: false,
		},
		"invalid strategy": {
			policy: func() *HorizontalPodAutoscalerPolicy {
				p := validPolicy.DeepCopy()
				p.Spec.Strategy = "InvalidStrategy"
				return p
			}(),
			expectError: true,
		},
		"invalid min replicas": {
			policy: func() *HorizontalPodAutoscalerPolicy {
				p := validPolicy.DeepCopy()
				p.Spec.MinReplicas = int32Ptr(-1)
				return p
			}(),
			expectError: true,
		},
		"min replicas greater than max": {
			policy: func() *HorizontalPodAutoscalerPolicy {
				p := validPolicy.DeepCopy()
				p.Spec.MinReplicas = int32Ptr(15)
				return p
			}(),
			expectError: true,
		},
		"missing target ref kind": {
			policy: func() *HorizontalPodAutoscalerPolicy {
				p := validPolicy.DeepCopy()
				p.Spec.TargetRef.Kind = ""
				return p
			}(),
			expectError: true,
		},
		"no metrics": {
			policy: func() *HorizontalPodAutoscalerPolicy {
				p := validPolicy.DeepCopy()
				p.Spec.Metrics = []MetricSpec{}
				return p
			}(),
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			errs := ValidateHorizontalPodAutoscalerPolicy(tc.policy)
			hasError := len(errs) > 0

			if hasError != tc.expectError {
				t.Errorf("expected error: %v, got errors: %v", tc.expectError, errs)
			}
		})
	}
}

func TestValidateMetricSpec(t *testing.T) {
	tests := map[string]struct {
		metric      MetricSpec
		expectError bool
	}{
		"valid resource metric": {
			metric: MetricSpec{
				Type: ResourceMetricSourceType,
				Resource: &ResourceMetricSource{
					Name: "cpu",
					Target: MetricTarget{
						Type:               UtilizationMetricType,
						AverageUtilization: int32Ptr(80),
					},
				},
			},
			expectError: false,
		},
		"valid pods metric": {
			metric: MetricSpec{
				Type: PodsMetricSourceType,
				Pods: &PodsMetricSource{
					Metric: MetricIdentifier{
						Name: "packets-per-second",
					},
					Target: MetricTarget{
						Type:         AverageValueMetricType,
						AverageValue: resource.NewQuantity(1000, resource.DecimalSI),
					},
				},
			},
			expectError: false,
		},
		"missing resource for resource metric": {
			metric: MetricSpec{
				Type:     ResourceMetricSourceType,
				Resource: nil,
			},
			expectError: true,
		},
		"invalid metric target type": {
			metric: MetricSpec{
				Type: ResourceMetricSourceType,
				Resource: &ResourceMetricSource{
					Name: "cpu",
					Target: MetricTarget{
						Type: "InvalidType",
					},
				},
			},
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			errs := validateMetricSpec(tc.metric, nil)
			hasError := len(errs) > 0

			if hasError != tc.expectError {
				t.Errorf("expected error: %v, got errors: %v", tc.expectError, errs)
			}
		})
	}
}

func TestConditionHelpers(t *testing.T) {
	conditions := []metav1.Condition{
		{
			Type:               HorizontalPodAutoscalerPolicyReady,
			Status:             metav1.ConditionTrue,
			Reason:             ReasonPolicyReady,
			Message:            "Policy is ready",
			ObservedGeneration: 1,
		},
	}

	t.Run("GetCondition", func(t *testing.T) {
		condition := GetCondition(conditions, HorizontalPodAutoscalerPolicyReady)
		if condition == nil {
			t.Fatal("expected condition to be found")
		}
		if condition.Status != metav1.ConditionTrue {
			t.Errorf("expected status True, got %v", condition.Status)
		}
	})

	t.Run("SetCondition new", func(t *testing.T) {
		testConditions := append([]metav1.Condition{}, conditions...)
		
		newCondition := metav1.Condition{
			Type:               HorizontalPodAutoscalerPolicyActive,
			Status:             metav1.ConditionTrue,
			Reason:             ReasonScalingActive,
			Message:            "Scaling is active",
			ObservedGeneration: 1,
		}
		
		SetCondition(&testConditions, newCondition)
		
		if len(testConditions) != 2 {
			t.Errorf("expected 2 conditions, got %d", len(testConditions))
		}
		
		activeCondition := GetCondition(testConditions, HorizontalPodAutoscalerPolicyActive)
		if activeCondition == nil {
			t.Fatal("expected active condition to be set")
		}
	})

	t.Run("RemoveCondition", func(t *testing.T) {
		testConditions := append([]metav1.Condition{}, conditions...)
		
		RemoveCondition(&testConditions, HorizontalPodAutoscalerPolicyReady)
		
		if len(testConditions) != 0 {
			t.Errorf("expected 0 conditions, got %d", len(testConditions))
		}
	})
}

func TestValidationHelpers(t *testing.T) {
	t.Run("ValidateScaleTarget", func(t *testing.T) {
		tests := map[string]struct {
			ref         CrossClusterObjectReference
			expectError bool
		}{
			"valid deployment": {
				ref: CrossClusterObjectReference{
					Kind: "Deployment",
				},
				expectError: false,
			},
			"invalid kind": {
				ref: CrossClusterObjectReference{
					Kind: "Service",
				},
				expectError: true,
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				err := ValidateScaleTarget(tc.ref)
				hasError := err != nil

				if hasError != tc.expectError {
					t.Errorf("expected error: %v, got: %v", tc.expectError, err)
				}
			})
		}
	})

	t.Run("NormalizeMetricName", func(t *testing.T) {
		tests := map[string]struct {
			input    string
			expected string
		}{
			"lowercase": {"cpu", "cpu"},
			"uppercase": {"CPU", "cpu"},
			"with underscore": {"memory_usage", "memory-usage"},
			"mixed": {"CPU_Usage", "cpu-usage"},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				result := NormalizeMetricName(tc.input)
				if result != tc.expected {
					t.Errorf("expected %q, got %q", tc.expected, result)
				}
			})
		}
	})

	t.Run("IsValidResourceName", func(t *testing.T) {
		tests := map[string]struct {
			name     string
			expected bool
		}{
			"cpu": {"cpu", true},
			"memory": {"memory", true},
			"ephemeral-storage": {"ephemeral-storage", true},
			"invalid": {"network", false},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				result := IsValidResourceName(tc.name)
				if result != tc.expected {
					t.Errorf("expected %v for %q, got %v", tc.expected, tc.name, result)
				}
			})
		}
	})
}

func int32Ptr(i int32) *int32 {
	return &i
}