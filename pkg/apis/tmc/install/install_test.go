/*
Copyright 2025 The KCP Authors.

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

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

func TestInstall(t *testing.T) {
	scheme := runtime.NewScheme()
	
	// Install should not panic or return errors
	Install(scheme)

	// Verify that TMC types are registered in the scheme
	gvk := tmcv1alpha1.SchemeGroupVersion.WithKind("ClusterRegistration")
	obj, err := scheme.New(gvk)
	if err != nil {
		t.Errorf("failed to create ClusterRegistration from scheme: %v", err)
	}
	if _, ok := obj.(*tmcv1alpha1.ClusterRegistration); !ok {
		t.Errorf("expected *ClusterRegistration, got %T", obj)
	}

	gvk = tmcv1alpha1.SchemeGroupVersion.WithKind("WorkloadPlacement")
	obj, err = scheme.New(gvk)
	if err != nil {
		t.Errorf("failed to create WorkloadPlacement from scheme: %v", err)
	}
	if _, ok := obj.(*tmcv1alpha1.WorkloadPlacement); !ok {
		t.Errorf("expected *WorkloadPlacement, got %T", obj)
	}

	// Verify list types are also registered
	gvk = tmcv1alpha1.SchemeGroupVersion.WithKind("ClusterRegistrationList")
	obj, err = scheme.New(gvk)
	if err != nil {
		t.Errorf("failed to create ClusterRegistrationList from scheme: %v", err)
	}
	if _, ok := obj.(*tmcv1alpha1.ClusterRegistrationList); !ok {
		t.Errorf("expected *ClusterRegistrationList, got %T", obj)
	}

	gvk = tmcv1alpha1.SchemeGroupVersion.WithKind("WorkloadPlacementList")
	obj, err = scheme.New(gvk)
	if err != nil {
		t.Errorf("failed to create WorkloadPlacementList from scheme: %v", err)
	}
	if _, ok := obj.(*tmcv1alpha1.WorkloadPlacementList); !ok {
		t.Errorf("expected *WorkloadPlacementList, got %T", obj)
	}
}