// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	corev1alpha1 "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v3"
)

// TestWorkspaceIsolationBaseController tests the security boundaries enforced
// by the base controller to prevent cross-tenant data access.
func TestWorkspaceIsolationBaseController(t *testing.T) {
	tests := map[string]struct {
		workspaceRoot     string
		allowedWorkspaces []string
		testWorkspace     string
		expectAccess      bool
		description       string
	}{
		"access to root workspace allowed": {
			workspaceRoot:     "root:default",
			allowedWorkspaces: nil,
			testWorkspace:     "root:default",
			expectAccess:      true,
			description:       "Controller should allow access to its root workspace",
		},
		"access to explicitly allowed workspace": {
			workspaceRoot:     "root:default",
			allowedWorkspaces: []string{"root:tenant-a", "root:tenant-b"},
			testWorkspace:     "root:tenant-a",
			expectAccess:      true,
			description:       "Controller should allow access to explicitly allowed workspaces",
		},
		"access denied to unauthorized workspace": {
			workspaceRoot:     "root:default",
			allowedWorkspaces: []string{"root:tenant-a"},
			testWorkspace:     "root:tenant-b",
			expectAccess:      false,
			description:       "Controller should deny access to unauthorized workspaces",
		},
		"access denied to empty workspace": {
			workspaceRoot:     "root:default",
			allowedWorkspaces: nil,
			testWorkspace:     "",
			expectAccess:      false,
			description:       "Controller should deny access to empty workspace strings",
		},
		"access denied to subtenant without permission": {
			workspaceRoot:     "root:default",
			allowedWorkspaces: nil,
			testWorkspace:     "root:default:subtenant",
			expectAccess:      false,
			description:       "Controller should deny access to subtenants without explicit permission",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create base controller configuration
			config := &BaseControllerConfig{
				Name:            "test-controller",
				ResyncPeriod:    time.Minute,
				WorkerCount:     1,
				Metrics:         NewManagerMetrics(),
				InformerFactory: nil, // Not needed for this test
				WorkspaceRoot:   logicalcluster.Name(tc.workspaceRoot),
				AllowedWorkspaces: func() []logicalcluster.Name {
					var allowed []logicalcluster.Name
					for _, ws := range tc.allowedWorkspaces {
						allowed = append(allowed, logicalcluster.Name(ws))
					}
					return allowed
				}(),
			}

			controller := NewBaseController(config)
			baseImpl := controller.(*baseControllerImpl)

			// Test workspace validation
			testWorkspace := logicalcluster.Name(tc.testWorkspace)
			err := baseImpl.ValidateWorkspaceAccess(testWorkspace)

			if tc.expectAccess && err != nil {
				t.Errorf("Expected access to workspace %s but got error: %v", tc.testWorkspace, err)
			}

			if !tc.expectAccess && err == nil {
				t.Errorf("Expected denial of access to workspace %s but access was granted", tc.testWorkspace)
			}

			// Test IsWorkspaceAllowed method
			allowed := baseImpl.IsWorkspaceAllowed(testWorkspace)
			if tc.expectAccess != allowed {
				t.Errorf("IsWorkspaceAllowed returned %v, expected %v for workspace %s", 
					allowed, tc.expectAccess, tc.testWorkspace)
			}
		})
	}
}

// TestWorkspaceKeyExtraction tests the security of key extraction and validation.
func TestWorkspaceKeyExtraction(t *testing.T) {
	tests := map[string]struct {
		workspaceRoot   string
		inputKey        string
		expectWorkspace string
		expectResource  string
		expectError     bool
		description     string
	}{
		"valid cluster-aware key": {
			workspaceRoot:   "root:default",
			inputKey:        "root:default|test-namespace/test-resource",
			expectWorkspace: "root:default",
			expectResource:  "test-namespace/test-resource",
			expectError:     false,
			description:     "Should successfully parse valid cluster-aware keys",
		},
		"valid cluster-scoped key": {
			workspaceRoot:   "root:default",
			inputKey:        "root:default|cluster-resource",
			expectWorkspace: "root:default",
			expectResource:  "cluster-resource",
			expectError:     false,
			description:     "Should successfully parse cluster-scoped resource keys",
		},
		"invalid key format": {
			workspaceRoot: "root:default",
			inputKey:      "invalid-key-format",
			expectError:   true,
			description:   "Should reject keys without cluster delimiter",
		},
		"unauthorized workspace in key": {
			workspaceRoot: "root:default",
			inputKey:      "root:unauthorized|resource",
			expectError:   true,
			description:   "Should reject keys with unauthorized workspaces",
		},
		"empty key": {
			workspaceRoot: "root:default",
			inputKey:      "",
			expectError:   true,
			description:   "Should reject empty keys",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create base controller
			config := &BaseControllerConfig{
				Name:              "test-controller",
				ResyncPeriod:      time.Minute,
				WorkerCount:       1,
				Metrics:           NewManagerMetrics(),
				InformerFactory:   nil,
				WorkspaceRoot:     logicalcluster.Name(tc.workspaceRoot),
				AllowedWorkspaces: nil,
			}

			controller := NewBaseController(config)
			baseImpl := controller.(*baseControllerImpl)

			// Test key extraction
			workspace, resource, err := baseImpl.ExtractWorkspaceFromKey(tc.inputKey)

			if tc.expectError && err == nil {
				t.Errorf("Expected error for key %s but extraction succeeded", tc.inputKey)
			}

			if !tc.expectError {
				if err != nil {
					t.Errorf("Unexpected error for key %s: %v", tc.inputKey, err)
				} else {
					if workspace.String() != tc.expectWorkspace {
						t.Errorf("Expected workspace %s, got %s", tc.expectWorkspace, workspace)
					}
					if resource != tc.expectResource {
						t.Errorf("Expected resource %s, got %s", tc.expectResource, resource)
					}
				}
			}
		})
	}
}

// TestManagerWorkspaceIsolation tests workspace isolation at the manager level.
func TestManagerWorkspaceIsolation(t *testing.T) {
	tests := map[string]struct {
		managerWorkspace string
		testWorkspace    string
		expectAccess     bool
		description      string
	}{
		"access to manager workspace allowed": {
			managerWorkspace: "root:manager-workspace",
			testWorkspace:    "root:manager-workspace",
			expectAccess:     true,
			description:      "Manager should allow access to its own workspace",
		},
		"access denied to different workspace": {
			managerWorkspace: "root:manager-workspace",
			testWorkspace:    "root:other-workspace",
			expectAccess:     false,
			description:      "Manager should deny access to other workspaces",
		},
		"access denied to empty workspace": {
			managerWorkspace: "root:manager-workspace",
			testWorkspace:    "",
			expectAccess:     false,
			description:      "Manager should deny access to empty workspace",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create mock config
			config := &Config{
				KCPConfig: &rest.Config{
					Host: "https://mock-kcp-server",
				},
				Workspace:       tc.managerWorkspace,
				ClusterConfigs:  make(map[string]*rest.Config),
				ResyncPeriod:    time.Minute,
				WorkerCount:     1,
				MetricsPort:     0, // Disable for test
				HealthPort:      0, // Disable for test
			}

			// We can't actually create the manager without a real KCP connection,
			// so we'll test the validation logic directly
			workspace := logicalcluster.Name(tc.managerWorkspace)
			testWorkspace := logicalcluster.Name(tc.testWorkspace)

			// Create a mock manager to test validation logic
			manager := &Manager{
				workspace: workspace,
			}

			err := manager.ValidateWorkspaceAccess(testWorkspace)

			if tc.expectAccess && err != nil {
				t.Errorf("Expected access to workspace %s but got error: %v", tc.testWorkspace, err)
			}

			if !tc.expectAccess && err == nil {
				t.Errorf("Expected denial of access to workspace %s but access was granted", tc.testWorkspace)
			}
		})
	}
}

// TestObjectWorkspaceValidation tests validation of runtime objects for workspace isolation.
func TestObjectWorkspaceValidation(t *testing.T) {
	tests := map[string]struct {
		workspaceRoot   string
		objectWorkspace string
		expectAccess    bool
		description     string
	}{
		"object from allowed workspace": {
			workspaceRoot:   "root:default",
			objectWorkspace: "root:default",
			expectAccess:    true,
			description:     "Should allow objects from the allowed workspace",
		},
		"object from unauthorized workspace": {
			workspaceRoot:   "root:default",
			objectWorkspace: "root:unauthorized",
			expectAccess:    false,
			description:     "Should reject objects from unauthorized workspaces",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create base controller
			config := &BaseControllerConfig{
				Name:              "test-controller",
				ResyncPeriod:      time.Minute,
				WorkerCount:       1,
				Metrics:           NewManagerMetrics(),
				InformerFactory:   nil,
				WorkspaceRoot:     logicalcluster.Name(tc.workspaceRoot),
				AllowedWorkspaces: nil,
			}

			controller := NewBaseController(config)
			baseImpl := controller.(*baseControllerImpl)

			// Create a mock object with logical cluster annotation
			obj := &corev1alpha1.LogicalCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-logical-cluster",
					Annotations: map[string]string{
						"kcp.io/logical-cluster": tc.objectWorkspace,
					},
				},
			}

			// Test object validation
			err := baseImpl.ValidateObjectWorkspace(obj)

			if tc.expectAccess && err != nil {
				t.Errorf("Expected access for object from workspace %s but got error: %v", tc.objectWorkspace, err)
			}

			if !tc.expectAccess && err == nil {
				t.Errorf("Expected denial for object from workspace %s but access was granted", tc.objectWorkspace)
			}
		})
	}
}

// TestWorkspaceIsolationPanicScenarios tests that the controller properly handles
// configuration errors that could lead to security vulnerabilities.
func TestWorkspaceIsolationPanicScenarios(t *testing.T) {
	t.Run("panic on nil config", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for nil config")
			}
		}()
		NewBaseController(nil)
	})

	t.Run("panic on empty workspace root", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic for empty workspace root")
			}
		}()

		config := &BaseControllerConfig{
			Name:              "test-controller",
			ResyncPeriod:      time.Minute,
			WorkerCount:       1,
			Metrics:           NewManagerMetrics(),
			InformerFactory:   nil,
			WorkspaceRoot:     logicalcluster.Name(""), // Empty workspace
			AllowedWorkspaces: nil,
		}
		NewBaseController(config)
	})
}

// mockRuntimeObject implements runtime.Object for testing
type mockRuntimeObject struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (m *mockRuntimeObject) DeepCopyObject() runtime.Object {
	return &mockRuntimeObject{
		TypeMeta:   m.TypeMeta,
		ObjectMeta: *m.ObjectMeta.DeepCopy(),
	}
}