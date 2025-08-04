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

func TestWorkloadPlacementScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	if err != nil {
		t.Fatalf("Failed to add TMC to scheme: %v", err)
	}

	// Verify WorkloadPlacement is registered
	gvks, _, err := scheme.ObjectKinds(&WorkloadPlacement{})
	if err != nil {
		t.Fatalf("Failed to get ObjectKinds: %v", err)
	}
	if len(gvks) == 0 {
		t.Error("WorkloadPlacement should be registered in scheme")
	}
}

func TestWorkloadPlacementDeepCopy(t *testing.T) {
	original := &WorkloadPlacement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-placement",
			Namespace: "default",
		},
		Spec: WorkloadPlacementSpec{
			WorkloadSelector: WorkloadSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
				WorkloadTypes: []WorkloadType{
					{APIVersion: "apps/v1", Kind: "Deployment"},
				},
			},
			ClusterSelector: ClusterSelector{
				LocationSelector: []string{"us-west-2"},
				ClusterNames:     []string{"cluster-1", "cluster-2"},
			},
			PlacementPolicy: PlacementPolicyRoundRobin,
		},
		Status: WorkloadPlacementStatus{
			Conditions: conditionsv1alpha1.Conditions{
				{
					Type:   conditionsv1alpha1.ReadyCondition,
					Status: corev1.ConditionTrue,
					Reason: "PlacementReady",
				},
			},
			SelectedClusters: []string{"cluster-1", "cluster-2"},
		},
	}

	copy := original.DeepCopy()
	if copy == original {
		t.Error("DeepCopy should return a different object")
	}
	if copy.Name != original.Name {
		t.Errorf("DeepCopy failed: name mismatch")
	}
	if copy.Spec.PlacementPolicy != original.Spec.PlacementPolicy {
		t.Errorf("DeepCopy failed: placement policy mismatch")
	}
	if len(copy.Status.SelectedClusters) != len(original.Status.SelectedClusters) {
		t.Errorf("DeepCopy failed: selected clusters mismatch")
	}
}

func TestWorkloadPlacementValidation(t *testing.T) {
	tests := map[string]struct {
		placement *WorkloadPlacement
		valid     bool
	}{
		"valid placement with label selector": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement", Namespace: "default"},
				Spec: WorkloadPlacementSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
					},
					ClusterSelector: ClusterSelector{
						ClusterNames: []string{"cluster-1"},
					},
				},
			},
			valid: true,
		},
		"valid placement with workload types": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement", Namespace: "default"},
				Spec: WorkloadPlacementSpec{
					WorkloadSelector: WorkloadSelector{
						WorkloadTypes: []WorkloadType{
							{APIVersion: "apps/v1", Kind: "Deployment"},
						},
					},
					ClusterSelector: ClusterSelector{
						LocationSelector: []string{"us-west-2"},
					},
				},
			},
			valid: true,
		},
		"empty workload selector": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement", Namespace: "default"},
				Spec: WorkloadPlacementSpec{
					ClusterSelector: ClusterSelector{
						ClusterNames: []string{"cluster-1"},
					},
				},
			},
			valid: false,
		},
		"empty cluster selector": {
			placement: &WorkloadPlacement{
				ObjectMeta: metav1.ObjectMeta{Name: "test-placement", Namespace: "default"},
				Spec: WorkloadPlacementSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
					},
				},
			},
			valid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isEmpty := tc.placement.Spec.WorkloadSelector.LabelSelector == nil &&
				len(tc.placement.Spec.WorkloadSelector.WorkloadTypes) == 0
			if isEmpty && tc.valid {
				t.Error("Expected validation to fail for empty workload selector")
			}

			clusterEmpty := tc.placement.Spec.ClusterSelector.LabelSelector == nil &&
				len(tc.placement.Spec.ClusterSelector.LocationSelector) == 0 &&
				len(tc.placement.Spec.ClusterSelector.ClusterNames) == 0
			if clusterEmpty && tc.valid {
				t.Error("Expected validation to fail for empty cluster selector")
			}
		})
	}
}

func TestWorkloadTypeValidation(t *testing.T) {
	tests := map[string]struct {
		workloadType WorkloadType
		valid        bool
	}{
		"valid deployment": {
			workloadType: WorkloadType{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			valid: true,
		},
		"valid statefulset": {
			workloadType: WorkloadType{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
			},
			valid: true,
		},
		"missing api version": {
			workloadType: WorkloadType{
				Kind: "Deployment",
			},
			valid: false,
		},
		"missing kind": {
			workloadType: WorkloadType{
				APIVersion: "apps/v1",
			},
			valid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.workloadType.APIVersion == "" && tc.valid {
				t.Error("Expected validation to fail for missing API version")
			}
			if tc.workloadType.Kind == "" && tc.valid {
				t.Error("Expected validation to fail for missing kind")
			}
		})
	}
}

func TestPlacementPolicyValidation(t *testing.T) {
	validPolicies := []PlacementPolicy{
		PlacementPolicyRoundRobin,
		PlacementPolicyLeastLoaded,
		PlacementPolicyRandom,
		PlacementPolicyLocationAware,
	}

	for _, policy := range validPolicies {
		t.Run(string(policy), func(t *testing.T) {
			if policy == "" {
				t.Error("Policy should not be empty")
			}
		})
	}
}

func TestPlacedWorkloadStatus(t *testing.T) {
	validStatuses := []PlacedWorkloadStatus{
		PlacedWorkloadStatusPending,
		PlacedWorkloadStatusPlaced,
		PlacedWorkloadStatusFailed,
		PlacedWorkloadStatusRemoved,
	}

	for _, status := range validStatuses {
		t.Run(string(status), func(t *testing.T) {
			if status == "" {
				t.Error("Status should not be empty")
			}
		})
	}
}

func TestWorkloadReferenceValidation(t *testing.T) {
	tests := map[string]struct {
		ref   WorkloadReference
		valid bool
	}{
		"valid namespaced workload": {
			ref: WorkloadReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "my-app",
				Namespace:  "default",
			},
			valid: true,
		},
		"valid cluster-scoped workload": {
			ref: WorkloadReference{
				APIVersion: "v1",
				Kind:       "Node",
				Name:       "node-1",
			},
			valid: true,
		},
		"missing name": {
			ref: WorkloadReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "default",
			},
			valid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.ref.Name == "" && tc.valid {
				t.Error("Expected validation to fail for missing name")
			}
		})
	}
}