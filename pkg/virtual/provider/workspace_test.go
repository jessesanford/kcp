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

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/kcp-dev/logicalcluster/v3"
)

// mockResourceDiscovery implements ResourceDiscoveryInterface for testing
type mockResourceDiscovery struct {
	resources map[logicalcluster.Name][]metav1.APIResource
	errors    map[logicalcluster.Name]error
}

func newMockResourceDiscovery() *mockResourceDiscovery {
	return &mockResourceDiscovery{
		resources: make(map[logicalcluster.Name][]metav1.APIResource),
		errors:    make(map[logicalcluster.Name]error),
	}
}

func (m *mockResourceDiscovery) DiscoverResources(ctx context.Context, workspace logicalcluster.Name) ([]metav1.APIResource, error) {
	if err, exists := m.errors[workspace]; exists {
		return nil, err
	}
	if resources, exists := m.resources[workspace]; exists {
		return resources, nil
	}
	return []metav1.APIResource{}, nil
}

func (m *mockResourceDiscovery) RefreshResources(ctx context.Context, workspace logicalcluster.Name) error {
	if err, exists := m.errors[workspace]; exists {
		return err
	}
	return nil
}

// mockAuthorizationProvider implements AuthorizationProvider for testing
type mockAuthorizationProvider struct {
	decisions map[string]authorizer.Decision
	reasons   map[string]string
	errors    map[string]error
}

func newMockAuthorizationProvider() *mockAuthorizationProvider {
	return &mockAuthorizationProvider{
		decisions: make(map[string]authorizer.Decision),
		reasons:   make(map[string]string),
		errors:    make(map[string]error),
	}
}

func (m *mockAuthorizationProvider) Authorize(ctx context.Context, attributes authorizer.Attributes) (authorizer.Decision, string, error) {
	key := attributes.GetResource() + ":" + attributes.GetVerb()
	if err, exists := m.errors[key]; exists {
		return authorizer.DecisionNoOpinion, "", err
	}
	decision := m.decisions[key]
	if decision == authorizer.DecisionNoOpinion {
		decision = authorizer.DecisionAllow // default to allow
	}
	reason := m.reasons[key]
	return decision, reason, nil
}

func (m *mockAuthorizationProvider) GetWorkspaceAccess(ctx context.Context, user string, workspace logicalcluster.Name) (string, error) {
	return "full", nil
}

// mockWorkspaceCache implements WorkspaceCache for testing
type mockWorkspaceCache struct {
	workspaces map[logicalcluster.Name]*VirtualWorkspace
	errors     map[logicalcluster.Name]error
}

func newMockWorkspaceCache() *mockWorkspaceCache {
	return &mockWorkspaceCache{
		workspaces: make(map[logicalcluster.Name]*VirtualWorkspace),
		errors:     make(map[logicalcluster.Name]error),
	}
}

func (m *mockWorkspaceCache) Get(name logicalcluster.Name) (*VirtualWorkspace, error) {
	if err, exists := m.errors[name]; exists {
		return nil, err
	}
	if workspace, exists := m.workspaces[name]; exists {
		return workspace, nil
	}
	return nil, ErrWorkspaceNotFound
}

func (m *mockWorkspaceCache) Set(workspace *VirtualWorkspace) error {
	if err, exists := m.errors[workspace.Name]; exists {
		return err
	}
	m.workspaces[workspace.Name] = workspace
	return nil
}

func (m *mockWorkspaceCache) Delete(name logicalcluster.Name) error {
	if err, exists := m.errors[name]; exists {
		return err
	}
	delete(m.workspaces, name)
	return nil
}

func (m *mockWorkspaceCache) List() ([]*VirtualWorkspace, error) {
	workspaces := make([]*VirtualWorkspace, 0, len(m.workspaces))
	for _, workspace := range m.workspaces {
		workspaces = append(workspaces, workspace)
	}
	return workspaces, nil
}

// Common test error
var ErrWorkspaceNotFound = &WorkspaceError{Code: "NotFound", Message: "workspace not found"}

type WorkspaceError struct {
	Code    string
	Message string
}

func (e *WorkspaceError) Error() string {
	return e.Message
}

func TestNewWorkspaceProvider(t *testing.T) {
	tests := []struct {
		name        string
		discovery   ResourceDiscoveryInterface
		auth        AuthorizationProvider
		cache       WorkspaceCache
		expectError bool
	}{
		{
			name:        "valid dependencies",
			discovery:   newMockResourceDiscovery(),
			auth:        newMockAuthorizationProvider(),
			cache:       newMockWorkspaceCache(),
			expectError: false,
		},
		{
			name:        "nil discovery",
			discovery:   nil,
			auth:        newMockAuthorizationProvider(),
			cache:       newMockWorkspaceCache(),
			expectError: true,
		},
		{
			name:        "nil auth",
			discovery:   newMockResourceDiscovery(),
			auth:        nil,
			cache:       newMockWorkspaceCache(),
			expectError: true,
		},
		{
			name:        "nil cache",
			discovery:   newMockResourceDiscovery(),
			auth:        newMockAuthorizationProvider(),
			cache:       nil,
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider, err := NewWorkspaceProvider(test.discovery, test.auth, test.cache)
			
			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				if provider != nil {
					t.Errorf("Expected nil provider but got: %v", provider)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if provider == nil {
					t.Errorf("Expected provider but got nil")
				}
			}
		})
	}
}

func TestWorkspaceProvider_Initialize(t *testing.T) {
	discovery := newMockResourceDiscovery()
	auth := newMockAuthorizationProvider()
	cache := newMockWorkspaceCache()

	// Add test workspace to cache
	testWorkspace := &VirtualWorkspace{
		Name:        logicalcluster.Name("test-workspace"),
		Config:      &VirtualWorkspaceConfig{Enabled: true},
		Resources:   []metav1.APIResource{},
		LastUpdated: metav1.Now(),
		Status:      VirtualWorkspaceStatusActive,
	}
	cache.Set(testWorkspace)

	provider, err := NewWorkspaceProvider(discovery, auth, cache)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.IsReady() {
		t.Errorf("Provider should not be ready before initialization")
	}

	err = provider.Initialize(context.Background())
	if err != nil {
		t.Errorf("Failed to initialize provider: %v", err)
	}

	if !provider.IsReady() {
		t.Errorf("Provider should be ready after initialization")
	}

	// Check that cached workspace was loaded
	loadedWorkspace, err := provider.GetWorkspace("test-workspace")
	if err != nil {
		t.Errorf("Failed to get workspace: %v", err)
	}
	if loadedWorkspace.Name != testWorkspace.Name {
		t.Errorf("Expected workspace name %s, got %s", testWorkspace.Name, loadedWorkspace.Name)
	}
}

func TestWorkspaceProvider_GetWorkspace(t *testing.T) {
	discovery := newMockResourceDiscovery()
	auth := newMockAuthorizationProvider()
	cache := newMockWorkspaceCache()

	provider, err := NewWorkspaceProvider(discovery, auth, cache)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	tests := []struct {
		name          string
		workspaceName string
		setupCache    func()
		expectError   bool
	}{
		{
			name:          "existing workspace",
			workspaceName: "test-workspace",
			setupCache: func() {
				cache.Set(&VirtualWorkspace{
					Name:   logicalcluster.Name("test-workspace"),
					Status: VirtualWorkspaceStatusActive,
				})
			},
			expectError: false,
		},
		{
			name:          "non-existing workspace",
			workspaceName: "non-existing",
			setupCache:    func() {},
			expectError:   true,
		},
		{
			name:          "empty workspace name",
			workspaceName: "",
			setupCache:    func() {},
			expectError:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clear cache and setup for this test
			cache.workspaces = make(map[logicalcluster.Name]*VirtualWorkspace)
			provider.workspaces = make(map[logicalcluster.Name]*VirtualWorkspace)
			test.setupCache()

			workspace, err := provider.GetWorkspace(test.workspaceName)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				if workspace != nil {
					t.Errorf("Expected nil workspace but got: %v", workspace)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if workspace == nil {
					t.Errorf("Expected workspace but got nil")
				}
				if workspace != nil && string(workspace.Name) != test.workspaceName {
					t.Errorf("Expected workspace name %s, got %s", test.workspaceName, workspace.Name)
				}
			}
		})
	}
}

func TestRouter_ExtractWorkspace(t *testing.T) {
	provider, err := NewWorkspaceProvider(
		newMockResourceDiscovery(),
		newMockAuthorizationProvider(),
		newMockWorkspaceCache(),
	)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	router := provider.router

	tests := []struct {
		name          string
		path          string
		expectedWS    string
		expectError   bool
	}{
		{
			name:        "valid workspace path",
			path:        "/clusters/test-workspace/api/v1/pods",
			expectedWS:  "test-workspace",
			expectError: false,
		},
		{
			name:        "valid workspace path with apis",
			path:        "/clusters/my-workspace/apis/apps/v1/deployments",
			expectedWS:  "my-workspace",
			expectError: false,
		},
		{
			name:        "workspace path without trailing slash",
			path:        "/clusters/simple-ws",
			expectedWS:  "simple-ws",
			expectError: false,
		},
		{
			name:        "invalid path format",
			path:        "/invalid/path",
			expectedWS:  "",
			expectError: true,
		},
		{
			name:        "empty path",
			path:        "",
			expectedWS:  "",
			expectError: true,
		},
		{
			name:        "path with invalid workspace name",
			path:        "/clusters/Invalid_Name/api/v1/pods",
			expectedWS:  "",
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace, err := router.extractWorkspace(test.path)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if string(workspace) != test.expectedWS {
					t.Errorf("Expected workspace %s, got %s", test.expectedWS, workspace)
				}
			}
		})
	}
}

func TestWorkspaceProvider_ServeHTTP(t *testing.T) {
	discovery := newMockResourceDiscovery()
	auth := newMockAuthorizationProvider()
	cache := newMockWorkspaceCache()

	provider, err := NewWorkspaceProvider(discovery, auth, cache)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Add a test workspace
	testWorkspace := &VirtualWorkspace{
		Name:   logicalcluster.Name("test-workspace"),
		Status: VirtualWorkspaceStatusActive,
		Config: &VirtualWorkspaceConfig{Enabled: true},
	}
	cache.Set(testWorkspace)
	provider.workspaces[testWorkspace.Name] = testWorkspace

	// Initialize provider
	provider.Initialize(context.Background())

	tests := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
	}{
		{
			name:           "valid workspace request",
			path:           "/clusters/test-workspace/api/v1/pods",
			method:         "GET",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid workspace path",
			path:           "/invalid/path",
			method:         "GET",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "non-existing workspace",
			path:           "/clusters/non-existing/api/v1/pods",
			method:         "GET",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequest(test.method, test.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			recorder := httptest.NewRecorder()
			provider.ServeHTTP(recorder, req)

			if recorder.Code != test.expectedStatus {
				t.Errorf("Expected status %d, got %d", test.expectedStatus, recorder.Code)
				t.Logf("Response body: %s", recorder.Body.String())
			}
		})
	}
}