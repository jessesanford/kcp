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

func TestWorkloadPlacementDefaults(t *testing.T) {
	placement := &WorkloadPlacement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-placement",
			Namespace: "default",
		},
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
		},
	}

	// Verify the object is well-formed
	if placement.Spec.PlacementPolicy != PlacementPolicyRoundRobin {
		t.Errorf("Expected placement policy RoundRobin, got %s", placement.Spec.PlacementPolicy)
	}

	if len(placement.Spec.WorkloadSelector.WorkloadTypes) != 1 {
		t.Errorf("Expected 1 workload type, got %d", len(placement.Spec.WorkloadSelector.WorkloadTypes))
	}

	workloadType := placement.Spec.WorkloadSelector.WorkloadTypes[0]
	if workloadType.APIVersion != "apps/v1" {
		t.Errorf("Expected APIVersion apps/v1, got %s", workloadType.APIVersion)
	}

	if workloadType.Kind != "Deployment" {
		t.Errorf("Expected Kind Deployment, got %s", workloadType.Kind)
	}
}

func TestWorkloadPlacementStatus(t *testing.T) {
	now := metav1.Now()
	placement := &WorkloadPlacement{
		Status: WorkloadPlacementStatus{
			LastPlacementTime: &now,
			SelectedClusters:  []string{"cluster-1", "cluster-2"},
			PlacedWorkloads: []PlacedWorkload{
				{
					WorkloadRef: WorkloadReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "test-app",
						Namespace:  "default",
					},
					ClusterName:   "cluster-1",
					PlacementTime: now,
					Status:        PlacedWorkloadStatusPlaced,
				},
			},
			Conditions: conditionsv1alpha1.Conditions{
				{
					Type:   "Ready",
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	// Verify status fields
	if placement.Status.LastPlacementTime == nil {
		t.Error("Expected LastPlacementTime to be set")
	}

	if len(placement.Status.SelectedClusters) != 2 {
		t.Errorf("Expected 2 selected clusters, got %d", len(placement.Status.SelectedClusters))
	}

	if len(placement.Status.PlacedWorkloads) != 1 {
		t.Errorf("Expected 1 placed workload, got %d", len(placement.Status.PlacedWorkloads))
	}

	placedWorkload := placement.Status.PlacedWorkloads[0]
	if placedWorkload.Status != PlacedWorkloadStatusPlaced {
		t.Errorf("Expected status Placed, got %s", placedWorkload.Status)
	}

	if placedWorkload.ClusterName != "cluster-1" {
		t.Errorf("Expected cluster name cluster-1, got %s", placedWorkload.ClusterName)
	}
}

func TestPlacementDecision(t *testing.T) {
	now := metav1.Now()
	decision := PlacementDecision{
		ClusterName:   "cluster-1",
		Reason:        "Least loaded cluster",
		Score:         int32Ptr(95),
		DecisionTime:  now,
	}

	if decision.ClusterName != "cluster-1" {
		t.Errorf("Expected cluster name cluster-1, got %s", decision.ClusterName)
	}

	if decision.Reason != "Least loaded cluster" {
		t.Errorf("Expected reason 'Least loaded cluster', got %s", decision.Reason)
	}

	if *decision.Score != 95 {
		t.Errorf("Expected score 95, got %d", *decision.Score)
	}
}

func TestPlacementPolicies(t *testing.T) {
	policies := []PlacementPolicy{
		PlacementPolicyRoundRobin,
		PlacementPolicyLeastLoaded,
		PlacementPolicyRandom,
		PlacementPolicyLocationAware,
	}

	expectedPolicies := []string{
		"RoundRobin",
		"LeastLoaded",
		"Random",
		"LocationAware",
	}

	for i, policy := range policies {
		if string(policy) != expectedPolicies[i] {
			t.Errorf("Expected policy %s, got %s", expectedPolicies[i], string(policy))
		}
	}
}

func TestPlacedWorkloadStatuses(t *testing.T) {
	statuses := []PlacedWorkloadStatus{
		PlacedWorkloadStatusPending,
		PlacedWorkloadStatusPlaced,
		PlacedWorkloadStatusFailed,
		PlacedWorkloadStatusRemoved,
	}

	expectedStatuses := []string{
		"Pending",
		"Placed",
		"Failed",
		"Removed",
	}

	for i, status := range statuses {
		if string(status) != expectedStatuses[i] {
			t.Errorf("Expected status %s, got %s", expectedStatuses[i], string(status))
		}
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}