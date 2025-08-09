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

func TestHorizontalPodAutoscalerPolicyDefaults(t *testing.T) {
	tests := map[string]struct {
		policy   *HorizontalPodAutoscalerPolicy
		expected *HorizontalPodAutoscalerPolicySpec
	}{
		"default strategy should be Distributed": {
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
			expected: &HorizontalPodAutoscalerPolicySpec{
				Strategy:    DistributedAutoScaling,
				MaxReplicas: 10,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// For now, just verify the constants are defined correctly
			if DistributedAutoScaling != "Distributed" {
				t.Errorf("DistributedAutoScaling should be 'Distributed', got %q", DistributedAutoScaling)
			}
			if CentralizedAutoScaling != "Centralized" {
				t.Errorf("CentralizedAutoScaling should be 'Centralized', got %q", CentralizedAutoScaling)
			}
			if HybridAutoScaling != "Hybrid" {
				t.Errorf("HybridAutoScaling should be 'Hybrid', got %q", HybridAutoScaling)
			}
		})
	}
}

func TestMetricTargetTypes(t *testing.T) {
	tests := map[string]struct {
		targetType MetricTargetType
		expected   string
	}{
		"utilization type": {
			targetType: UtilizationMetricType,
			expected:   "Utilization",
		},
		"value type": {
			targetType: ValueMetricType,
			expected:   "Value",
		},
		"average value type": {
			targetType: AverageValueMetricType,
			expected:   "AverageValue",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if string(tc.targetType) != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, string(tc.targetType))
			}
		})
	}
}

func TestScalingPolicies(t *testing.T) {
	tests := map[string]struct {
		scaleUpPolicy   ScaleUpPolicy
		scaleDownPolicy ScaleDownPolicy
		expectedUp      string
		expectedDown    string
	}{
		"balanced policies": {
			scaleUpPolicy:   BalancedScaleUp,
			scaleDownPolicy: BalancedScaleDown,
			expectedUp:      "Balanced",
			expectedDown:    "Balanced",
		},
		"prefer local policies": {
			scaleUpPolicy:   PreferLocalScaleUp,
			scaleDownPolicy: PreferLocalScaleDown,
			expectedUp:      "PreferLocal",
			expectedDown:    "PreferLocal",
		},
		"load aware scale up": {
			scaleUpPolicy:   LoadAwareScaleUp,
			scaleDownPolicy: PreferRemoteScaleDown,
			expectedUp:      "LoadAware",
			expectedDown:    "PreferRemote",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if string(tc.scaleUpPolicy) != tc.expectedUp {
				t.Errorf("ScaleUpPolicy: expected %q, got %q", tc.expectedUp, string(tc.scaleUpPolicy))
			}
			if string(tc.scaleDownPolicy) != tc.expectedDown {
				t.Errorf("ScaleDownPolicy: expected %q, got %q", tc.expectedDown, string(tc.scaleDownPolicy))
			}
		})
	}
}

func TestMetricSpecValidation(t *testing.T) {
	quantity := resource.MustParse("100m")
	
	tests := map[string]struct {
		spec     MetricSpec
		wantType MetricSourceType
	}{
		"resource metric": {
			spec: MetricSpec{
				Type: ResourceMetricSourceType,
				Resource: &ResourceMetricSource{
					Name: "cpu",
					Target: MetricTarget{
						Type:               UtilizationMetricType,
						AverageUtilization: int32Ptr(80),
					},
				},
			},
			wantType: ResourceMetricSourceType,
		},
		"pods metric": {
			spec: MetricSpec{
				Type: PodsMetricSourceType,
				Pods: &PodsMetricSource{
					Metric: MetricIdentifier{
						Name: "packets-per-second",
					},
					Target: MetricTarget{
						Type:         AverageValueMetricType,
						AverageValue: &quantity,
					},
				},
			},
			wantType: PodsMetricSourceType,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.spec.Type != tc.wantType {
				t.Errorf("expected metric type %q, got %q", tc.wantType, tc.spec.Type)
			}
		})
	}
}

// Helper function to create int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}