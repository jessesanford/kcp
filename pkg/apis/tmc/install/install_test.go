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

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

func TestInstall(t *testing.T) {
	scheme := runtime.NewScheme()
	Install(scheme)

	// Verify ClusterRegistration is registered
	gvks, _, err := scheme.ObjectKinds(&tmcv1alpha1.ClusterRegistration{})
	if err != nil {
		t.Fatalf("Failed to get ObjectKinds: %v", err)
	}
	if len(gvks) == 0 {
		t.Error("ClusterRegistration should be registered")
	}

	expectedGVK := tmcv1alpha1.SchemeGroupVersion.WithKind("ClusterRegistration")
	found := false
	for _, gvk := range gvks {
		if gvk == expectedGVK {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected GVK %v not found in registered types", expectedGVK)
	}
}