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

package interfaces

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceDiscoveryInterface provides resource discovery capabilities for virtual workspaces
type ResourceDiscoveryInterface interface {
	// Start initializes the discovery provider and begins monitoring
	Start(ctx context.Context) error

	// Discover returns available resources in the specified workspace
	Discover(ctx context.Context, workspace string) ([]ResourceInfo, error)

	// GetOpenAPISchema returns the OpenAPI schema for workspace resources
	GetOpenAPISchema(ctx context.Context, workspace string) ([]byte, error)

	// Watch monitors for resource changes in the workspace
	Watch(ctx context.Context, workspace string) (<-chan DiscoveryEvent, error)

	// IsResourceAvailable checks if a specific resource is available
	IsResourceAvailable(ctx context.Context, workspace string, gvr schema.GroupVersionResource) (bool, error)
}

// DiscoveryCache provides caching for discovered resources
type DiscoveryCache interface {
	// GetResources retrieves cached resources for a workspace
	GetResources(workspace string) ([]ResourceInfo, bool)

	// SetResources caches resources for a workspace with TTL
	SetResources(workspace string, resources []ResourceInfo, ttl int64)

	// InvalidateWorkspace removes cached data for a workspace
	InvalidateWorkspace(workspace string)

	// Clear removes all cached data
	Clear()
}

// ResourceInfo contains information about a discovered resource
type ResourceInfo struct {
	// GroupVersionResource identifies the resource
	GroupVersionResource schema.GroupVersionResource

	// APIResource contains metadata about the resource
	APIResource metav1.APIResource

	// Workspace identifies the workspace this resource belongs to
	Workspace string

	// APIExportName is the name of the APIExport providing this resource
	APIExportName string

	// OpenAPISchema contains the OpenAPI schema for this resource
	OpenAPISchema []byte

	// IsWorkspaceScoped indicates if the resource is workspace-scoped
	IsWorkspaceScoped bool
}

// DiscoveryEvent represents a change in resource discovery
type DiscoveryEvent struct {
	// Type indicates the type of event (Added, Updated, Deleted)
	Type DiscoveryEventType

	// Workspace identifies the workspace where the event occurred
	Workspace string

	// Resource contains the resource information for the event
	Resource ResourceInfo

	// Timestamp when the event occurred
	Timestamp metav1.Time
}

// DiscoveryEventType represents the type of discovery event
type DiscoveryEventType string

const (
	// DiscoveryEventAdded indicates a resource was added
	DiscoveryEventAdded DiscoveryEventType = "Added"

	// DiscoveryEventUpdated indicates a resource was updated
	DiscoveryEventUpdated DiscoveryEventType = "Updated"

	// DiscoveryEventDeleted indicates a resource was deleted
	DiscoveryEventDeleted DiscoveryEventType = "Deleted"
)