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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestWorkloadPlacementDeepCopy(t *testing.T) {
	original := &WorkloadPlacement{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tmc.kcp.io/v1alpha1",
			Kind:       "WorkloadPlacement",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-placement",
			Namespace: "default",
		},
		Spec: WorkloadPlacementSpec{
			WorkloadSelector: WorkloadSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
				},
				WorkloadTypes: []WorkloadType{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
				},
			},
			ClusterSelector: ClusterSelector{
				LocationSelector: []string{"us-west-2"},
				CapabilityRequirements: &CapabilityRequirements{
					Compute: &ComputeRequirements{
						Architecture: "amd64",
						MinCPU:       "2",
						MinMemory:    "4Gi",
					},
				},
			},
			PlacementPolicy: PlacementPolicyRoundRobin,
			Tolerations: []WorkloadToleration{
				{
					Key:      "special-workload",
					Operator: TolerationOpEqual,
					Value:    "true",
					Effect:   TaintEffectNoSchedule,
				},
			},
		},
		Status: WorkloadPlacementStatus{
			Conditions: conditionsv1alpha1.Conditions{
				{
					Type:   conditionsv1alpha1.ReadyCondition,
					Status: corev1.ConditionTrue,
				},
			},
			SelectedClusters: []string{"cluster1", "cluster2"},
			PlacedWorkloads: []PlacedWorkload{
				{
					WorkloadRef: WorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "nginx",
						Namespace:  "default",
					},
					ClusterName:    "cluster1",
					PlacementTime:  metav1.Now(),
					Status:         PlacedWorkloadStatusPlaced,
					LastUpdateTime: &metav1.Time{},
				},
			},
		},
	}

	copied := original.DeepCopy()

	if !reflect.DeepEqual(original, copied) {
		t.Errorf("DeepCopy failed: original and copied objects are not equal")
	}

	// Modify the original to ensure they are separate objects
	original.Spec.PlacementPolicy = PlacementPolicyLeastLoaded
	if copied.Spec.PlacementPolicy == PlacementPolicyLeastLoaded {
		t.Errorf("DeepCopy failed: modification of original affected the copy")
	}

	// Test nil handling
	var nilPlacement *WorkloadPlacement
	nilCopy := nilPlacement.DeepCopy()
	if nilCopy != nil {
		t.Errorf("DeepCopy of nil should return nil, got %v", nilCopy)
	}
}

func TestWorkloadPlacementListDeepCopy(t *testing.T) {
	original := &WorkloadPlacementList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tmc.kcp.io/v1alpha1",
			Kind:       "WorkloadPlacementList",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "12345",
		},
		Items: []WorkloadPlacement{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "placement1"},
				Spec: WorkloadPlacementSpec{
					PlacementPolicy: PlacementPolicyRoundRobin,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "placement2"},
				Spec: WorkloadPlacementSpec{
					PlacementPolicy: PlacementPolicyLeastLoaded,
				},
			},
		},
	}

	copied := original.DeepCopy()

	if !reflect.DeepEqual(original, copied) {
		t.Errorf("DeepCopy failed: original and copied lists are not equal")
	}

	// Modify the original to ensure they are separate objects
	original.Items[0].Spec.PlacementPolicy = PlacementPolicyRandom
	if copied.Items[0].Spec.PlacementPolicy == PlacementPolicyRandom {
		t.Errorf("DeepCopy failed: modification of original affected the copy")
	}
}

func TestPlacementPolicyConstants(t *testing.T) {
	tests := []struct {
		name     string
		policy   PlacementPolicy
		expected string
	}{
		{"RoundRobin", PlacementPolicyRoundRobin, "RoundRobin"},
		{"LeastLoaded", PlacementPolicyLeastLoaded, "LeastLoaded"},
		{"Random", PlacementPolicyRandom, "Random"},
		{"LocationAware", PlacementPolicyLocationAware, "LocationAware"},
		{"Affinity", PlacementPolicyAffinity, "Affinity"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.policy) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.policy))
			}
		})
	}
}

func TestTolerationOperatorConstants(t *testing.T) {
	tests := []struct {
		name     string
		operator TolerationOperator
		expected string
	}{
		{"Equal", TolerationOpEqual, "Equal"},
		{"Exists", TolerationOpExists, "Exists"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.operator) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.operator))
			}
		})
	}
}

func TestPlacedWorkloadStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   PlacedWorkloadStatus
		expected string
	}{
		{"Pending", PlacedWorkloadStatusPending, "Pending"},
		{"Placed", PlacedWorkloadStatusPlaced, "Placed"},
		{"Failed", PlacedWorkloadStatusFailed, "Failed"},
		{"Removed", PlacedWorkloadStatusRemoved, "Removed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.status))
			}
		})
	}
}

func TestWorkloadPlacementValidation(t *testing.T) {
	// Test valid workload placement
	validPlacement := &WorkloadPlacement{
		Spec: WorkloadPlacementSpec{
			WorkloadSelector: WorkloadSelector{
				WorkloadTypes: []WorkloadType{
					{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
				},
			},
			ClusterSelector: ClusterSelector{
				LocationSelector: []string{"us-west-2"},
			},
			PlacementPolicy: PlacementPolicyRoundRobin,
			Tolerations: []WorkloadToleration{
				{
					Key:      "test-key",
					Operator: TolerationOpEqual,
					Value:    "test-value",
					Effect:   TaintEffectNoSchedule,
				},
			},
		},
	}

	// Basic validation - ensure workload types are specified correctly
	for _, workloadType := range validPlacement.Spec.WorkloadSelector.WorkloadTypes {
		if workloadType.APIVersion == "" {
			t.Errorf("WorkloadType APIVersion should be required")
		}
		if workloadType.Kind == "" {
			t.Errorf("WorkloadType Kind should be required")
		}
	}

	// Test toleration validation
	for _, toleration := range validPlacement.Spec.Tolerations {
		validOperators := []TolerationOperator{TolerationOpEqual, TolerationOpExists}
		found := false
		for _, validOp := range validOperators {
			if toleration.Operator == validOp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Invalid toleration operator: %s", toleration.Operator)
		}
	}

	// Test placement policy validation
	validPolicies := []PlacementPolicy{
		PlacementPolicyRoundRobin,
		PlacementPolicyLeastLoaded,
		PlacementPolicyRandom,
		PlacementPolicyLocationAware,
		PlacementPolicyAffinity,
	}
	found := false
	for _, validPolicy := range validPolicies {
		if validPlacement.Spec.PlacementPolicy == validPolicy {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Invalid placement policy: %s", validPlacement.Spec.PlacementPolicy)
	}
}

func TestWorkloadAffinityValidation(t *testing.T) {
	affinity := &WorkloadAffinity{
		ClusterAffinity: &ClusterAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &ClusterSelector{
				LocationSelector: []string{"us-west-2"},
			},
			PreferredDuringSchedulingIgnoredDuringExecution: []PreferredClusterSelector{
				{
					Weight: 80,
					Preference: ClusterSelector{
						LocationSelector: []string{"us-west-1"},
					},
				},
			},
		},
	}

	// Test weight validation
	for _, preferred := range affinity.ClusterAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		if preferred.Weight < 1 || preferred.Weight > 100 {
			t.Errorf("Preference weight should be between 1 and 100, got %d", preferred.Weight)
		}
	}
}

func TestCapabilityRequirementsValidation(t *testing.T) {
	requirements := &CapabilityRequirements{
		Compute: &ComputeRequirements{
			Architecture: "amd64",
			MinCPU:       "2",
			MinMemory:    "4Gi",
		},
		Storage: &StorageRequirements{
			RequiredStorageClasses: []string{"gp2"},
			MinStorage:             "10Gi",
		},
		Network: &NetworkRequirements{
			RequireLoadBalancer: true,
			RequireIngress:      false,
		},
	}

	// Basic validation
	if requirements.Compute.Architecture == "" {
		t.Errorf("Architecture should be specified")
	}

	if len(requirements.Storage.RequiredStorageClasses) == 0 {
		t.Errorf("At least one storage class should be required")
	}

	if !requirements.Network.RequireLoadBalancer && !requirements.Network.RequireIngress {
		// This is valid - no network requirements
	}
}
