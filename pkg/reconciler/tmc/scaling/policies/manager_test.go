/*
Copyright The KCP Authors.

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

package policies

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/features"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpfakeclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/fake"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

func TestNewManager(t *testing.T) {
	tests := map[string]struct {
		featureGateEnabled bool
		expectFeatureFlag  bool
	}{
		"feature gate enabled": {
			featureGateEnabled: true,
			expectFeatureFlag:  true,
		},
		"feature gate disabled": {
			featureGateEnabled: false,
			expectFeatureFlag:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup fake clients
			kcpClusterClient := kcpfakeclientset.NewSimpleClusterClientset().Cluster(logicalcluster.Wildcard)
			informerFactory := kcpinformers.NewSharedInformerFactory(kcpClusterClient, time.Minute)

			// Mock feature gate
			originalGate := features.DefaultFeatureGate.Enabled(features.TMCScaling)
			defer func() {
				if originalGate != tc.featureGateEnabled {
					// Feature gates are typically immutable in tests, so we just check the behavior
				}
			}()

			manager, err := NewManager(kcpClusterClient, informerFactory)
			require.NoError(t, err)
			assert.NotNil(t, manager)
			assert.Equal(t, tc.expectFeatureFlag, manager.featureGateEnabled)
			assert.NotNil(t, manager.policyCache)
			assert.NotNil(t, manager.validator)
			assert.NotNil(t, manager.queue)
		})
	}
}

func TestManagerAddPolicy(t *testing.T) {
	tests := map[string]struct {
		featureEnabled bool
		policy         *ScalingPolicySpec
		expectError    bool
		errorContains  string
	}{
		"valid policy with feature enabled": {
			featureEnabled: true,
			policy: &ScalingPolicySpec{
				Target: ScalingTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
					Namespace:  "default",
				},
				Triggers: []ScalingTrigger{
					{
						Type: "cpu",
						Threshold: &ScalingThreshold{
							Metric:      "cpu",
							TargetValue: "80",
							Operator:    ">",
						},
					},
				},
				Constraints: ScalingConstraints{
					MinReplicas: int32Ptr(1),
					MaxReplicas: int32Ptr(10),
				},
			},
			expectError: false,
		},
		"feature disabled": {
			featureEnabled: false,
			policy: &ScalingPolicySpec{
				Target: ScalingTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
				},
				Triggers: []ScalingTrigger{
					{
						Type: "cpu",
						Threshold: &ScalingThreshold{
							Metric:      "cpu",
							TargetValue: "80",
							Operator:    ">",
						},
					},
				},
				Constraints: ScalingConstraints{},
			},
			expectError:   true,
			errorContains: "TMC scaling feature is disabled",
		},
		"invalid policy - no triggers": {
			featureEnabled: true,
			policy: &ScalingPolicySpec{
				Target: ScalingTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
				},
				Triggers:    []ScalingTrigger{},
				Constraints: ScalingConstraints{},
			},
			expectError:   true,
			errorContains: "at least one trigger must be specified",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			manager := createTestManager(t, tc.featureEnabled)
			cluster := logicalcluster.Name("root:test")

			err := manager.AddPolicy(cluster, "test-policy", tc.policy)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)

				// Verify policy was added to cache
				policy, exists := manager.GetPolicy(cluster, "test-policy")
				assert.True(t, exists)
				assert.Equal(t, tc.policy, policy)
			}
		})
	}
}

func TestManagerRemovePolicy(t *testing.T) {
	manager := createTestManager(t, true)
	cluster := logicalcluster.Name("root:test")

	// Add a policy first
	policy := createValidScalingPolicy()
	err := manager.AddPolicy(cluster, "test-policy", policy)
	require.NoError(t, err)

	// Verify policy exists
	_, exists := manager.GetPolicy(cluster, "test-policy")
	assert.True(t, exists)

	// Remove the policy
	manager.RemovePolicy(cluster, "test-policy")

	// Verify policy was removed
	_, exists = manager.GetPolicy(cluster, "test-policy")
	assert.False(t, exists)
}

func TestManagerListPolicies(t *testing.T) {
	manager := createTestManager(t, true)
	cluster1 := logicalcluster.Name("root:cluster1")
	cluster2 := logicalcluster.Name("root:cluster2")

	// Add policies to different clusters
	policy1 := createValidScalingPolicy()
	policy2 := createValidScalingPolicy()
	policy2.Target.Name = "deployment2"

	err := manager.AddPolicy(cluster1, "policy1", policy1)
	require.NoError(t, err)
	err = manager.AddPolicy(cluster1, "policy2", policy2)
	require.NoError(t, err)
	err = manager.AddPolicy(cluster2, "policy3", policy1)
	require.NoError(t, err)

	// List policies for cluster1
	policies1 := manager.ListPolicies(cluster1)
	assert.Len(t, policies1, 2)

	// List policies for cluster2
	policies2 := manager.ListPolicies(cluster2)
	assert.Len(t, policies2, 1)

	// List policies for non-existent cluster
	policies3 := manager.ListPolicies(logicalcluster.Name("root:nonexistent"))
	assert.Len(t, policies3, 0)
}

func TestPolicyValidation(t *testing.T) {
	validator := &PolicyValidator{timeout: defaultValidationTimeout}

	tests := map[string]struct {
		policy        *ScalingPolicySpec
		expectError   bool
		errorContains string
	}{
		"valid CPU-based policy": {
			policy: &ScalingPolicySpec{
				Target: ScalingTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
				},
				Triggers: []ScalingTrigger{
					{
						Type: "cpu",
						Threshold: &ScalingThreshold{
							Metric:      "cpu",
							TargetValue: "80",
							Operator:    ">",
						},
					},
				},
				Constraints: ScalingConstraints{
					MinReplicas: int32Ptr(1),
					MaxReplicas: int32Ptr(10),
				},
			},
			expectError: false,
		},
		"valid schedule-based policy": {
			policy: &ScalingPolicySpec{
				Target: ScalingTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
				},
				Triggers: []ScalingTrigger{
					{
						Type: "schedule",
						Schedule: &ScheduleTrigger{
							Cron:     "0 9 * * MON-FRI",
							Replicas: int32Ptr(5),
						},
					},
				},
				Constraints: ScalingConstraints{
					MinReplicas: int32Ptr(1),
					MaxReplicas: int32Ptr(10),
				},
			},
			expectError: false,
		},
		"invalid target - missing apiVersion": {
			policy: &ScalingPolicySpec{
				Target: ScalingTarget{
					Kind: "Deployment",
					Name: "test-deployment",
				},
				Triggers: []ScalingTrigger{
					{
						Type: "cpu",
						Threshold: &ScalingThreshold{
							Metric:      "cpu",
							TargetValue: "80",
							Operator:    ">",
						},
					},
				},
				Constraints: ScalingConstraints{},
			},
			expectError:   true,
			errorContains: "apiVersion is required",
		},
		"invalid trigger - unsupported type": {
			policy: &ScalingPolicySpec{
				Target: ScalingTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
				},
				Triggers: []ScalingTrigger{
					{
						Type: "invalid-type",
					},
				},
				Constraints: ScalingConstraints{},
			},
			expectError:   true,
			errorContains: "unsupported trigger type",
		},
		"invalid constraints - minReplicas > maxReplicas": {
			policy: &ScalingPolicySpec{
				Target: ScalingTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
				},
				Triggers: []ScalingTrigger{
					{
						Type: "cpu",
						Threshold: &ScalingThreshold{
							Metric:      "cpu",
							TargetValue: "80",
							Operator:    ">",
						},
					},
				},
				Constraints: ScalingConstraints{
					MinReplicas: int32Ptr(10),
					MaxReplicas: int32Ptr(5),
				},
			},
			expectError:   true,
			errorContains: "minReplicas cannot be greater than maxReplicas",
		},
		"invalid behavior - invalid selectPolicy": {
			policy: &ScalingPolicySpec{
				Target: ScalingTarget{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
				},
				Triggers: []ScalingTrigger{
					{
						Type: "cpu",
						Threshold: &ScalingThreshold{
							Metric:      "cpu",
							TargetValue: "80",
							Operator:    ">",
						},
					},
				},
				Constraints: ScalingConstraints{},
				Behavior: &ScalingBehavior{
					ScaleUp: &ScalingPolicyBehavior{
						SelectPolicy: stringPtr("InvalidPolicy"),
					},
				},
			},
			expectError:   true,
			errorContains: "invalid selectPolicy",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validator.ValidatePolicy(context.Background(), tc.policy)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateTarget(t *testing.T) {
	validator := &PolicyValidator{timeout: defaultValidationTimeout}

	tests := map[string]struct {
		target        *ScalingTarget
		expectError   bool
		errorContains string
	}{
		"valid target": {
			target: &ScalingTarget{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "test-deployment",
				Namespace:  "default",
			},
			expectError: false,
		},
		"missing apiVersion": {
			target: &ScalingTarget{
				Kind: "Deployment",
				Name: "test-deployment",
			},
			expectError:   true,
			errorContains: "apiVersion is required",
		},
		"missing kind": {
			target: &ScalingTarget{
				APIVersion: "apps/v1",
				Name:       "test-deployment",
			},
			expectError:   true,
			errorContains: "kind is required",
		},
		"missing name": {
			target: &ScalingTarget{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			expectError:   true,
			errorContains: "name is required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validator.validateTarget(tc.target)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateTrigger(t *testing.T) {
	validator := &PolicyValidator{timeout: defaultValidationTimeout}

	tests := map[string]struct {
		trigger       *ScalingTrigger
		expectError   bool
		errorContains string
	}{
		"valid CPU trigger": {
			trigger: &ScalingTrigger{
				Type: "cpu",
				Threshold: &ScalingThreshold{
					Metric:      "cpu",
					TargetValue: "80",
					Operator:    ">",
				},
			},
			expectError: false,
		},
		"valid schedule trigger": {
			trigger: &ScalingTrigger{
				Type: "schedule",
				Schedule: &ScheduleTrigger{
					Cron:     "0 9 * * MON-FRI",
					Replicas: int32Ptr(5),
				},
			},
			expectError: false,
		},
		"unsupported trigger type": {
			trigger: &ScalingTrigger{
				Type: "unsupported",
			},
			expectError:   true,
			errorContains: "unsupported trigger type",
		},
		"metric trigger without threshold": {
			trigger: &ScalingTrigger{
				Type: "cpu",
			},
			expectError:   true,
			errorContains: "threshold is required for metric-based triggers",
		},
		"schedule trigger without schedule": {
			trigger: &ScalingTrigger{
				Type: "schedule",
			},
			expectError:   true,
			errorContains: "schedule is required for schedule triggers",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validator.validateTrigger(tc.trigger)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints(t *testing.T) {
	validator := &PolicyValidator{timeout: defaultValidationTimeout}

	tests := map[string]struct {
		constraints   *ScalingConstraints
		expectError   bool
		errorContains string
	}{
		"valid constraints": {
			constraints: &ScalingConstraints{
				MinReplicas:  int32Ptr(1),
				MaxReplicas:  int32Ptr(10),
				MaxScaleUp:   int32Ptr(3),
				MaxScaleDown: int32Ptr(2),
			},
			expectError: false,
		},
		"negative minReplicas": {
			constraints: &ScalingConstraints{
				MinReplicas: int32Ptr(-1),
			},
			expectError:   true,
			errorContains: "minReplicas cannot be negative",
		},
		"negative maxReplicas": {
			constraints: &ScalingConstraints{
				MaxReplicas: int32Ptr(-1),
			},
			expectError:   true,
			errorContains: "maxReplicas cannot be negative",
		},
		"minReplicas > maxReplicas": {
			constraints: &ScalingConstraints{
				MinReplicas: int32Ptr(10),
				MaxReplicas: int32Ptr(5),
			},
			expectError:   true,
			errorContains: "minReplicas cannot be greater than maxReplicas",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validator.validateConstraints(tc.constraints)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestManagerSyncHandler(t *testing.T) {
	tests := map[string]struct {
		key           string
		setupPolicy   bool
		expectError   bool
		errorContains string
	}{
		"valid key with existing policy": {
			key:         "root:test::test-policy",
			setupPolicy: true,
			expectError: false,
		},
		"valid key with non-existent policy": {
			key:         "root:test::non-existent",
			setupPolicy: false,
			expectError: false,
		},
		"invalid key format": {
			key:           "invalid-key",
			setupPolicy:   false,
			expectError:   true,
			errorContains: "error parsing key",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			manager := createTestManager(t, true)

			if tc.setupPolicy {
				cluster := logicalcluster.Name("root:test")
				policy := createValidScalingPolicy()
				err := manager.AddPolicy(cluster, "test-policy", policy)
				require.NoError(t, err)
			}

			err := manager.syncHandler(context.Background(), tc.key)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Helper functions

func createTestManager(t *testing.T, featureEnabled bool) *Manager {
	kcpClusterClient := kcpfakeclientset.NewSimpleClusterClientset().Cluster(logicalcluster.Wildcard)
	informerFactory := kcpinformers.NewSharedInformerFactory(kcpClusterClient, time.Minute)

	manager, err := NewManager(kcpClusterClient, informerFactory)
	require.NoError(t, err)

	// Override feature gate for testing
	manager.featureGateEnabled = featureEnabled

	return manager
}

func createValidScalingPolicy() *ScalingPolicySpec {
	return &ScalingPolicySpec{
		Target: ScalingTarget{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "test-deployment",
			Namespace:  "default",
		},
		Triggers: []ScalingTrigger{
			{
				Type: "cpu",
				Threshold: &ScalingThreshold{
					Metric:      "cpu",
					TargetValue: "80",
					Operator:    ">",
				},
			},
		},
		Constraints: ScalingConstraints{
			MinReplicas: int32Ptr(1),
			MaxReplicas: int32Ptr(10),
		},
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}

func stringPtr(s string) *string {
	return &s
}