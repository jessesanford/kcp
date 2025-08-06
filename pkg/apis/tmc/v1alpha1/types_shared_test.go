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

func TestWorkloadSelectorValidation(t *testing.T) {
	tests := map[string]struct {
		selector  WorkloadSelector
		wantValid bool
	}{
		"valid selector with label selector": {
			selector: WorkloadSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "web"},
				},
			},
			wantValid: true,
		},
		"valid selector with workload types": {
			selector: WorkloadSelector{
				WorkloadTypes: []WorkloadType{
					{APIVersion: "apps/v1", Kind: "Deployment"},
					{APIVersion: "apps/v1", Kind: "StatefulSet"},
				},
			},
			wantValid: true,
		},
		"valid selector with namespace selector": {
			selector: WorkloadSelector{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"tier": "production"},
				},
			},
			wantValid: true,
		},
		"valid selector with multiple criteria": {
			selector: WorkloadSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "web"},
				},
				WorkloadTypes: []WorkloadType{
					{APIVersion: "apps/v1", Kind: "Deployment"},
				},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"tier": "production"},
				},
			},
			wantValid: true,
		},
		"empty selector": {
			selector:  WorkloadSelector{},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isValid := isValidWorkloadSelector(tc.selector)
			if isValid != tc.wantValid {
				t.Errorf("expected validity %v, got %v", tc.wantValid, isValid)
			}
		})
	}
}

func TestClusterSelectorValidation(t *testing.T) {
	tests := map[string]struct {
		selector  ClusterSelector
		wantValid bool
	}{
		"valid selector with cluster names": {
			selector: ClusterSelector{
				ClusterNames: []string{"cluster-1", "cluster-2"},
			},
			wantValid: true,
		},
		"valid selector with label selector": {
			selector: ClusterSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"tier": "production"},
				},
			},
			wantValid: true,
		},
		"valid selector with location selector": {
			selector: ClusterSelector{
				LocationSelector: []string{"us-west-1", "us-east-1"},
			},
			wantValid: true,
		},
		"valid selector with multiple criteria": {
			selector: ClusterSelector{
				ClusterNames: []string{"cluster-1"},
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"tier": "production"},
				},
				LocationSelector: []string{"us-west-1"},
			},
			wantValid: true,
		},
		"empty selector": {
			selector:  ClusterSelector{},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isValid := isValidClusterSelector(tc.selector)
			if isValid != tc.wantValid {
				t.Errorf("expected validity %v, got %v", tc.wantValid, isValid)
			}
		})
	}
}

func TestWorkloadTypeValidation(t *testing.T) {
	tests := map[string]struct {
		workloadType WorkloadType
		wantValid    bool
	}{
		"valid deployment": {
			workloadType: WorkloadType{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			wantValid: true,
		},
		"valid statefulset": {
			workloadType: WorkloadType{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
			},
			wantValid: true,
		},
		"valid custom resource": {
			workloadType: WorkloadType{
				APIVersion: "custom.io/v1",
				Kind:       "CustomWorkload",
			},
			wantValid: true,
		},
		"missing api version": {
			workloadType: WorkloadType{
				Kind: "Deployment",
			},
			wantValid: false,
		},
		"missing kind": {
			workloadType: WorkloadType{
				APIVersion: "apps/v1",
			},
			wantValid: false,
		},
		"empty workload type": {
			workloadType: WorkloadType{},
			wantValid:    false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			isValid := isValidWorkloadType(tc.workloadType)
			if isValid != tc.wantValid {
				t.Errorf("expected validity %v, got %v", tc.wantValid, isValid)
			}
		})
	}
}

func TestWorkloadHealthStatusValues(t *testing.T) {
	validStatuses := []WorkloadHealthStatus{
		WorkloadHealthStatusHealthy,
		WorkloadHealthStatusUnhealthy,
		WorkloadHealthStatusDegraded,
		WorkloadHealthStatusUnknown,
		WorkloadHealthStatusChecking,
	}

	expectedValues := []string{
		"Healthy",
		"Unhealthy",
		"Degraded",
		"Unknown",
		"Checking",
	}

	if len(validStatuses) != len(expectedValues) {
		t.Fatalf("expected %d status values, got %d", len(expectedValues), len(validStatuses))
	}

	for i, status := range validStatuses {
		if string(status) != expectedValues[i] {
			t.Errorf("expected status %s, got %s", expectedValues[i], string(status))
		}
	}
}

// Helper validation functions (would be in separate validation file in real implementation)

func isValidWorkloadSelector(selector WorkloadSelector) bool {
	return selector.LabelSelector != nil ||
		len(selector.WorkloadTypes) > 0 ||
		selector.NamespaceSelector != nil
}

func isValidClusterSelector(selector ClusterSelector) bool {
	return len(selector.ClusterNames) > 0 ||
		selector.LabelSelector != nil ||
		len(selector.LocationSelector) > 0
}

func isValidWorkloadType(workloadType WorkloadType) bool {
	return workloadType.APIVersion != "" && workloadType.Kind != ""
}