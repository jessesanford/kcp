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

package auth

import (
	"context"
	"testing"

	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/authentication/user"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
)

type mockAuthorizer struct {
	decision authorizer.Decision
	reason   string
	err      error
}

func (m *mockAuthorizer) Authorize(ctx context.Context, attr authorizer.Attributes) (authorizer.Decision, string, error) {
	return m.decision, m.reason, m.err
}

type mockAttributes struct {
	user      authorizerfactory.UserInfo
	verb      string
	apiGroup  string
	resource  string
	path      string
	namespace string
}

func (m *mockAttributes) GetUser() authorizerfactory.UserInfo { return m.user }
func (m *mockAttributes) GetVerb() string                     { return m.verb }
func (m *mockAttributes) GetNamespace() string               { return m.namespace }
func (m *mockAttributes) GetResource() string                { return m.resource }
func (m *mockAttributes) GetSubresource() string             { return "" }
func (m *mockAttributes) GetName() string                    { return "" }
func (m *mockAttributes) GetAPIGroup() string                { return m.apiGroup }
func (m *mockAttributes) GetAPIVersion() string              { return "v1alpha1" }
func (m *mockAttributes) IsReadOnly() bool                   { return m.verb == "get" || m.verb == "list" || m.verb == "watch" }
func (m *mockAttributes) GetPath() string                    { return m.path }

func TestTMCAuthorizer_Authorize(t *testing.T) {
	testUser := &user.DefaultInfo{
		Name:   "test-user",
		UID:    "test-uid",
		Groups: []string{"test-group"},
	}

	adminUser := &user.DefaultInfo{
		Name:   "system:admin",
		UID:    "admin-uid",
		Groups: []string{"system:masters"},
	}

	testWorkspace := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-workspace",
		},
	}

	tests := []struct {
		name               string
		attr               *mockAttributes
		delegateDecision   authorizer.Decision
		delegateReason     string
		delegateErr        error
		workspaces         map[string]*v1alpha1.Workspace
		userAccess         map[string][]*v1alpha1.Workspace
		expectedDecision   authorizer.Decision
		expectDelegation   bool
	}{
		{
			name: "successful TMC resource access",
			attr: &mockAttributes{
				user:     testUser,
				verb:     "get",
				apiGroup: "tmc.kcp.io/v1alpha1",
				resource: "clusters",
				path:     "/services/apiexport/test-workspace/api/v1alpha1/clusters",
			},
			delegateDecision: authorizer.DecisionAllow,
			delegateReason:   "allowed",
			workspaces: map[string]*v1alpha1.Workspace{
				"test-workspace": testWorkspace,
			},
			userAccess: map[string][]*v1alpha1.Workspace{
				"test-user": {testWorkspace},
			},
			expectedDecision: authorizer.DecisionAllow,
			expectDelegation: true,
		},
		{
			name: "admin user full access",
			attr: &mockAttributes{
				user:     adminUser,
				verb:     "create",
				apiGroup: "tmc.kcp.io/v1alpha1",
				resource: "clusters",
				path:     "/services/apiexport/test-workspace/api/v1alpha1/clusters",
			},
			delegateDecision: authorizer.DecisionAllow,
			delegateReason:   "allowed",
			workspaces: map[string]*v1alpha1.Workspace{
				"test-workspace": testWorkspace,
			},
			userAccess: map[string][]*v1alpha1.Workspace{
				"system:admin": {testWorkspace},
			},
			expectedDecision: authorizer.DecisionAllow,
			expectDelegation: true,
		},
		{
			name: "no workspace in path - delegate",
			attr: &mockAttributes{
				user:     testUser,
				verb:     "get",
				apiGroup: "v1",
				resource: "pods",
				path:     "/api/v1/pods",
			},
			delegateDecision: authorizer.DecisionAllow,
			delegateReason:   "allowed",
			workspaces:       map[string]*v1alpha1.Workspace{},
			userAccess:       map[string][]*v1alpha1.Workspace{},
			expectedDecision: authorizer.DecisionAllow,
			expectDelegation: true,
		},
		{
			name: "workspace access denied",
			attr: &mockAttributes{
				user:     testUser,
				verb:     "get",
				apiGroup: "tmc.kcp.io/v1alpha1",
				resource: "clusters",
				path:     "/services/apiexport/test-workspace/api/v1alpha1/clusters",
			},
			workspaces: map[string]*v1alpha1.Workspace{
				"test-workspace": testWorkspace,
			},
			userAccess: map[string][]*v1alpha1.Workspace{
				"test-user": {}, // No access to workspace
			},
			expectedDecision: authorizer.DecisionDeny,
			expectDelegation: false,
		},
		{
			name: "TMC resource not allowed",
			attr: &mockAttributes{
				user:     testUser,
				verb:     "get",
				apiGroup: "tmc.kcp.io/v1alpha1",
				resource: "secrets", // Not in allowed resources
				path:     "/services/apiexport/test-workspace/api/v1alpha1/secrets",
			},
			workspaces: map[string]*v1alpha1.Workspace{
				"test-workspace": testWorkspace,
			},
			userAccess: map[string][]*v1alpha1.Workspace{
				"test-user": {testWorkspace},
			},
			expectedDecision: authorizer.DecisionDeny,
			expectDelegation: false,
		},
		{
			name: "verb not allowed for regular user",
			attr: &mockAttributes{
				user:     testUser,
				verb:     "delete", // Not in default permissions
				apiGroup: "tmc.kcp.io/v1alpha1",
				resource: "clusters",
				path:     "/services/apiexport/test-workspace/api/v1alpha1/clusters",
			},
			workspaces: map[string]*v1alpha1.Workspace{
				"test-workspace": testWorkspace,
			},
			userAccess: map[string][]*v1alpha1.Workspace{
				"test-user": {testWorkspace},
			},
			expectedDecision: authorizer.DecisionDeny,
			expectDelegation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuth := &mockAuthorizer{
				decision: tt.delegateDecision,
				reason:   tt.delegateReason,
				err:      tt.delegateErr,
			}

			mockProvider := &mockWorkspaceProvider{
				workspaces: tt.workspaces,
				userAccess: tt.userAccess,
			}

			auth := NewTMCAuthorizer(mockAuth, mockProvider)

			decision, reason, err := auth.Authorize(context.Background(), tt.attr)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if decision != tt.expectedDecision {
				t.Errorf("expected decision %v, got %v (reason: %s)", tt.expectedDecision, decision, reason)
			}
		})
	}
}

func TestExtractWorkspaceFromURL(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/services/apiexport/test-workspace/api/v1/clusters", "test-workspace"},
		{"/clusters/cluster-a/api/v1/nodes", "cluster-a"},
		{"/api/v1/pods", ""},
		{"/services/apiexport", ""},
		{"/clusters", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractWorkspaceFromURL(tt.path)
			if result != tt.expected {
				t.Errorf("extractWorkspaceFromURL(%q) = %q, expected %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsWorkspaceAdmin(t *testing.T) {
	auth := &TMCAuthorizer{}

	tests := []struct {
		name     string
		user     authorizerfactory.UserInfo
		workspace string
		expected bool
	}{
		{
			name: "system admin",
			user: &user.DefaultInfo{
				Name: "system:admin",
			},
			workspace: "test-workspace",
			expected:  true,
		},
		{
			name: "masters group member",
			user: &user.DefaultInfo{
				Name:   "test-user",
				Groups: []string{"system:masters"},
			},
			workspace: "test-workspace",
			expected:  true,
		},
		{
			name: "workspace admin group",
			user: &user.DefaultInfo{
				Name:   "test-user",
				Groups: []string{"workspace:test-workspace:admin"},
			},
			workspace: "test-workspace",
			expected:  true,
		},
		{
			name: "regular user",
			user: &user.DefaultInfo{
				Name:   "regular-user",
				Groups: []string{"regular-group"},
			},
			workspace: "test-workspace",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := auth.isWorkspaceAdmin(tt.user, tt.workspace)
			if result != tt.expected {
				t.Errorf("isWorkspaceAdmin() = %v, expected %v", result, tt.expected)
			}
		})
	}
}