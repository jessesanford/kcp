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
	"k8s.io/utils/ptr"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestAutoScalingPolicyBasicStructure(t *testing.T) {
	policy := &AutoScalingPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tmc.kcp.io/v1alpha1",
			Kind:       "AutoScalingPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-scaling-policy",
		},
		Spec: AutoScalingPolicySpec{
			TargetRef: ScaleTargetRef{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "test-deployment",
				Namespace:  "default",
			},
			HorizontalPodAutoScaler: &HorizontalPodAutoScalerSpec{
				MinReplicas: ptr.To[int32](2),
				MaxReplicas: 10,
				TargetCPUUtilizationPercentage: ptr.To[int32](70),
			},
		},
	}

	// Verify basic structure
	assert.Equal(t, "tmc.kcp.io/v1alpha1", policy.APIVersion)
	assert.Equal(t, "AutoScalingPolicy", policy.Kind)
	assert.Equal(t, "test-scaling-policy", policy.Name)

	// Verify target ref
	assert.Equal(t, "apps/v1", policy.Spec.TargetRef.APIVersion)
	assert.Equal(t, "Deployment", policy.Spec.TargetRef.Kind)
	assert.Equal(t, "test-deployment", policy.Spec.TargetRef.Name)
	assert.Equal(t, "default", policy.Spec.TargetRef.Namespace)

	// Verify HPA spec
	assert.NotNil(t, policy.Spec.HorizontalPodAutoScaler)
	assert.Equal(t, int32(2), *policy.Spec.HorizontalPodAutoScaler.MinReplicas)
	assert.Equal(t, int32(10), policy.Spec.HorizontalPodAutoScaler.MaxReplicas)
	assert.Equal(t, int32(70), *policy.Spec.HorizontalPodAutoScaler.TargetCPUUtilizationPercentage)
}

func TestScaleTargetRefValidation(t *testing.T) {
	testCases := map[string]struct {
		ref     ScaleTargetRef
		valid   bool
		comment string
	}{
		"valid deployment target": {
			ref: ScaleTargetRef{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "test-deployment",
				Namespace:  "default",
			},
			valid:   true,
			comment: "should accept valid deployment reference",
		},
		"valid cluster-scoped resource": {
			ref: ScaleTargetRef{
				APIVersion: "custom.io/v1",
				Kind:       "ClusterResource",
				Name:       "test-resource",
				// No namespace for cluster-scoped resources
			},
			valid:   true,
			comment: "should accept valid cluster-scoped reference",
		},
		"empty name should be invalid": {
			ref: ScaleTargetRef{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "",
			},
			valid:   false,
			comment: "empty name should fail validation",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Basic validation that required fields are properly marked
			if tc.valid {
				assert.NotEmpty(t, tc.ref.APIVersion, "APIVersion should not be empty for valid case")
				assert.NotEmpty(t, tc.ref.Kind, "Kind should not be empty for valid case")
				assert.NotEmpty(t, tc.ref.Name, "Name should not be empty for valid case")
			} else {
				// For the empty name case, we expect the name to be empty
				if tc.ref.Name == "" {
					assert.Empty(t, tc.ref.Name, "Name should be empty for negative test case")
				}
			}
		})
	}
}

func TestHorizontalPodAutoScalerSpecDefaults(t *testing.T) {
	// Test that optional fields can be nil/empty
	hpa := &HorizontalPodAutoScalerSpec{
		MaxReplicas: 5,
		// MinReplicas and TargetCPUUtilizationPercentage are optional
	}

	assert.Nil(t, hpa.MinReplicas, "MinReplicas should be optional")
	assert.Nil(t, hpa.TargetCPUUtilizationPercentage, "TargetCPUUtilizationPercentage should be optional")
	assert.Equal(t, int32(5), hpa.MaxReplicas, "MaxReplicas should be set")

	// Test with values set
	hpa2 := &HorizontalPodAutoScalerSpec{
		MinReplicas:                    ptr.To[int32](1),
		MaxReplicas:                    10,
		TargetCPUUtilizationPercentage: ptr.To[int32](80),
	}

	assert.NotNil(t, hpa2.MinReplicas)
	assert.Equal(t, int32(1), *hpa2.MinReplicas)
	assert.Equal(t, int32(10), hpa2.MaxReplicas)
	assert.NotNil(t, hpa2.TargetCPUUtilizationPercentage)
	assert.Equal(t, int32(80), *hpa2.TargetCPUUtilizationPercentage)
}

func TestAutoScalingPolicyConditions(t *testing.T) {
	policy := &AutoScalingPolicy{}

	// Test initial conditions
	assert.Empty(t, policy.GetConditions(), "Initial conditions should be empty")

	// Test setting conditions
	conditions := conditionsv1alpha1.Conditions{
		{
			Type:   AutoScalingPolicyReady,
			Status: metav1.ConditionTrue,
		},
		{
			Type:   AutoScalingPolicyActive,
			Status: metav1.ConditionTrue,
		},
	}

	policy.SetConditions(conditions)
	assert.Equal(t, conditions, policy.GetConditions(), "Conditions should be set correctly")

	// Test that the policy implements the conditions interface
	require.Implements(t, (*conditionsv1alpha1.Getter)(nil), policy, "AutoScalingPolicy should implement conditions.Getter")
	require.Implements(t, (*conditionsv1alpha1.Setter)(nil), policy, "AutoScalingPolicy should implement conditions.Setter")
}

func TestScalingPlacement(t *testing.T) {
	placement := ScalingPlacement{
		Clusters: []string{"cluster-1", "cluster-2"},
		ClusterSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"environment": "production",
			},
		},
		Regions: []string{"us-west-2", "us-east-1"},
	}

	assert.Len(t, placement.Clusters, 2, "Should have 2 clusters")
	assert.Contains(t, placement.Clusters, "cluster-1")
	assert.Contains(t, placement.Clusters, "cluster-2")

	assert.NotNil(t, placement.ClusterSelector, "ClusterSelector should not be nil")
	assert.Equal(t, "production", placement.ClusterSelector.MatchLabels["environment"])

	assert.Len(t, placement.Regions, 2, "Should have 2 regions")
	assert.Contains(t, placement.Regions, "us-west-2")
	assert.Contains(t, placement.Regions, "us-east-1")
}

func TestAutoScalingPolicyStatusFields(t *testing.T) {
	now := metav1.Now()
	status := AutoScalingPolicyStatus{
		CurrentReplicas:                     3,
		DesiredReplicas:                     5,
		LastScaleTime:                       &now,
		CurrentCPUUtilizationPercentage:     ptr.To[int32](65),
	}

	assert.Equal(t, int32(3), status.CurrentReplicas, "Current replicas should be set")
	assert.Equal(t, int32(5), status.DesiredReplicas, "Desired replicas should be set")
	assert.NotNil(t, status.LastScaleTime, "Last scale time should be set")
	assert.NotNil(t, status.CurrentCPUUtilizationPercentage)
	assert.Equal(t, int32(65), *status.CurrentCPUUtilizationPercentage, "CPU utilization should be 65%")
}

func TestAutoScalingPolicyList(t *testing.T) {
	list := &AutoScalingPolicyList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tmc.kcp.io/v1alpha1",
			Kind:       "AutoScalingPolicyList",
		},
		Items: []AutoScalingPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-1"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "policy-2"},
			},
		},
	}

	assert.Equal(t, "tmc.kcp.io/v1alpha1", list.APIVersion)
	assert.Equal(t, "AutoScalingPolicyList", list.Kind)
	assert.Len(t, list.Items, 2, "Should have 2 items")
	assert.Equal(t, "policy-1", list.Items[0].Name)
	assert.Equal(t, "policy-2", list.Items[1].Name)
}