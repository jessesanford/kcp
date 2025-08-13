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
)

func TestClusterRegistrationValidation(t *testing.T) {
	tests := map[string]struct {
		cluster *ClusterRegistration
		wantErr bool
	}{
		"valid cluster registration": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				Spec: ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: ClusterEndpoint{
						ServerURL: "https://api.example.com",
						CABundle:  []byte("test-ca-bundle"),
						TLSConfig: &TLSConfig{
							InsecureSkipVerify: false,
						},
					},
					Capacity: ClusterCapacity{
						CPU:     int64Ptr(8000), // 8 CPUs in milliCPU
						Memory:  int64Ptr(16 * 1024 * 1024 * 1024), // 16GB in bytes
						MaxPods: int32Ptr(110),
					},
				},
			},
			wantErr: false,
		},
		"minimal cluster registration": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "minimal-cluster",
				},
				Spec: ClusterRegistrationSpec{
					Location: "us-east-1",
					ClusterEndpoint: ClusterEndpoint{
						ServerURL: "https://minimal.example.com",
					},
				},
			},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.cluster.Spec.Location == "" {
				t.Error("Location is required but was empty")
			}
			
			if tc.cluster.Spec.ClusterEndpoint.ServerURL == "" {
				t.Error("ServerURL is required but was empty")
			}
			
			// Verify deepcopy works
			copy := tc.cluster.DeepCopy()
			if copy == nil {
				t.Error("DeepCopy returned nil")
			}
			
			if copy.Name != tc.cluster.Name {
				t.Errorf("DeepCopy failed: expected name %s, got %s", tc.cluster.Name, copy.Name)
			}
		})
	}
}

func TestClusterResourceUsage(t *testing.T) {
	usage := &ClusterResourceUsage{
		CPU:    int64Ptr(4000), // 4 CPUs in milliCPU
		Memory: int64Ptr(8 * 1024 * 1024 * 1024), // 8GB in bytes
		Pods:   int32Ptr(50),
	}

	// Test deepcopy
	copy := usage.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}

	if *copy.CPU != *usage.CPU {
		t.Errorf("DeepCopy failed: expected CPU %d, got %d", *usage.CPU, *copy.CPU)
	}

	if *copy.Memory != *usage.Memory {
		t.Errorf("DeepCopy failed: expected Memory %d, got %d", *usage.Memory, *copy.Memory)
	}

	if *copy.Pods != *usage.Pods {
		t.Errorf("DeepCopy failed: expected Pods %d, got %d", *usage.Pods, *copy.Pods)
	}
}

func TestClusterCapabilities(t *testing.T) {
	capabilities := &ClusterCapabilities{
		KubernetesVersion:    "v1.28.0",
		SupportedAPIVersions: []string{"apps/v1", "v1"},
		AvailableResources:   []string{"pods", "services", "deployments"},
		NodeCount:            int32Ptr(3),
		Features:             []string{"pv-provisioning", "load-balancer"},
	}

	// Test deepcopy
	copy := capabilities.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}

	if copy.KubernetesVersion != capabilities.KubernetesVersion {
		t.Errorf("DeepCopy failed: expected version %s, got %s", 
			capabilities.KubernetesVersion, copy.KubernetesVersion)
	}

	if len(copy.Features) != len(capabilities.Features) {
		t.Errorf("DeepCopy failed: expected %d features, got %d", 
			len(capabilities.Features), len(copy.Features))
	}
}

// Helper functions for pointer values
func int64Ptr(i int64) *int64 {
	return &i
}

func int32Ptr(i int32) *int32 {
	return &i
}