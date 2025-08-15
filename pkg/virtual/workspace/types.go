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

package workspace

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/kcp-dev/kcp/pkg/authorization"
	"github.com/kcp-dev/logicalcluster/v3"
)

// WorkspaceReference uniquely identifies a virtual workspace within KCP.
// It combines logical cluster information with workspace-specific metadata
// to enable precise workspace targeting and access control.
type WorkspaceReference struct {
	// LogicalCluster represents the logical cluster containing this workspace.
	// This is the primary namespace isolation mechanism in KCP.
	LogicalCluster logicalcluster.Name `json:"logicalCluster"`
	
	// Name is the unique identifier for this workspace within the logical cluster.
	// Must follow KCP workspace naming conventions.
	Name string `json:"name"`
	
	// Type indicates the category or kind of this workspace (e.g., "user", "service", "system").
	// Used for access control policies and resource quotas.
	Type WorkspaceType `json:"type"`
	
	// Labels contains arbitrary key-value pairs for workspace metadata.
	// Used for filtering, organization, and policy application.
	Labels map[string]string `json:"labels,omitempty"`
}

// WorkspaceType defines the category of a virtual workspace.
// Different types may have different access patterns, resource limits,
// and security policies applied.
type WorkspaceType string

const (
	// WorkspaceTypeUser represents workspaces owned by individual users.
	// Typically have restrictive access controls and moderate resource limits.
	WorkspaceTypeUser WorkspaceType = "user"
	
	// WorkspaceTypeService represents workspaces for service accounts or applications.
	// Often have programmatic access patterns and service-specific policies.
	WorkspaceTypeService WorkspaceType = "service"
	
	// WorkspaceTypeSystem represents system-level workspaces for cluster infrastructure.
	// Usually have elevated privileges and special access patterns.
	WorkspaceTypeSystem WorkspaceType = "system"
	
	// WorkspaceTypeShared represents multi-tenant collaborative workspaces.
	// Require complex access control matrices and resource sharing policies.
	WorkspaceTypeShared WorkspaceType = "shared"
)

// WorkspaceState represents the current operational state of a virtual workspace.
// Used for lifecycle management and health monitoring.
type WorkspaceState string

const (
	// WorkspaceStateActive indicates the workspace is fully operational and accessible.
	WorkspaceStateActive WorkspaceState = "Active"
	
	// WorkspaceStateInitializing indicates the workspace is being set up.
	// Resources may not be fully available yet.
	WorkspaceStateInitializing WorkspaceState = "Initializing"
	
	// WorkspaceStateSuspended indicates the workspace is temporarily disabled.
	// Access is blocked but data is preserved.
	WorkspaceStateSuspended WorkspaceState = "Suspended"
	
	// WorkspaceStateTerminating indicates the workspace is being deleted.
	// Resources are being cleaned up and access is blocked.
	WorkspaceStateTerminating WorkspaceState = "Terminating"
	
	// WorkspaceStateError indicates the workspace has encountered a critical error.
	// Manual intervention may be required to restore functionality.
	WorkspaceStateError WorkspaceState = "Error"
)

// WorkspaceInfo contains metadata and status information about a virtual workspace.
// This is the primary data structure for workspace introspection and management.
type WorkspaceInfo struct {
	// Reference uniquely identifies this workspace.
	Reference WorkspaceReference `json:"reference"`
	
	// State represents the current operational state.
	State WorkspaceState `json:"state"`
	
	// CreationTimestamp indicates when this workspace was created.
	CreationTimestamp time.Time `json:"creationTimestamp"`
	
	// LastAccessTimestamp indicates when this workspace was last accessed.
	// Used for lifecycle policies and usage tracking.
	LastAccessTimestamp *time.Time `json:"lastAccessTimestamp,omitempty"`
	
	// ResourceVersion provides optimistic concurrency control for workspace metadata.
	// Must be preserved across updates to prevent conflicts.
	ResourceVersion string `json:"resourceVersion"`
	
	// Capabilities lists the API resources and features available in this workspace.
	// May vary based on workspace type and applied policies.
	Capabilities []WorkspaceCapability `json:"capabilities,omitempty"`
	
	// AccessPolicy defines the authorization rules for this workspace.
	// Determines who can access what resources within the workspace.
	AccessPolicy *authorization.WorkspaceAccessPolicy `json:"accessPolicy,omitempty"`
}

// WorkspaceCapability represents an API resource or feature available in a workspace.
// Used for feature discovery and compatibility checking.
type WorkspaceCapability struct {
	// Group identifies the API group (e.g., "apps", "networking.k8s.io").
	Group string `json:"group"`
	
	// Version specifies the API version (e.g., "v1", "v1beta1").
	Version string `json:"version"`
	
	// Resource names the specific resource type (e.g., "deployments", "services").
	Resource string `json:"resource"`
	
	// Verbs lists the allowed operations (e.g., ["get", "list", "create"]).
	Verbs []string `json:"verbs"`
}

// WorkspaceConfig contains configuration parameters for workspace creation and management.
// Used by WorkspaceProvider implementations to customize behavior.
type WorkspaceConfig struct {
	// DefaultResourceQuotas specify resource limits applied to new workspaces.
	DefaultResourceQuotas map[WorkspaceType]map[string]string `json:"defaultResourceQuotas,omitempty"`
	
	// CacheConfiguration controls caching behavior for workspace operations.
	CacheConfiguration *WorkspaceCacheConfig `json:"cacheConfiguration,omitempty"`
	
	// SecurityPolicies define access control templates for workspace types.
	SecurityPolicies map[WorkspaceType]*authorization.WorkspaceAccessPolicy `json:"securityPolicies,omitempty"`
}

// WorkspaceCacheConfig controls caching behavior for workspace operations.
// Proper configuration is essential for performance in multi-tenant environments.
type WorkspaceCacheConfig struct {
	// TTL specifies how long workspace metadata is cached.
	TTL time.Duration `json:"ttl"`
	
	// MaxSize limits the number of workspaces cached in memory.
	MaxSize int `json:"maxSize"`
	
	// RefreshInterval controls periodic cache refresh for active workspaces.
	RefreshInterval time.Duration `json:"refreshInterval"`
}

// WorkspaceClient provides Kubernetes client interfaces scoped to a specific workspace.
// This is the primary mechanism for performing resource operations within a workspace.
type WorkspaceClient interface {
	// Config returns the REST configuration for this workspace.
	// Used for creating specialized clients or direct REST operations.
	Config() *rest.Config
	
	// Dynamic returns a dynamic client scoped to this workspace.
	// Supports operations on arbitrary resource types.
	Dynamic() dynamic.Interface
	
	// Discovery returns a discovery client for API resource introspection.
	// Used to determine what resources are available in this workspace.
	Discovery() discovery.DiscoveryInterface
	
	// LogicalCluster returns the logical cluster for this workspace.
	// Essential for KCP-aware operations and multi-cluster scenarios.
	LogicalCluster() logicalcluster.Name
}