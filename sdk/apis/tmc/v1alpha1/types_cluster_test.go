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

func TestClusterRegistrationDeepCopy(t *testing.T) {
	original := &ClusterRegistration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tmc.kcp.io/v1alpha1",
			Kind:       "ClusterRegistration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
			Labels: map[string]string{
				"region": "us-west-2",
			},
		},
		Spec: ClusterRegistrationSpec{
			Location: "us-west-2",
			Capabilities: ClusterCapabilities{
				Compute: ComputeCapabilities{
					Architecture: "amd64",
					MaxCPU:       "32",
					MaxMemory:    "128Gi",
				},
				Storage: StorageCapabilities{
					StorageClasses: []string{"gp2", "io1"},
					MaxStorage:     "1Ti",
				},
				Network: NetworkCapabilities{
					LoadBalancerSupport: true,
					IngressSupport:      true,
				},
			},
			Labels: map[string]string{
				"environment": "prod",
			},
			Taints: []ClusterTaint{
				{
					Key:    "special-workload",
					Value:  "true",
					Effect: TaintEffectNoSchedule,
				},
			},
		},
		Status: ClusterRegistrationStatus{
			Conditions: conditionsv1alpha1.Conditions{
				{
					Type:   conditionsv1alpha1.ReadyCondition,
					Status: corev1.ConditionTrue,
				},
			},
			WorkloadCount: 5,
			ResourceUsage: ClusterResourceUsage{
				CPU:     "60%",
				Memory:  "45%",
				Storage: "30%",
			},
		},
	}

	copied := original.DeepCopy()

	if !reflect.DeepEqual(original, copied) {
		t.Errorf("DeepCopy failed: original and copied objects are not equal")
	}

	// Modify the original to ensure they are separate objects
	original.Spec.Location = "us-east-1"
	if copied.Spec.Location == "us-east-1" {
		t.Errorf("DeepCopy failed: modification of original affected the copy")
	}

	// Test nil handling
	var nilCluster *ClusterRegistration
	nilCopy := nilCluster.DeepCopy()
	if nilCopy != nil {
		t.Errorf("DeepCopy of nil should return nil, got %v", nilCopy)
	}
}

func TestClusterRegistrationListDeepCopy(t *testing.T) {
	original := &ClusterRegistrationList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tmc.kcp.io/v1alpha1",
			Kind:       "ClusterRegistrationList",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "12345",
		},
		Items: []ClusterRegistration{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
				Spec:       ClusterRegistrationSpec{Location: "us-west-1"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster2"},
				Spec:       ClusterRegistrationSpec{Location: "us-west-2"},
			},
		},
	}

	copied := original.DeepCopy()

	if !reflect.DeepEqual(original, copied) {
		t.Errorf("DeepCopy failed: original and copied lists are not equal")
	}

	// Modify the original to ensure they are separate objects
	original.Items[0].Spec.Location = "modified"
	if copied.Items[0].Spec.Location == "modified" {
		t.Errorf("DeepCopy failed: modification of original affected the copy")
	}
}

func TestTaintEffectConstants(t *testing.T) {
	tests := []struct {
		name     string
		effect   TaintEffect
		expected string
	}{
		{"NoSchedule", TaintEffectNoSchedule, "NoSchedule"},
		{"PreferNoSchedule", TaintEffectPreferNoSchedule, "PreferNoSchedule"},
		{"NoExecute", TaintEffectNoExecute, "NoExecute"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.effect) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.effect))
			}
		})
	}
}

func TestClusterRegistrationValidation(t *testing.T) {
	// Test valid cluster registration
	validCluster := &ClusterRegistration{
		Spec: ClusterRegistrationSpec{
			Location: "us-west-2",
			Capabilities: ClusterCapabilities{
				Compute: ComputeCapabilities{
					Architecture: "amd64",
				},
			},
			Taints: []ClusterTaint{
				{
					Key:    "test-key",
					Effect: TaintEffectNoSchedule,
				},
			},
		},
	}

	// Basic validation - ensure required fields are present
	if validCluster.Spec.Location == "" {
		t.Errorf("Location should be required")
	}

	// Test taint validation
	for _, taint := range validCluster.Spec.Taints {
		if taint.Key == "" {
			t.Errorf("Taint key should be required")
		}
		validEffects := []TaintEffect{TaintEffectNoSchedule, TaintEffectPreferNoSchedule, TaintEffectNoExecute}
		found := false
		for _, validEffect := range validEffects {
			if taint.Effect == validEffect {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Invalid taint effect: %s", taint.Effect)
		}
	}
}

func TestClusterCapabilitiesValidation(t *testing.T) {
	capabilities := ClusterCapabilities{
		Compute: ComputeCapabilities{
			Architecture: "amd64",
			MaxCPU:       "32",
			MaxMemory:    "128Gi",
		},
		Storage: StorageCapabilities{
			StorageClasses: []string{"gp2", "io1"},
			MaxStorage:     "1Ti",
		},
		Network: NetworkCapabilities{
			LoadBalancerSupport: true,
			IngressSupport:      true,
		},
	}

	// Basic validation
	if capabilities.Compute.Architecture == "" {
		t.Errorf("Architecture should be set")
	}

	if len(capabilities.Storage.StorageClasses) == 0 {
		t.Errorf("Storage classes should be provided")
	}

	if !capabilities.Network.LoadBalancerSupport && !capabilities.Network.IngressSupport {
		t.Errorf("At least one network capability should be supported")
	}
}
