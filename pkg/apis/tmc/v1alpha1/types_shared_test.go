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

func TestWorkloadSelector(t *testing.T) {
	selector := WorkloadSelector{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "test",
			},
		},
		WorkloadTypes: []WorkloadType{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
			},
		},
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"environment": "production",
			},
		},
	}

	// Verify label selector
	if selector.LabelSelector == nil {
		t.Error("Expected LabelSelector to be set")
	}

	if len(selector.LabelSelector.MatchLabels) != 1 {
		t.Errorf("Expected 1 match label, got %d", len(selector.LabelSelector.MatchLabels))
	}

	if selector.LabelSelector.MatchLabels["app"] != "test" {
		t.Errorf("Expected app=test, got app=%s", selector.LabelSelector.MatchLabels["app"])
	}

	// Verify workload types
	if len(selector.WorkloadTypes) != 2 {
		t.Errorf("Expected 2 workload types, got %d", len(selector.WorkloadTypes))
	}

	deployment := selector.WorkloadTypes[0]
	if deployment.APIVersion != "apps/v1" || deployment.Kind != "Deployment" {
		t.Errorf("Expected apps/v1 Deployment, got %s %s", deployment.APIVersion, deployment.Kind)
	}

	statefulset := selector.WorkloadTypes[1]
	if statefulset.APIVersion != "apps/v1" || statefulset.Kind != "StatefulSet" {
		t.Errorf("Expected apps/v1 StatefulSet, got %s %s", statefulset.APIVersion, statefulset.Kind)
	}

	// Verify namespace selector
	if selector.NamespaceSelector == nil {
		t.Error("Expected NamespaceSelector to be set")
	}

	if selector.NamespaceSelector.MatchLabels["environment"] != "production" {
		t.Errorf("Expected environment=production, got environment=%s", selector.NamespaceSelector.MatchLabels["environment"])
	}
}

func TestClusterSelector(t *testing.T) {
	selector := ClusterSelector{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"zone": "us-west-2a",
			},
		},
		LocationSelector: []string{"us-west-2", "us-east-1"},
		ClusterNames:     []string{"cluster-prod-1", "cluster-prod-2"},
	}

	// Verify label selector
	if selector.LabelSelector == nil {
		t.Error("Expected LabelSelector to be set")
	}

	if selector.LabelSelector.MatchLabels["zone"] != "us-west-2a" {
		t.Errorf("Expected zone=us-west-2a, got zone=%s", selector.LabelSelector.MatchLabels["zone"])
	}

	// Verify location selector
	if len(selector.LocationSelector) != 2 {
		t.Errorf("Expected 2 locations, got %d", len(selector.LocationSelector))
	}

	expectedLocations := []string{"us-west-2", "us-east-1"}
	for i, location := range selector.LocationSelector {
		if location != expectedLocations[i] {
			t.Errorf("Expected location %s, got %s", expectedLocations[i], location)
		}
	}

	// Verify cluster names
	if len(selector.ClusterNames) != 2 {
		t.Errorf("Expected 2 cluster names, got %d", len(selector.ClusterNames))
	}

	expectedNames := []string{"cluster-prod-1", "cluster-prod-2"}
	for i, name := range selector.ClusterNames {
		if name != expectedNames[i] {
			t.Errorf("Expected cluster name %s, got %s", expectedNames[i], name)
		}
	}
}

func TestWorkloadReference(t *testing.T) {
	ref := WorkloadReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "test-deployment",
		Namespace:  "default",
	}

	if ref.APIVersion != "apps/v1" {
		t.Errorf("Expected APIVersion apps/v1, got %s", ref.APIVersion)
	}

	if ref.Kind != "Deployment" {
		t.Errorf("Expected Kind Deployment, got %s", ref.Kind)
	}

	if ref.Name != "test-deployment" {
		t.Errorf("Expected Name test-deployment, got %s", ref.Name)
	}

	if ref.Namespace != "default" {
		t.Errorf("Expected Namespace default, got %s", ref.Namespace)
	}
}

func TestWorkloadType(t *testing.T) {
	workloadType := WorkloadType{
		APIVersion: "batch/v1",
		Kind:       "Job",
	}

	if workloadType.APIVersion != "batch/v1" {
		t.Errorf("Expected APIVersion batch/v1, got %s", workloadType.APIVersion)
	}

	if workloadType.Kind != "Job" {
		t.Errorf("Expected Kind Job, got %s", workloadType.Kind)
	}
}
