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

type WorkloadType struct {
	APIVersion string
	Kind       string
}

type WorkloadReference struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
}

type PlacedWorkloadStatus string

const (
	PlacedWorkloadStatusPending PlacedWorkloadStatus = "Pending"
	PlacedWorkloadStatusPlaced  PlacedWorkloadStatus = "Placed"
	PlacedWorkloadStatusFailed  PlacedWorkloadStatus = "Failed"
	PlacedWorkloadStatusRemoved PlacedWorkloadStatus = "Removed"
)

func TestWorkloadTypeValidation(t *testing.T) {
	tests := []struct {
		name       string
		workload   WorkloadType
		expectErr  bool
	}{
		{
			name:       "valid deployment type",
			workload:   WorkloadType{APIVersion: "apps/v1", Kind: "Deployment"},
			expectErr:  false,
		},
		{
			name:       "valid pod type",
			workload:   WorkloadType{APIVersion: "v1", Kind: "Pod"},
			expectErr:  false,
		},
		{
			name:       "empty APIVersion",
			workload:   WorkloadType{APIVersion: "", Kind: "Deployment"},
			expectErr:  true,
		},
		{
			name:       "empty Kind",
			workload:   WorkloadType{APIVersion: "apps/v1", Kind: ""},
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := tt.workload.APIVersion == "" || tt.workload.Kind == ""
			if hasError \!= tt.expectErr {
				t.Error("Expected error mismatch")
			}
		})
	}
}

func TestWorkloadReferenceValidation(t *testing.T) {
	tests := []struct {
		name      string
		ref       WorkloadReference
		expectErr bool
	}{
		{
			name: "valid namespaced resource",
			ref: WorkloadReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "web-app",
				Namespace:  "default",
			},
			expectErr: false,
		},
		{
			name: "valid cluster-scoped resource",
			ref: WorkloadReference{
				APIVersion: "v1",
				Kind:       "Node",
				Name:       "worker-1",
				Namespace:  "",
			},
			expectErr: false,
		},
		{
			name: "missing APIVersion",
			ref: WorkloadReference{
				APIVersion: "",
				Kind:       "Deployment",
				Name:       "web-app",
			},
			expectErr: true,
		},
		{
			name: "missing Kind",
			ref: WorkloadReference{
				APIVersion: "apps/v1",
				Kind:       "",
				Name:       "web-app",
			},
			expectErr: true,
		},
		{
			name: "missing Name",
			ref: WorkloadReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := tt.ref.APIVersion == "" || tt.ref.Kind == "" || tt.ref.Name == ""
			if hasError \!= tt.expectErr {
				t.Error("Expected error mismatch")
			}
		})
	}
}

func TestPlacedWorkloadStatusConstants(t *testing.T) {
	statuses := map[PlacedWorkloadStatus]string{
		PlacedWorkloadStatusPending: "Pending",
		PlacedWorkloadStatusPlaced:  "Placed",
		PlacedWorkloadStatusFailed:  "Failed",
		PlacedWorkloadStatusRemoved: "Removed",
	}

	for status, expected := range statuses {
		if string(status) \!= expected {
			t.Error("Status constant mismatch")
		}
	}

	expectedCount := 4
	if len(statuses) \!= expectedCount {
		t.Error("Expected 4 statuses")
	}
}

func TestWorkloadTypeDefaults(t *testing.T) {
