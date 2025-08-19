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

package storage

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpdynamic "github.com/kcp-dev/client-go/dynamic"
)

// TestNewTMCVirtualWorkspaceStorage tests the creation of TMC virtual workspace storage.
func TestNewTMCVirtualWorkspaceStorage(t *testing.T) {
	tests := map[string]struct {
		config     TMCStorageConfig
		expectNil  bool
	}{
		"valid config": {
			config: TMCStorageConfig{
				DynamicClient: nil, // Mock client would go here
				GroupVersionResource: schema.GroupVersionResource{
					Group:    "tmc.kcp.io",
					Version:  "v1alpha1",
					Resource: "clusterregistrations",
				},
				Workspace:    logicalcluster.Name("test-workspace"),
				IsNamespaced: false,
			},
			expectNil: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := NewTMCVirtualWorkspaceStorage(tc.config)

			if tc.expectNil {
				if storage != nil {
					t.Errorf("Expected nil storage but got non-nil")
				}
			} else {
				if storage == nil {
					t.Errorf("Expected non-nil storage but got nil")
				}
				if storage.gvr != tc.config.GroupVersionResource {
					t.Errorf("Expected GVR %v but got %v", tc.config.GroupVersionResource, storage.gvr)
				}
				if storage.workspace != tc.config.Workspace {
					t.Errorf("Expected workspace %v but got %v", tc.config.Workspace, storage.workspace)
				}
				if storage.isNamespaced != tc.config.IsNamespaced {
					t.Errorf("Expected isNamespaced %v but got %v", tc.config.IsNamespaced, storage.isNamespaced)
				}
			}
		})
	}
}

// TestGetKind tests the kind name generation.
func TestGetKind(t *testing.T) {
	tests := map[string]struct {
		resource     string
		expectedKind string
	}{
		"clusterregistrations": {
			resource:     "clusterregistrations",
			expectedKind: "ClusterRegistration",
		},
		"workloadplacements": {
			resource:     "workloadplacements", 
			expectedKind: "WorkloadPlacement",
		},
		"syncerconfigs": {
			resource:     "syncerconfigs",
			expectedKind: "SyncerConfig",
		},
		"empty resource": {
			resource:     "",
			expectedKind: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := &TMCVirtualWorkspaceStorage{
				gvr: schema.GroupVersionResource{
					Resource: tc.resource,
				},
			}

			kind := storage.getKind()
			if kind != tc.expectedKind {
				t.Errorf("Expected kind %s but got %s", tc.expectedKind, kind)
			}
		})
	}
}

// TestNewObject tests the New method.
func TestNewObject(t *testing.T) {
	storage := &TMCVirtualWorkspaceStorage{
		gvr: schema.GroupVersionResource{
			Group:    "tmc.kcp.io",
			Version:  "v1alpha1",
			Resource: "clusterregistrations",
		},
	}

	obj := storage.New()
	if obj == nil {
		t.Errorf("Expected non-nil object but got nil")
	}
}