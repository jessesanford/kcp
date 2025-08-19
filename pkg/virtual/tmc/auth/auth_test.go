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

package auth

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/kcp/pkg/features"
)

// TestTMCVirtualWorkspaceAuthenticator tests the TMC virtual workspace authenticator.
func TestTMCVirtualWorkspaceAuthenticator(t *testing.T) {
	tests := map[string]struct {
		requestURL        string
		userInfo          user.Info
		expectSuccess     bool
		expectError       bool
		delegateSuccess   bool
		workspaceAuthFunc func(ctx context.Context, user user.Info, workspace logicalcluster.Name) error
	}{
		"valid TMC virtual workspace request": {
			requestURL:      "/services/tmc/workspaces/test-workspace/api/v1/clusterregistrations",
			userInfo:        &user.DefaultInfo{Name: "test-user"},
			expectSuccess:   true,
			expectError:     false,
			delegateSuccess: true,
			workspaceAuthFunc: func(ctx context.Context, user user.Info, workspace logicalcluster.Name) error {
				return nil
			},
		},
		"invalid URL format": {
			requestURL:      "/invalid/path",
			userInfo:        &user.DefaultInfo{Name: "test-user"},
			expectSuccess:   false,
			expectError:     true,
			delegateSuccess: true,
			workspaceAuthFunc: func(ctx context.Context, user user.Info, workspace logicalcluster.Name) error {
				return nil
			},
		},
		"workspace authentication failure": {
			requestURL:      "/services/tmc/workspaces/test-workspace/api/v1/clusterregistrations",
			userInfo:        &user.DefaultInfo{Name: "test-user"},
			expectSuccess:   false,
			expectError:     true,
			delegateSuccess: true,
			workspaceAuthFunc: func(ctx context.Context, user user.Info, workspace logicalcluster.Name) error {
				return ErrWorkspaceAccessDenied
			},
		},
		"delegate authentication failure": {
			requestURL:      "/services/tmc/workspaces/test-workspace/api/v1/clusterregistrations",
			userInfo:        &user.DefaultInfo{Name: "test-user"},
			expectSuccess:   false,
			expectError:     false,
			delegateSuccess: false,
			workspaceAuthFunc: func(ctx context.Context, user user.Info, workspace logicalcluster.Name) error {
				return nil
			},
		},
	}

	// Enable TMC feature for testing
	originalValue := features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled)
	features.DefaultMutableFeatureGate.Set("TMCEnabled=true")
	defer func() {
		if originalValue {
			features.DefaultMutableFeatureGate.Set("TMCEnabled=true")
		} else {
			features.DefaultMutableFeatureGate.Set("TMCEnabled=false")
		}
	}()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create mock delegate authenticator
			mockDelegate := &mockAuthenticator{
				success:  tc.delegateSuccess,
				userInfo: tc.userInfo,
			}

			// Create mock workspace authenticator
			mockWorkspaceAuth := &mockWorkspaceAuthenticator{
				authFunc: tc.workspaceAuthFunc,
			}

			// Create TMC authenticator
			tmcAuth := NewTMCVirtualWorkspaceAuthenticator(mockDelegate, mockWorkspaceAuth)

			// Create test request
			req := &http.Request{
				URL: &url.URL{Path: tc.requestURL},
			}

			// Test authentication
			resp, success, err := tmcAuth.AuthenticateRequest(req)

			// Validate results
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if success != tc.expectSuccess {
				t.Errorf("Expected success %v but got %v", tc.expectSuccess, success)
			}

			if tc.expectSuccess && (resp == nil || resp.User != tc.userInfo) {
				t.Errorf("Expected valid response with user info")
			}
		})
	}
}

// TestTMCVirtualWorkspaceAuthorizer tests the TMC virtual workspace authorizer.
func TestTMCVirtualWorkspaceAuthorizer(t *testing.T) {
	tests := map[string]struct {
		resource      string
		verb          string
		requestPath   string
		userInfo      user.Info
		expectDecision authorizer.Decision
		expectError    bool
	}{
		"valid TMC resource access": {
			resource:       "clusterregistrations",
			verb:           "get",
			requestPath:    "/services/tmc/workspaces/test/api/v1/clusterregistrations",
			userInfo:       &user.DefaultInfo{Name: "test-user"},
			expectDecision: authorizer.DecisionAllow,
			expectError:    false,
		},
		"invalid verb for TMC resource": {
			resource:       "clusterregistrations",
			verb:           "invalid-verb",
			requestPath:    "/services/tmc/workspaces/test/api/v1/clusterregistrations",
			userInfo:       &user.DefaultInfo{Name: "test-user"},
			expectDecision: authorizer.DecisionDeny,
			expectError:    false,
		},
		"non-TMC resource through virtual workspace": {
			resource:       "pods",
			verb:           "get",
			requestPath:    "/services/tmc/workspaces/test/api/v1/pods",
			userInfo:       &user.DefaultInfo{Name: "test-user"},
			expectDecision: authorizer.DecisionDeny,
			expectError:    false,
		},
		"system user access": {
			resource:       "clusterregistrations",
			verb:           "get",
			requestPath:    "/services/tmc/workspaces/test/api/v1/clusterregistrations",
			userInfo:       &user.DefaultInfo{Name: "system:test-user"},
			expectDecision: authorizer.DecisionAllow,
			expectError:    false,
		},
	}

	// Enable TMC feature for testing
	originalValue := features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled)
	features.DefaultMutableFeatureGate.Set("TMCEnabled=true")
	defer func() {
		if originalValue {
			features.DefaultMutableFeatureGate.Set("TMCEnabled=true")
		} else {
			features.DefaultMutableFeatureGate.Set("TMCEnabled=false")
		}
	}()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create mock delegate authorizer
			mockDelegate := &mockAuthorizer{
				decision: authorizer.DecisionAllow,
			}

			// Create TMC authorizer
			tmcAuth := NewTMCVirtualWorkspaceAuthorizer(mockDelegate)

			// Create test attributes
			attrs := &mockAuthorizerAttributes{
				resource: tc.resource,
				verb:     tc.verb,
				userInfo: tc.userInfo,
			}

			// Create context with request info
			ctx := context.WithValue(context.Background(), "request-path", tc.requestPath)

			// Test authorization
			decision, reason, err := tmcAuth.Authorize(ctx, attrs)

			// Validate results
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if decision != tc.expectDecision {
				t.Errorf("Expected decision %v but got %v (reason: %s)", tc.expectDecision, decision, reason)
			}
		})
	}
}

// Test helper types and functions

type mockAuthenticator struct {
	success  bool
	userInfo user.Info
}

func (m *mockAuthenticator) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	if m.success {
		return &authenticator.Response{User: m.userInfo}, true, nil
	}
	return nil, false, nil
}

type mockWorkspaceAuthenticator struct {
	authFunc func(ctx context.Context, user user.Info, workspace logicalcluster.Name) error
}

func (m *mockWorkspaceAuthenticator) AuthenticateWorkspace(ctx context.Context, user user.Info, workspace logicalcluster.Name) error {
	return m.authFunc(ctx, user, workspace)
}

func (m *mockWorkspaceAuthenticator) GetUserWorkspaces(ctx context.Context, user user.Info) ([]logicalcluster.Name, error) {
	return nil, nil
}

type mockAuthorizer struct {
	decision authorizer.Decision
	reason   string
}

func (m *mockAuthorizer) Authorize(ctx context.Context, attr authorizer.Attributes) (authorizer.Decision, string, error) {
	return m.decision, m.reason, nil
}

type mockAuthorizerAttributes struct {
	resource string
	verb     string
	userInfo user.Info
}

func (m *mockAuthorizerAttributes) GetUser() user.Info               { return m.userInfo }
func (m *mockAuthorizerAttributes) GetVerb() string                  { return m.verb }
func (m *mockAuthorizerAttributes) IsReadOnly() bool                 { return false }
func (m *mockAuthorizerAttributes) GetNamespace() string             { return "" }
func (m *mockAuthorizerAttributes) GetResource() string              { return m.resource }
func (m *mockAuthorizerAttributes) GetSubresource() string           { return "" }
func (m *mockAuthorizerAttributes) GetName() string                  { return "" }
func (m *mockAuthorizerAttributes) GetAPIGroup() string              { return "tmc.kcp.io" }
func (m *mockAuthorizerAttributes) GetAPIVersion() string            { return "v1alpha1" }
func (m *mockAuthorizerAttributes) IsResourceRequest() bool          { return true }
func (m *mockAuthorizerAttributes) GetPath() string                  { return "" }

// Error definitions
var (
	ErrWorkspaceAccessDenied = fmt.Errorf("workspace access denied")
)