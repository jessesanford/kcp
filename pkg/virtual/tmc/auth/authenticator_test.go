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
	"net/http"
	"net/url"
	"testing"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
)

type mockAuthenticator struct {
	user *user.DefaultInfo
	ok   bool
	err  error
}

func (m *mockAuthenticator) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	if m.err != nil {
		return nil, false, m.err
	}
	if !m.ok {
		return nil, false, nil
	}
	return &authenticator.Response{User: m.user}, true, nil
}

type mockWorkspaceProvider struct {
	workspaces    map[string]*v1alpha1.Workspace
	userAccess    map[string][]*v1alpha1.Workspace
}

func (m *mockWorkspaceProvider) GetWorkspace(ctx context.Context, name string) (*v1alpha1.Workspace, error) {
	return m.workspaces[name], nil
}

func (m *mockWorkspaceProvider) ListWorkspacesForUser(ctx context.Context, user user.Info) ([]*v1alpha1.Workspace, error) {
	return m.userAccess[user.GetName()], nil
}

func TestTMCAuthenticator_AuthenticateRequest(t *testing.T) {
	testUser := &user.DefaultInfo{
		Name:   "test-user",
		UID:    "test-uid",
		Groups: []string{"test-group"},
	}

	testWorkspace := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-workspace",
		},
	}

	tests := []struct {
		name                    string
		path                   string
		delegateUser           *user.DefaultInfo
		delegateOK             bool
		delegateErr            error
		workspaces             map[string]*v1alpha1.Workspace
		userAccess             map[string][]*v1alpha1.Workspace
		expectAuth             bool
		expectWorkspaceInGroup bool
	}{
		{
			name:         "successful authentication with workspace access",
			path:         "/services/apiexport/test-workspace/api/v1/pods",
			delegateUser: testUser,
			delegateOK:   true,
			workspaces: map[string]*v1alpha1.Workspace{
				"test-workspace": testWorkspace,
			},
			userAccess: map[string][]*v1alpha1.Workspace{
				"test-user": {testWorkspace},
			},
			expectAuth:             true,
			expectWorkspaceInGroup: true,
		},
		{
			name:         "authentication without workspace in path",
			path:         "/api/v1/pods",
			delegateUser: testUser,
			delegateOK:   true,
			workspaces:   map[string]*v1alpha1.Workspace{},
			userAccess:   map[string][]*v1alpha1.Workspace{},
			expectAuth:   true,
		},
		{
			name:         "failed delegation",
			path:         "/services/apiexport/test-workspace/api/v1/pods",
			delegateUser: nil,
			delegateOK:   false,
			workspaces:   map[string]*v1alpha1.Workspace{},
			userAccess:   map[string][]*v1alpha1.Workspace{},
			expectAuth:   false,
		},
		{
			name:         "workspace access denied",
			path:         "/services/apiexport/test-workspace/api/v1/pods",
			delegateUser: testUser,
			delegateOK:   true,
			workspaces: map[string]*v1alpha1.Workspace{
				"test-workspace": testWorkspace,
			},
			userAccess: map[string][]*v1alpha1.Workspace{
				"test-user": {}, // No access to any workspace
			},
			expectAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuth := &mockAuthenticator{
				user: tt.delegateUser,
				ok:   tt.delegateOK,
				err:  tt.delegateErr,
			}

			mockProvider := &mockWorkspaceProvider{
				workspaces: tt.workspaces,
				userAccess: tt.userAccess,
			}

			auth := NewTMCAuthenticator(mockAuth, mockProvider)

			req := &http.Request{
				URL: &url.URL{Path: tt.path},
			}
			req = req.WithContext(context.Background())

			resp, ok, err := auth.AuthenticateRequest(req)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if ok != tt.expectAuth {
				t.Errorf("expected authentication %v, got %v", tt.expectAuth, ok)
				return
			}

			if !tt.expectAuth {
				return
			}

			if resp == nil || resp.User == nil {
				t.Error("expected valid response with user")
				return
			}

			if tt.expectWorkspaceInGroup {
				found := false
				for _, group := range resp.User.GetGroups() {
					if group == "workspace:test-workspace" {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected workspace group to be added to user")
				}
			}
		})
	}
}

func TestExtractWorkspaceFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/services/apiexport/test-workspace/api/v1/pods", "test-workspace"},
		{"/clusters/cluster-a/api/v1/nodes", "cluster-a"},
		{"/api/v1/pods", ""},
		{"/services/apiexport", ""},
		{"/clusters", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractWorkspaceFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("extractWorkspaceFromPath(%q) = %q, expected %q", tt.path, result, tt.expected)
			}
		})
	}
}