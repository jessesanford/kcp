/*
Copyright 2023 The KCP Authors.

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

package discovery

import (
	"context"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MockProvider provides mock discovery functionality for testing virtual workspace discovery.
// It maintains in-memory resource data and supports all Provider interface operations
// with predictable behavior for test scenarios.
type MockProvider struct {
	mu        sync.RWMutex
	name      string
	resources map[string]*DiscoveryResult
	schemas   map[string][]byte
	watchers  map[string][]chan DiscoveryEvent
	config    ProviderConfig
}

// NewMockProvider creates a new mock discovery provider with the given name.
// The provider starts with default resources that can be extended for testing.
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:      name,
		resources: make(map[string]*DiscoveryResult),
		schemas:   make(map[string][]byte),
		watchers:  make(map[string][]chan DiscoveryEvent),
	}
}

// Name returns the provider name.
func (m *MockProvider) Name() string {
	return m.name
}

// Initialize sets up the mock provider with the given configuration.
// This initializes default resources that are available for all workspaces.
func (m *MockProvider) Initialize(ctx context.Context, config ProviderConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config

	// Initialize with default resources available to all workspaces
	m.setupDefaultResources()

	return nil
}

// Discover returns mock discovery results for the specified workspace.
// If no specific resources are configured for a workspace, returns default resources.
func (m *MockProvider) Discover(ctx context.Context, workspaceName string) (*DiscoveryResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.resources[workspaceName]
	if !ok {
		// Return default resources for unknown workspaces
		return m.getDefaultDiscoveryResult(), nil
	}

	return result, nil
}

// GetOpenAPISchema returns mock OpenAPI schema for the specified resource.
// Returns a simple JSON object if no specific schema is configured.
func (m *MockProvider) GetOpenAPISchema(ctx context.Context, workspaceName string, gvr schema.GroupVersionResource) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", workspaceName, gvr.String())
	schema, ok := m.schemas[key]
	if !ok {
		return []byte("{}"), nil // Return empty schema for unknown resources
	}

	return schema, nil
}

// Watch monitors for mock discovery changes in the specified workspace.
// Returns a buffered channel that receives DiscoveryEvent notifications.
func (m *MockProvider) Watch(ctx context.Context, workspaceName string) (<-chan DiscoveryEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan DiscoveryEvent, 10)
	m.watchers[workspaceName] = append(m.watchers[workspaceName], ch)

	return ch, nil
}

// Refresh triggers a mock refresh for the specified workspace.
// This sends a ResourceUpdated event to all watchers for the workspace.
func (m *MockProvider) Refresh(ctx context.Context, workspaceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate refresh by sending event to watchers
	event := DiscoveryEvent{
		Type:      EventTypeResourceUpdated,
		Workspace: workspaceName,
	}

	for _, ch := range m.watchers[workspaceName] {
		select {
		case ch <- event:
		default:
			// Channel full, skip to avoid blocking
		}
	}

	return nil
}

// Close cleans up the mock provider by closing all watcher channels.
// This ensures no goroutines are leaked when the provider is no longer needed.
func (m *MockProvider) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all watcher channels to prevent goroutine leaks
	for _, watchers := range m.watchers {
		for _, ch := range watchers {
			close(ch)
		}
	}

	m.watchers = make(map[string][]chan DiscoveryEvent)
	return nil
}

// setupDefaultResources initializes the provider with standard Kubernetes resources.
// These resources are available to all workspaces by default.
func (m *MockProvider) setupDefaultResources() {
	defaultResult := m.getDefaultDiscoveryResult()
	m.resources["default"] = defaultResult
}

// getDefaultDiscoveryResult returns a standard set of Kubernetes API resources.
// This includes common resources like deployments that are typically available.
func (m *MockProvider) getDefaultDiscoveryResult() *DiscoveryResult {
	return &DiscoveryResult{
		Groups: []metav1.APIGroup{
			{
				Name: "apps",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "apps/v1", Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{
					GroupVersion: "apps/v1",
					Version:      "v1",
				},
			},
		},
		Resources: map[schema.GroupVersionResource]ResourceInfo{
			{Group: "apps", Version: "v1", Resource: "deployments"}: {
				APIResource: metav1.APIResource{
					Name:       "deployments",
					Namespaced: true,
					Kind:       "Deployment",
					Verbs:      metav1.Verbs{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				WorkspaceScoped: true,
			},
		},
		PreferredVersions: map[string]string{
			"apps": "v1",
		},
	}
}

// AddResource adds a mock resource for testing scenarios.
// This allows tests to configure specific resources for specific workspaces.
func (m *MockProvider) AddResource(workspaceName string, gvr schema.GroupVersionResource, resource ResourceInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.resources[workspaceName]; !ok {
		m.resources[workspaceName] = &DiscoveryResult{
			Resources: make(map[schema.GroupVersionResource]ResourceInfo),
		}
	}

	m.resources[workspaceName].Resources[gvr] = resource
}

// AddSchema adds a mock OpenAPI schema for testing scenarios.
// This allows tests to configure specific schemas for specific resources.
func (m *MockProvider) AddSchema(workspaceName string, gvr schema.GroupVersionResource, schemaBytes []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", workspaceName, gvr.String())
	m.schemas[key] = schemaBytes
}