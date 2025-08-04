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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestClusterRegistrationScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	if err != nil {
		t.Fatalf("Failed to add TMC to scheme: %v", err)
	}

	// Verify ClusterRegistration is registered
	gvks, _, err := scheme.ObjectKinds(&ClusterRegistration{})
	if err != nil {
		t.Fatalf("Failed to get ObjectKinds: %v", err)
	}
	if len(gvks) == 0 {
		t.Error("ClusterRegistration should be registered in scheme")
	}
}

func TestClusterRegistrationDeepCopy(t *testing.T) {
	maxWorkloads := int32(100)
	original := &ClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		Spec: ClusterRegistrationSpec{
			Location: "us-west-2",
			ClusterEndpoint: ClusterEndpoint{
				ServerURL: "https://cluster.example.com:6443",
				CABundle:  []byte("test-ca-bundle"),
			},
			Capabilities: ClusterCapabilities{
				Architecture: "amd64",
				SupportedWorkloads: []WorkloadCapability{
					{
						Type:       "deployment",
						APIVersion: "apps/v1",
						Supported:  true,
					},
				},
				Features:          []string{"ingress", "storage"},
				KubernetesVersion: "v1.28.0",
			},
			Credentials: &ClusterCredentials{
				SecretRef: &corev1.LocalObjectReference{Name: "cluster-secret"},
			},
			ResourceQuotas: ClusterResourceQuotas{
				Hard: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100"),
					corev1.ResourceMemory: resource.MustParse("200Gi"),
				},
				MaxWorkloads: &maxWorkloads,
			},
		},
		Status: ClusterRegistrationStatus{
			Conditions: conditionsv1alpha1.Conditions{
				{
					Type:   conditionsv1alpha1.ReadyCondition,
					Status: corev1.ConditionTrue,
					Reason: "ClusterHealthy",
				},
			},
			WorkloadCount: 5,
			AllocatedResources: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("10"),
			},
		},
	}

	copy := original.DeepCopy()
	if copy == original {
		t.Error("DeepCopy should return a different object")
	}
	if copy.Name != original.Name {
		t.Errorf("DeepCopy failed: name mismatch")
	}
	if copy.Spec.Location != original.Spec.Location {
		t.Errorf("DeepCopy failed: location mismatch")
	}
}

func TestClusterRegistrationValidation(t *testing.T) {
	tests := map[string]struct {
		cluster *ClusterRegistration
		valid   bool
	}{
		"valid cluster": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: ClusterRegistrationSpec{
					Location: "us-west-2",
					ClusterEndpoint: ClusterEndpoint{
						ServerURL: "https://cluster.example.com:6443",
					},
				},
			},
			valid: true,
		},
		"empty location": {
			cluster: &ClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: ClusterRegistrationSpec{
					ClusterEndpoint: ClusterEndpoint{
						ServerURL: "https://cluster.example.com:6443",
					},
				},
			},
			valid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.cluster.Spec.Location == "" && tc.valid {
				t.Error("Expected validation to fail for empty location")
			}
		})
	}
}

func TestClusterCapabilitiesValidation(t *testing.T) {
	tests := map[string]struct {
		capabilities ClusterCapabilities
		valid        bool
	}{
		"valid capabilities": {
			capabilities: ClusterCapabilities{
				Architecture: "amd64",
				SupportedWorkloads: []WorkloadCapability{
					{Type: "deployment", APIVersion: "apps/v1", Supported: true},
				},
				Features:          []string{"ingress"},
				KubernetesVersion: "v1.28.0",
			},
			valid: true,
		},
		"invalid architecture": {
			capabilities: ClusterCapabilities{
				Architecture: "invalid-arch",
			},
			valid: false,
		},
		"empty workload type": {
			capabilities: ClusterCapabilities{
				SupportedWorkloads: []WorkloadCapability{
					{APIVersion: "apps/v1", Supported: true},
				},
			},
			valid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.capabilities.Architecture == "invalid-arch" && tc.valid {
				t.Error("Expected validation to fail for invalid architecture")
			}
			for _, wl := range tc.capabilities.SupportedWorkloads {
				if wl.Type == "" && tc.valid {
					t.Error("Expected validation to fail for empty workload type")
				}
			}
		})
	}
}

func TestClusterCredentialsValidation(t *testing.T) {
	tests := map[string]struct {
		credentials *ClusterCredentials
		valid       bool
	}{
		"secret ref credentials": {
			credentials: &ClusterCredentials{
				SecretRef: &corev1.LocalObjectReference{Name: "cluster-secret"},
			},
			valid: true,
		},
		"service account credentials": {
			credentials: &ClusterCredentials{
				ServiceAccountRef: &corev1.LocalObjectReference{Name: "cluster-sa"},
			},
			valid: true,
		},
		"token ref credentials": {
			credentials: &ClusterCredentials{
				TokenRef: &TokenReference{
					SecretRef: corev1.LocalObjectReference{Name: "token-secret"},
					Key:       "token",
				},
			},
			valid: true,
		},
		"empty credentials": {
			credentials: &ClusterCredentials{},
			valid:       false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isEmpty := tc.credentials.SecretRef == nil &&
				tc.credentials.ServiceAccountRef == nil &&
				tc.credentials.TokenRef == nil
			if isEmpty && tc.valid {
				t.Error("Expected validation to fail for empty credentials")
			}
		})
	}
}

func TestClusterResourceQuotasValidation(t *testing.T) {
	negativeWorkloads := int32(-1)
	validWorkloads := int32(50)

	tests := map[string]struct {
		quotas ClusterResourceQuotas
		valid  bool
	}{
		"valid quotas": {
			quotas: ClusterResourceQuotas{
				Hard: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100"),
					corev1.ResourceMemory: resource.MustParse("200Gi"),
				},
				MaxWorkloads: &validWorkloads,
			},
			valid: true,
		},
		"negative max workloads": {
			quotas: ClusterResourceQuotas{
				MaxWorkloads: &negativeWorkloads,
			},
			valid: false,
		},
		"empty quotas": {
			quotas: ClusterResourceQuotas{},
			valid:  true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.quotas.MaxWorkloads != nil && *tc.quotas.MaxWorkloads < 0 && tc.valid {
				t.Error("Expected validation to fail for negative max workloads")
			}
		})
	}
}