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

package install

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestInstall(t *testing.T) {
	scheme := runtime.NewScheme()
	Install(scheme)

	// Verify that TMC API types are registered
	expectedTypes := []schema.GroupVersionKind{
		{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "ClusterRegistration"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "ClusterRegistrationList"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacement"},
		{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacementList"},
	}

	for _, gvk := range expectedTypes {
		if !scheme.Recognizes(gvk) {
			t.Errorf("Scheme does not recognize %v after Install()", gvk)
		}
	}

	// Verify that we can create objects of the registered types
	obj, err := scheme.New(schema.GroupVersionKind{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "ClusterRegistration"})
	if err != nil {
		t.Errorf("Failed to create ClusterRegistration object: %v", err)
	}
	if obj == nil {
		t.Errorf("Created ClusterRegistration object is nil")
	}

	obj, err = scheme.New(schema.GroupVersionKind{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "WorkloadPlacement"})
	if err != nil {
		t.Errorf("Failed to create WorkloadPlacement object: %v", err)
	}
	if obj == nil {
		t.Errorf("Created WorkloadPlacement object is nil")
	}
}

func TestInstallMultipleTimes(t *testing.T) {
	scheme := runtime.NewScheme()

	// Install multiple times - should not cause issues
	Install(scheme)
	Install(scheme)
	Install(scheme)

	// Verify types are still registered correctly
	gvk := schema.GroupVersionKind{Group: "tmc.kcp.io", Version: "v1alpha1", Kind: "ClusterRegistration"}
	if !scheme.Recognizes(gvk) {
		t.Errorf("Scheme does not recognize ClusterRegistration after multiple Install() calls")
	}
}

func TestInstallWithNilScheme(t *testing.T) {
	// This should not panic
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected Install(nil) to panic, but it didn't")
		}
	}()

	Install(nil)
}
