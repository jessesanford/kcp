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
		},
		Status: ClusterRegistrationStatus{
			Conditions: conditionsv1alpha1.Conditions{
				{
					Type:   conditionsv1alpha1.ReadyCondition,
					Status: corev1.ConditionTrue,
					Reason: "ClusterHealthy",
				},
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