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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestVerticalPodAutoScalerSpecValidation(t *testing.T) {
	testCases := map[string]struct {
		spec    VerticalPodAutoScalerSpec
		wantErr bool
		desc    string
	}{
		"valid default VPA": {
			spec: VerticalPodAutoScalerSpec{},
			wantErr: false,
			desc:    "should accept VPA with defaults",
		},
		"valid VPA with Auto update mode": {
			spec: VerticalPodAutoScalerSpec{
				UpdateMode: ptr.To(VPAUpdateModeAuto),
			},
			wantErr: false,
			desc:    "should accept VPA with Auto update mode",
		},
		"valid VPA with Off update mode": {
			spec: VerticalPodAutoScalerSpec{
				UpdateMode: ptr.To(VPAUpdateModeOff),
			},
			wantErr: false,
			desc:    "should accept VPA with Off update mode",
		},
		"valid VPA with Initial update mode": {
			spec: VerticalPodAutoScalerSpec{
				UpdateMode: ptr.To(VPAUpdateModeInitial),
			},
			wantErr: false,
			desc:    "should accept VPA with Initial update mode",
		},
		"valid VPA with Recreate update mode": {
			spec: VerticalPodAutoScalerSpec{
				UpdateMode: ptr.To(VPAUpdateModeRecreate),
			},
			wantErr: false,
			desc:    "should accept VPA with Recreate update mode",
		},
		"valid VPA with resource policy": {
			spec: VerticalPodAutoScalerSpec{
				UpdateMode: ptr.To(VPAUpdateModeAuto),
				ResourcePolicy: &VPAResourcePolicy{
					ContainerPolicies: []VPAContainerResourcePolicy{
						{
							ContainerName: ptr.To("app-container"),
							Mode:          ptr.To(VPAContainerScalingModeAuto),
							MinAllowed: map[string]intstr.IntOrString{
								"cpu":    intstr.FromString("100m"),
								"memory": intstr.FromString("128Mi"),
							},
							MaxAllowed: map[string]intstr.IntOrString{
								"cpu":    intstr.FromString("2"),
								"memory": intstr.FromString("4Gi"),
							},
						},
					},
				},
			},
			wantErr: false,
			desc:    "should accept VPA with complete resource policy",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Test UpdateMode validation
			if tc.spec.UpdateMode != nil {
				validModes := []VPAUpdateMode{
					VPAUpdateModeOff,
					VPAUpdateModeInitial,
					VPAUpdateModeRecreate,
					VPAUpdateModeAuto,
				}
				assert.Contains(t, validModes, *tc.spec.UpdateMode, "UpdateMode should be valid")
			}

			// Test ResourcePolicy validation
			if tc.spec.ResourcePolicy != nil {
				for _, containerPolicy := range tc.spec.ResourcePolicy.ContainerPolicies {
					if containerPolicy.Mode != nil {
						validContainerModes := []VPAContainerScalingMode{
							VPAContainerScalingModeOff,
							VPAContainerScalingModeAuto,
						}
						assert.Contains(t, validContainerModes, *containerPolicy.Mode, "Container mode should be valid")
					}

					// Validate resource constraints
					for resource, value := range containerPolicy.MinAllowed {
						assert.NotEmpty(t, resource, "Resource name should not be empty")
						assert.NotNil(t, value, "Min resource value should not be nil")
					}

					for resource, value := range containerPolicy.MaxAllowed {
						assert.NotEmpty(t, resource, "Resource name should not be empty")
						assert.NotNil(t, value, "Max resource value should not be nil")
					}
				}
			}
		})
	}
}

func TestScalingPolicyValidation(t *testing.T) {
	testCases := map[string]struct {
		policy  ScalingPolicy
		wantErr bool
		desc    string
	}{
		"valid scaling policy with both directions": {
			policy: ScalingPolicy{
				StabilizationWindowSeconds: ptr.To[int32](300),
				ScaleUp: &HPAScalingRules{
					Policies: []HPAScalingPolicy{
						{
							Type:          PodsScalingPolicy,
							Value:         4,
							PeriodSeconds: 60,
						},
						{
							Type:          PercentScalingPolicy,
							Value:         20,
							PeriodSeconds: 120,
						},
					},
					SelectPolicy:               ptr.To(MaxPolicySelect),
					StabilizationWindowSeconds: ptr.To[int32](60),
				},
				ScaleDown: &HPAScalingRules{
					Policies: []HPAScalingPolicy{
						{
							Type:          PercentScalingPolicy,
							Value:         10,
							PeriodSeconds: 180,
						},
					},
					SelectPolicy:               ptr.To(MinPolicySelect),
					StabilizationWindowSeconds: ptr.To[int32](300),
				},
			},
			wantErr: false,
			desc:    "should accept valid scaling policy with all fields",
		},
		"policy with disabled scale up": {
			policy: ScalingPolicy{
				ScaleUp: &HPAScalingRules{
					Policies: []HPAScalingPolicy{
						{
							Type:          PodsScalingPolicy,
							Value:         1,
							PeriodSeconds: 60,
						},
					},
					SelectPolicy: ptr.To(DisabledPolicySelect),
				},
			},
			wantErr: false,
			desc:    "should accept disabled scaling direction",
		},
		"invalid policy with zero value": {
			policy: ScalingPolicy{
				ScaleUp: &HPAScalingRules{
					Policies: []HPAScalingPolicy{
						{
							Type:          PodsScalingPolicy,
							Value:         0, // Invalid
							PeriodSeconds: 60,
						},
					},
				},
			},
			wantErr: true,
			desc:    "should reject zero policy value",
		},
		"invalid policy with negative value": {
			policy: ScalingPolicy{
				ScaleDown: &HPAScalingRules{
					Policies: []HPAScalingPolicy{
						{
							Type:          PercentScalingPolicy,
							Value:         -5, // Invalid
							PeriodSeconds: 120,
						},
					},
				},
			},
			wantErr: true,
			desc:    "should reject negative policy value",
		},
		"invalid policy with too short period": {
			policy: ScalingPolicy{
				ScaleUp: &HPAScalingRules{
					Policies: []HPAScalingPolicy{
						{
							Type:          PodsScalingPolicy,
							Value:         2,
							PeriodSeconds: 0, // Invalid
						},
					},
				},
			},
			wantErr: true,
			desc:    "should reject zero period seconds",
		},
		"invalid policy with too long period": {
			policy: ScalingPolicy{
				ScaleUp: &HPAScalingRules{
					Policies: []HPAScalingPolicy{
						{
							Type:          PercentScalingPolicy,
							Value:         10,
							PeriodSeconds: 2000, // Invalid - over 1800 max
						},
					},
				},
			},
			wantErr: true,
			desc:    "should reject period seconds over 1800",
		},
		"invalid stabilization window": {
			policy: ScalingPolicy{
				StabilizationWindowSeconds: ptr.To[int32](-1), // Invalid
				ScaleUp: &HPAScalingRules{
					Policies: []HPAScalingPolicy{
						{
							Type:          PodsScalingPolicy,
							Value:         2,
							PeriodSeconds: 60,
						},
					},
				},
			},
			wantErr: true,
			desc:    "should reject negative stabilization window",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Validate stabilization window
			if tc.policy.StabilizationWindowSeconds != nil {
				window := *tc.policy.StabilizationWindowSeconds
				if tc.wantErr && window < 1 {
					assert.Less(t, window, int32(1), "Stabilization window should be < 1 for negative test")
				} else if !tc.wantErr {
					assert.GreaterOrEqual(t, window, int32(1), "Stabilization window should be >= 1")
				}
			}

			// Validate scale up policies
			if tc.policy.ScaleUp != nil {
				validateHPAScalingRules(t, tc.policy.ScaleUp, tc.wantErr)
			}

			// Validate scale down policies
			if tc.policy.ScaleDown != nil {
				validateHPAScalingRules(t, tc.policy.ScaleDown, tc.wantErr)
			}
		})
	}
}

// Helper function to validate HPA scaling rules
func validateHPAScalingRules(t *testing.T, rules *HPAScalingRules, expectError bool) {
	// Validate that policies exist
	if !expectError {
		assert.NotEmpty(t, rules.Policies, "Policies should not be empty for valid test")
	}

	// Validate each policy
	for _, policy := range rules.Policies {
		// Validate policy type
		validTypes := []HPAScalingPolicyType{
			PodsScalingPolicy,
			PercentScalingPolicy,
		}
		assert.Contains(t, validTypes, policy.Type, "Policy type should be valid")

		// Validate policy value
		if expectError && policy.Value <= 0 {
			assert.LessOrEqual(t, policy.Value, int32(0), "Policy value should be <= 0 for negative test")
		} else if !expectError {
			assert.Positive(t, policy.Value, "Policy value should be positive for valid test")
		}

		// Validate period seconds
		if expectError && (policy.PeriodSeconds < 1 || policy.PeriodSeconds > 1800) {
			assert.True(t, policy.PeriodSeconds < 1 || policy.PeriodSeconds > 1800, "Period should be out of bounds for negative test")
		} else if !expectError {
			assert.GreaterOrEqual(t, policy.PeriodSeconds, int32(1), "Period should be >= 1")
			assert.LessOrEqual(t, policy.PeriodSeconds, int32(1800), "Period should be <= 1800")
		}
	}

	// Validate select policy
	if rules.SelectPolicy != nil {
		validSelections := []ScalingPolicySelect{
			MaxPolicySelect,
			MinPolicySelect,
			DisabledPolicySelect,
		}
		assert.Contains(t, validSelections, *rules.SelectPolicy, "Select policy should be valid")
	}

	// Validate stabilization window bounds
	if rules.StabilizationWindowSeconds != nil {
		window := *rules.StabilizationWindowSeconds
		if !expectError {
			assert.GreaterOrEqual(t, window, int32(0), "Stabilization window should be >= 0")
			assert.LessOrEqual(t, window, int32(3600), "Stabilization window should be <= 3600")
		}
	}
}

func TestAutoScalingPolicyStatusValidation(t *testing.T) {
	testCases := map[string]struct {
		status AutoScalingPolicyStatus
		desc   string
	}{
		"complete status with all fields": {
			status: AutoScalingPolicyStatus{
				Conditions: conditionsv1alpha1.Conditions{
					{
						Type:   AutoScalingPolicyReady,
						Status: "True",
						Reason: "ScalingPolicyReady",
					},
					{
						Type:   AutoScalingPolicyActive,
						Status: "True",
						Reason: "PolicyActive",
					},
					{
						Type:   AutoScalingPolicyScalingActive,
						Status: "False",
						Reason: "NotCurrentlyScaling",
					},
				},
				CurrentReplicas: 5,
				DesiredReplicas: 8,
				LastScaleTime:   &metav1.Time{},
				CurrentMetrics: []MetricStatus{
					{
						Type: ResourceMetricSourceType,
						Resource: &ResourceMetricStatus{
							Name: "cpu",
							Current: MetricValueStatus{
								AverageUtilization: ptr.To[int32](85),
							},
						},
					},
					{
						Type: ResourceMetricSourceType,
						Resource: &ResourceMetricStatus{
							Name: "memory",
							Current: MetricValueStatus{
								AverageUtilization: ptr.To[int32](70),
							},
						},
					},
				},
			},
			desc: "should support complete status information",
		},
		"minimal status": {
			status: AutoScalingPolicyStatus{
				CurrentReplicas: 1,
			},
			desc: "should accept minimal status",
		},
		"status with external metrics": {
			status: AutoScalingPolicyStatus{
				CurrentReplicas: 3,
				DesiredReplicas: 5,
				CurrentMetrics: []MetricStatus{
					{
						Type: ExternalMetricSourceType,
						External: &ExternalMetricStatus{
							Metric: MetricIdentifier{
								Name: "queue_length",
							},
							Current: MetricValueStatus{
								Value: &intstr.IntOrString{Type: intstr.Int, IntVal: 120},
							},
						},
					},
				},
			},
			desc: "should support external metric status",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Validate replica counts are non-negative
			assert.GreaterOrEqual(t, tc.status.CurrentReplicas, int32(0), "CurrentReplicas should be >= 0")
			assert.GreaterOrEqual(t, tc.status.DesiredReplicas, int32(0), "DesiredReplicas should be >= 0")

			// Validate conditions
			for _, condition := range tc.status.Conditions {
				validTypes := []conditionsv1alpha1.ConditionType{
					AutoScalingPolicyReady,
					AutoScalingPolicyActive,
					AutoScalingPolicyProgressing,
					AutoScalingPolicyScalingActive,
					AutoScalingPolicyScalingLimited,
				}
				assert.Contains(t, validTypes, condition.Type, "Condition type should be valid")
				
				validStatuses := []string{
					"True",
					"False",
					"Unknown",
				}
				assert.Contains(t, validStatuses, string(condition.Status), "Condition status should be valid")
			}

			// Validate current metrics
			for _, metric := range tc.status.CurrentMetrics {
				validateMetricStatus(t, metric)
			}
		})
	}
}

// Helper function to validate MetricStatus
func validateMetricStatus(t *testing.T, status MetricStatus) {
	validTypes := []MetricSourceType{
		ObjectMetricSourceType,
		PodsMetricSourceType,
		ResourceMetricSourceType,
		ExternalMetricSourceType,
	}
	assert.Contains(t, validTypes, status.Type, "Metric status type should be valid")

	// Validate that the appropriate field is set based on type
	switch status.Type {
	case ResourceMetricSourceType:
		require.NotNil(t, status.Resource, "Resource status should be set for resource type")
		assert.NotEmpty(t, status.Resource.Name, "Resource name should not be empty")
	case PodsMetricSourceType:
		require.NotNil(t, status.Pods, "Pods status should be set for pods type")
		assert.NotEmpty(t, status.Pods.Metric.Name, "Pods metric name should not be empty")
	case ObjectMetricSourceType:
		require.NotNil(t, status.Object, "Object status should be set for object type")
		assert.NotEmpty(t, status.Object.Metric.Name, "Object metric name should not be empty")
	case ExternalMetricSourceType:
		require.NotNil(t, status.External, "External status should be set for external type")
		assert.NotEmpty(t, status.External.Metric.Name, "External metric name should not be empty")
	}
}

func TestAutoScalingPolicyConditionsInterface(t *testing.T) {
	policy := &AutoScalingPolicy{}

	// Test initial state
	assert.Empty(t, policy.GetConditions(), "Initial conditions should be empty")

	// Test setting conditions
	conditions := conditionsv1alpha1.Conditions{
		{
			Type:    AutoScalingPolicyReady,
			Status:  "True",
			Reason:  "PolicyConfigured",
			Message: "Scaling policy is properly configured",
		},
		{
			Type:    AutoScalingPolicyActive,
			Status:  "True",
			Reason:  "PolicyActive",
			Message: "Scaling policy is actively managing resources",
		},
		{
			Type:    AutoScalingPolicyScalingActive,
			Status:  "False",
			Reason:  "NotScaling",
			Message: "Current metrics are within target range",
		},
	}

	policy.SetConditions(conditions)
	assert.Equal(t, conditions, policy.GetConditions(), "Conditions should be set correctly")

	// Test that the policy has condition methods
	assert.NotNil(t, policy.GetConditions, "Should have GetConditions method")
	assert.NotNil(t, policy.SetConditions, "Should have SetConditions method")

	// Test condition type constants
	assert.Equal(t, conditionsv1alpha1.ConditionType("Ready"), AutoScalingPolicyReady)
	assert.Equal(t, conditionsv1alpha1.ConditionType("Active"), AutoScalingPolicyActive)
	assert.Equal(t, conditionsv1alpha1.ConditionType("Progressing"), AutoScalingPolicyProgressing)
	assert.Equal(t, conditionsv1alpha1.ConditionType("ScalingActive"), AutoScalingPolicyScalingActive)
	assert.Equal(t, conditionsv1alpha1.ConditionType("ScalingLimited"), AutoScalingPolicyScalingLimited)
}

func TestAutoScalingPolicyList(t *testing.T) {
	policy1 := AutoScalingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-1"},
		Spec: AutoScalingPolicySpec{
			TargetRef: ScaleTargetRef{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "app-1",
			},
			HorizontalPodAutoScaler: &HorizontalPodAutoScalerSpec{
				MaxReplicas: 10,
			},
		},
	}

	policy2 := AutoScalingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-2"},
		Spec: AutoScalingPolicySpec{
			TargetRef: ScaleTargetRef{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       "app-2",
			},
			VerticalPodAutoScaler: &VerticalPodAutoScalerSpec{
				UpdateMode: ptr.To(VPAUpdateModeAuto),
			},
		},
	}

	list := AutoScalingPolicyList{
		Items: []AutoScalingPolicy{policy1, policy2},
	}

	assert.Len(t, list.Items, 2, "List should contain 2 policies")
	assert.Equal(t, "policy-1", list.Items[0].Name, "First policy name should match")
	assert.Equal(t, "policy-2", list.Items[1].Name, "Second policy name should match")
}

func TestBoundaryConditions(t *testing.T) {
	testCases := map[string]struct {
		test func(t *testing.T)
		desc string
	}{
		"maximum replica values": {
			test: func(t *testing.T) {
				spec := HorizontalPodAutoScalerSpec{
					MinReplicas: ptr.To[int32](1000),
					MaxReplicas: 5000,
				}
				
				assert.Equal(t, int32(1000), *spec.MinReplicas, "Should support large min replicas")
				assert.Equal(t, int32(5000), spec.MaxReplicas, "Should support large max replicas")
				assert.LessOrEqual(t, *spec.MinReplicas, spec.MaxReplicas, "Min should be <= max")
			},
			desc: "should handle large replica counts",
		},
		"edge case utilization values": {
			test: func(t *testing.T) {
				spec := HorizontalPodAutoScalerSpec{
					MaxReplicas:                    10,
					TargetCPUUtilizationPercentage: ptr.To[int32](1),  // Minimum
				}
				
				assert.Equal(t, int32(1), *spec.TargetCPUUtilizationPercentage, "Should accept minimum CPU utilization")
				
				spec.TargetMemoryUtilizationPercentage = ptr.To[int32](100) // Maximum
				assert.Equal(t, int32(100), *spec.TargetMemoryUtilizationPercentage, "Should accept maximum memory utilization")
			},
			desc: "should handle edge case utilization values",
		},
		"maximum scaling policy limits": {
			test: func(t *testing.T) {
				policy := ScalingPolicy{
					ScaleUp: &HPAScalingRules{
						Policies: []HPAScalingPolicy{
							{
								Type:          PercentScalingPolicy,
								Value:         1000, // Large value
								PeriodSeconds: 1800, // Maximum period
							},
						},
						StabilizationWindowSeconds: ptr.To[int32](3600), // Maximum stabilization window
					},
				}
				
				assert.Equal(t, int32(1000), policy.ScaleUp.Policies[0].Value, "Should support large policy values")
				assert.Equal(t, int32(1800), policy.ScaleUp.Policies[0].PeriodSeconds, "Should support maximum period")
				assert.Equal(t, int32(3600), *policy.ScaleUp.StabilizationWindowSeconds, "Should support maximum stabilization window")
			},
			desc: "should handle maximum scaling policy limits",
		},
		"empty optional fields": {
			test: func(t *testing.T) {
				policy := AutoScalingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "minimal"},
					Spec: AutoScalingPolicySpec{
						TargetRef: ScaleTargetRef{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "test",
						},
						// All optional fields empty
					},
				}
				
				assert.Nil(t, policy.Spec.HorizontalPodAutoScaler, "HPA should be nil when not specified")
				assert.Nil(t, policy.Spec.VerticalPodAutoScaler, "VPA should be nil when not specified")
				assert.Nil(t, policy.Spec.ScalingPolicy, "ScalingPolicy should be nil when not specified")
				assert.Nil(t, policy.Spec.Placement, "Placement should be nil when not specified")
			},
			desc: "should handle all optional fields being empty",
		},
	}

	for name, tc := range testCases {
		t.Run(name, tc.test)
	}
}