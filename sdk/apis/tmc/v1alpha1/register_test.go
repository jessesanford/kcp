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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSchemeGroupVersion(t *testing.T) {
	expected := schema.GroupVersion{Group: "tmc.kcp.io", Version: "v1alpha1"}
	if SchemeGroupVersion != expected {
		t.Errorf("Expected SchemeGroupVersion %v, got %v", expected, SchemeGroupVersion)
	}
}

func TestGroupName(t *testing.T) {
	expected := "tmc.kcp.io"
	if GroupName != expected {
		t.Errorf("Expected GroupName %s, got %s", expected, GroupName)
	}
}

func TestResource(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		expected schema.GroupResource
	}{
		{
			name:     "cluster registrations",
			resource: "clusterregistrations",
			expected: schema.GroupResource{Group: "tmc.kcp.io", Resource: "clusterregistrations"},
		},
		{
			name:     "workload placements",
			resource: "workloadplacements",
			expected: schema.GroupResource{Group: "tmc.kcp.io", Resource: "workloadplacements"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Resource(tt.resource)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAddToScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	if err != nil {
		t.Fatalf("AddToScheme failed: %v", err)
	}

	// Verify that the types are registered
	gvk := schema.GroupVersionKind{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "ClusterRegistration"}
	if !scheme.Recognizes(gvk) {
		t.Errorf("Scheme does not recognize ClusterRegistration")
	}

	gvk = schema.GroupVersionKind{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacement"}
	if !scheme.Recognizes(gvk) {
		t.Errorf("Scheme does not recognize WorkloadPlacement")
	}

	gvk = schema.GroupVersionKind{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "ClusterRegistrationList"}
	if !scheme.Recognizes(gvk) {
		t.Errorf("Scheme does not recognize ClusterRegistrationList")
	}

	gvk = schema.GroupVersionKind{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacementList"}
	if !scheme.Recognizes(gvk) {
		t.Errorf("Scheme does not recognize WorkloadPlacementList")
	}
}

func TestAddKnownTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	err := addKnownTypes(scheme)
	if err != nil {
		t.Fatalf("addKnownTypes failed: %v", err)
	}

	// Test that all expected types are registered
	expectedTypes := []schema.GroupVersionKind{
		{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "ClusterRegistration"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "ClusterRegistrationList"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacement"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacementList"},
	}

	for _, gvk := range expectedTypes {
		if !scheme.Recognizes(gvk) {
			t.Errorf("Scheme does not recognize %v", gvk)
		}
	}
}
