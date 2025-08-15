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

// Package discovery provides abstraction layer for resource discovery in virtual workspaces.
// It defines the Provider interface that enables pluggable discovery implementations
// and includes support for workspace-aware resource discovery, caching, and schema management.
package discovery

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Provider handles resource discovery for virtual workspaces.
// It provides a pluggable interface for different discovery implementations,
// supporting workspace-aware resource discovery, schema retrieval, and change monitoring.
type Provider interface {
	// Name returns the unique identifier for this discovery provider.
	Name() string

	// Initialize sets up the provider with the given configuration.
	// This must be called before using other provider methods.
	Initialize(ctx context.Context, config ProviderConfig) error

	// Discover returns available API resources for the specified workspace.
	// The result includes API groups, resource metadata, and preferred versions.
	Discover(ctx context.Context, workspaceName string) (*DiscoveryResult, error)

	// GetOpenAPISchema returns the OpenAPI schema for a specific resource.
	// Returns the schema as JSON bytes, or empty schema if not available.
	GetOpenAPISchema(ctx context.Context, workspaceName string, gvr schema.GroupVersionResource) ([]byte, error)

	// Watch monitors for changes in resource discovery for a workspace.
	// Returns a channel that receives DiscoveryEvent notifications.
	Watch(ctx context.Context, workspaceName string) (<-chan DiscoveryEvent, error)

	// Refresh forces a refresh of discovery data for the specified workspace.
	// This bypasses any caching and fetches fresh discovery information.
	Refresh(ctx context.Context, workspaceName string) error

	// Close cleans up provider resources and stops all ongoing operations.
	// Should be called when the provider is no longer needed.
	Close(ctx context.Context) error
}

// ProviderConfig contains configuration parameters for discovery providers.
type ProviderConfig struct {
	// WorkspaceManager provides access to workspace operations and metadata.
	// This is an interface{} to avoid dependencies in the contracts package.
	WorkspaceManager interface{}

	// CacheEnabled determines whether discovery results should be cached.
	CacheEnabled bool

	// CacheTTL is the time-to-live for cached discovery data in seconds.
	CacheTTL int64

	// RefreshInterval is the interval for automatic refresh in seconds.
	// Set to 0 to disable automatic refresh.
	RefreshInterval int64
}

// DiscoveryResult contains comprehensive discovery information for a workspace.
type DiscoveryResult struct {
	// Groups lists all available API groups with their versions.
	Groups []metav1.APIGroup

	// Resources maps GroupVersionResource to detailed resource information.
	Resources map[schema.GroupVersionResource]ResourceInfo

	// PreferredVersions maps API group name to its preferred version.
	PreferredVersions map[string]string
}

// ResourceInfo contains detailed information about a discovered API resource.
type ResourceInfo struct {
	metav1.APIResource

	// Schema contains the OpenAPI schema for this resource as JSON bytes.
	Schema []byte

	// WorkspaceScoped indicates whether this resource is scoped to workspaces.
	WorkspaceScoped bool
}

// DiscoveryEvent represents a change in resource discovery.
type DiscoveryEvent struct {
	// Type specifies the kind of discovery change that occurred.
	Type EventType

	// Workspace is the name of the workspace where the change occurred.
	Workspace string

	// Resource contains the resource information, if applicable.
	Resource *ResourceInfo

	// Error contains any error that occurred during discovery.
	Error error
}

// EventType represents the type of discovery event.
type EventType string

const (
	// EventTypeResourceAdded indicates a new resource was discovered.
	EventTypeResourceAdded EventType = "ResourceAdded"

	// EventTypeResourceRemoved indicates a resource is no longer available.
	EventTypeResourceRemoved EventType = "ResourceRemoved"

	// EventTypeResourceUpdated indicates an existing resource was modified.
	EventTypeResourceUpdated EventType = "ResourceUpdated"
)