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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestClusterRegistrationDefaults(t *testing.T) {
	cluster := &ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: ClusterRegistrationSpec{
			Location: "us-west-2",
			ClusterEndpoint: ClusterEndpoint{
				ServerURL: "https://test-cluster.example.com",
			},
		},
	}

	// Verify the object is well-formed
	if cluster.Spec.Location != "us-west-2" {
		t.Errorf("Expected location us-west-2, got %s", cluster.Spec.Location)
	}

	if cluster.Spec.ClusterEndpoint.ServerURL != "https://test-cluster.example.com" {
		t.Errorf("Expected server URL https://test-cluster.example.com, got %s", cluster.Spec.ClusterEndpoint.ServerURL)
	}
}

func TestClusterRegistrationStatus(t *testing.T) {
	now := metav1.Now()
	cluster := &ClusterRegistration{
		Status: ClusterRegistrationStatus{
			LastHeartbeat: &now,
			Conditions: conditionsv1alpha1.Conditions{
				{
					Type:   "Ready",
					Status: corev1.ConditionTrue,
				},
			},
			AllocatedResources: &ClusterResourceUsage{
				CPU:    int64Ptr(2000),                   // 2 CPU cores in milliCPU
				Memory: int64Ptr(4 * 1024 * 1024 * 1024), // 4GB in bytes
				Pods:   int32Ptr(10),
			},
		},
	}

	// Verify status fields
	if cluster.Status.LastHeartbeat == nil {
		t.Error("Expected LastHeartbeat to be set")
	}

	if len(cluster.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(cluster.Status.Conditions))
	}

	if cluster.Status.Conditions[0].Type != "Ready" {
		t.Errorf("Expected condition type Ready, got %s", cluster.Status.Conditions[0].Type)
	}

	if cluster.Status.AllocatedResources == nil {
		t.Error("Expected AllocatedResources to be set")
	}

	if *cluster.Status.AllocatedResources.CPU != 2000 {
		t.Errorf("Expected CPU 2000, got %d", *cluster.Status.AllocatedResources.CPU)
	}
}

func TestClusterCapacity(t *testing.T) {
	capacity := ClusterCapacity{
		CPU:     int64Ptr(4000),                   // 4 CPU cores
		Memory:  int64Ptr(8 * 1024 * 1024 * 1024), // 8GB
		MaxPods: int32Ptr(110),
	}

	if *capacity.CPU != 4000 {
		t.Errorf("Expected CPU 4000, got %d", *capacity.CPU)
	}

	if *capacity.Memory != 8*1024*1024*1024 {
		t.Errorf("Expected Memory 8GB, got %d", *capacity.Memory)
	}

	if *capacity.MaxPods != 110 {
		t.Errorf("Expected MaxPods 110, got %d", *capacity.MaxPods)
	}
}

func TestTLSConfig(t *testing.T) {
	tlsConfig := &TLSConfig{
		InsecureSkipVerify: true,
	}

	if !tlsConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}
}

// Helper functions for pointer creation
func int64Ptr(i int64) *int64 {
	return &i
}

func int32Ptr(i int32) *int32 {
	return &i
}
