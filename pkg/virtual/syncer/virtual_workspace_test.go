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

package syncer

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

func TestSyncerVirtualWorkspace_ResolveRootPath(t *testing.T) {
	tests := map[string]struct {
		urlPath        string
		expectAccepted bool
		expectPrefix   string
		expectSyncerID string
		expectWorkspace string
	}{
		"valid syncer path": {
			urlPath:         "/services/syncer/test-syncer/clusters/test-workspace/api/v1/pods",
			expectAccepted:  true,
			expectPrefix:    "/services/syncer/test-syncer/clusters/test-workspace",
			expectSyncerID:  "test-syncer",
			expectWorkspace: "test-workspace",
		},
		"valid syncer path without remainder": {
			urlPath:         "/services/syncer/my-syncer/clusters/my-workspace",
			expectAccepted:  true,
			expectPrefix:    "/services/syncer/my-syncer/clusters/my-workspace",
			expectSyncerID:  "my-syncer",
			expectWorkspace: "my-workspace",
		},
		"invalid path - wrong prefix": {
			urlPath:        "/api/v1/pods",
			expectAccepted: false,
		},
		"invalid path - missing syncer id": {
			urlPath:        "/services/syncer//clusters/workspace",
			expectAccepted: false,
		},
		"invalid path - missing workspace": {
			urlPath:        "/services/syncer/test-syncer/clusters/",
			expectAccepted: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			authConfig := NewDefaultAuthConfig()
			workspace, err := NewSyncerVirtualWorkspace(&AuthConfig{
				ValidateCertificate:    authConfig.ValidateCertificate,
				GetSyncTargetForSyncer: authConfig.GetSyncTargetForSyncer,
			})
			if err != nil {
				t.Fatalf("failed to create workspace: %v", err)
			}

			accepted, prefix, ctx := workspace.ResolveRootPath(tc.urlPath, context.Background())

			if accepted != tc.expectAccepted {
				t.Errorf("expected accepted=%v, got %v", tc.expectAccepted, accepted)
			}

			if accepted {
				if prefix != tc.expectPrefix {
					t.Errorf("expected prefix=%q, got %q", tc.expectPrefix, prefix)
				}

				syncerID, workspaceName, ok := extractSyncerIdentity(ctx)
				if !ok {
					t.Error("expected syncer identity in context")
				}

				if syncerID != tc.expectSyncerID {
					t.Errorf("expected syncerID=%q, got %q", tc.expectSyncerID, syncerID)
				}

				if workspaceName != tc.expectWorkspace {
					t.Errorf("expected workspace=%q, got %q", tc.expectWorkspace, workspaceName)
				}
			}
		})
	}
}

func TestSyncerVirtualWorkspace_Authorize(t *testing.T) {
	authConfig := NewDefaultAuthConfig()
	
	// Register a test SyncTarget
	syncTarget := &workloadv1alpha1.SyncTarget{}
	syncTarget.Name = "test-syncer"
	syncTarget.Namespace = "test-workspace"
	syncTarget.Spec.SupportedResourceTypes = []string{"pods", "services"}
	
	authConfig.RegisterSyncTarget("test-syncer", syncTarget)

	workspace, err := NewSyncerVirtualWorkspace(&AuthConfig{
		ValidateCertificate:    authConfig.ValidateCertificate,
		GetSyncTargetForSyncer: authConfig.GetSyncTargetForSyncer,
	})
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	tests := map[string]struct {
		syncerID       string
		workspaceName  string
		username       string
		resource       string
		expectDecision authorizer.Decision
		expectError    bool
	}{
		"valid syncer certificate and resource": {
			syncerID:       "test-syncer",
			workspaceName:  "test-workspace",
			username:       "system:syncer:test-syncer",
			resource:       "pods",
			expectDecision: authorizer.DecisionAllow,
		},
		"invalid certificate - wrong username format": {
			syncerID:       "test-syncer",
			workspaceName:  "test-workspace",
			username:       "user:test-syncer",
			resource:       "pods",
			expectDecision: authorizer.DecisionDeny,
		},
		"unsupported resource": {
			syncerID:       "test-syncer",
			workspaceName:  "test-workspace",
			username:       "system:syncer:test-syncer",
			resource:       "deployments",
			expectDecision: authorizer.DecisionDeny,
		},
		"non-existent syncer": {
			syncerID:       "unknown-syncer",
			workspaceName:  "test-workspace",
			username:       "system:syncer:unknown-syncer",
			resource:       "pods",
			expectDecision: authorizer.DecisionDeny,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create context with syncer identity
			ctx := withSyncerIdentity(context.Background(), tc.syncerID, tc.workspaceName)

			// Create test attributes
			attrs := &testAttributes{
				user:     &user.DefaultInfo{Name: tc.username},
				resource: tc.resource,
				verb:     "get",
			}

			decision, _, err := workspace.Authorize(ctx, attrs)

			if tc.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if decision != tc.expectDecision {
				t.Errorf("expected decision %v, got %v", tc.expectDecision, decision)
			}
		})
	}
}

func TestSyncerVirtualWorkspace_IsReady(t *testing.T) {
	tests := map[string]struct {
		authConfig  *AuthConfig
		expectError bool
	}{
		"valid configuration": {
			authConfig: &AuthConfig{
				ValidateCertificate:    func(user.Info) error { return nil },
				GetSyncTargetForSyncer: func(string, string) (*workloadv1alpha1.SyncTarget, error) { return nil, nil },
			},
			expectError: false,
		},
		"missing certificate validator": {
			authConfig: &AuthConfig{
				GetSyncTargetForSyncer: func(string, string) (*workloadv1alpha1.SyncTarget, error) { return nil, nil },
			},
			expectError: true,
		},
		"missing sync target resolver": {
			authConfig: &AuthConfig{
				ValidateCertificate: func(user.Info) error { return nil },
			},
			expectError: true,
		},
		"nil auth config": {
			authConfig:  nil,
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			workspace, err := NewSyncerVirtualWorkspace(tc.authConfig)
			if tc.authConfig == nil {
				if err == nil {
					t.Error("expected error for nil auth config")
				}
				return
			}

			if err != nil {
				t.Fatalf("failed to create workspace: %v", err)
			}

			err = workspace.IsReady()

			if tc.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// testAttributes implements authorizer.Attributes for testing
type testAttributes struct {
	user      user.Info
	verb      string
	resource  string
	namespace string
	name      string
}

func (a *testAttributes) GetUser() user.Info                    { return a.user }
func (a *testAttributes) GetVerb() string                       { return a.verb }
func (a *testAttributes) IsReadOnly() bool                      { return a.verb == "get" || a.verb == "list" || a.verb == "watch" }
func (a *testAttributes) GetNamespace() string                  { return a.namespace }
func (a *testAttributes) GetResource() string                   { return a.resource }
func (a *testAttributes) GetSubresource() string                { return "" }
func (a *testAttributes) GetName() string                       { return a.name }
func (a *testAttributes) GetAPIGroup() string                   { return "" }
func (a *testAttributes) GetAPIVersion() string                 { return "v1" }
func (a *testAttributes) IsResourceRequest() bool               { return true }
func (a *testAttributes) GetPath() string                       { return "" }
func (a *testAttributes) GetFieldSelector() (fields.Requirements, error) { return nil, nil }
func (a *testAttributes) GetLabelSelector() (labels.Requirements, error) { return nil, nil }