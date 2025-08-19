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

package interfaces

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"github.com/kcp-dev/logicalcluster/v3"
)

// TMCVirtualWorkspaceInterface defines the core interface for TMC virtual workspace operations.
type TMCVirtualWorkspaceInterface interface {
	// GetWorkspace returns the logical cluster name for this virtual workspace
	GetWorkspace() logicalcluster.Name
	
	// IsReady returns whether the virtual workspace is ready to serve requests
	IsReady(ctx context.Context) (bool, error)
	
	// GetSupportedResources returns the list of TMC resources supported by this virtual workspace
	GetSupportedResources() []schema.GroupVersionResource
	
	// Shutdown gracefully shuts down the virtual workspace
	Shutdown(ctx context.Context) error
}

// TMCResourceHandler defines the interface for handling TMC resource operations.
type TMCResourceHandler interface {
	// Get retrieves a TMC resource by name
	Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
	
	// List retrieves TMC resources matching the given options
	List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error)
	
	// Create creates a new TMC resource
	Create(ctx context.Context, obj runtime.Object, options *metav1.CreateOptions) (runtime.Object, error)
	
	// Update updates an existing TMC resource
	Update(ctx context.Context, obj runtime.Object, options *metav1.UpdateOptions) (runtime.Object, error)
	
	// Delete deletes a TMC resource
	Delete(ctx context.Context, name string, options *metav1.DeleteOptions) error
	
	// GetGroupVersionResource returns the GVR for this resource handler
	GetGroupVersionResource() schema.GroupVersionResource
}

// TMCAuthenticationProvider defines the interface for TMC virtual workspace authentication.
type TMCAuthenticationProvider interface {
	// AuthenticateUser validates user access to TMC virtual workspace
	AuthenticateUser(ctx context.Context, userInfo user.Info) (*TMCUserInfo, error)
	
	// GetUserWorkspaces returns the workspaces accessible to the user
	GetUserWorkspaces(ctx context.Context, userInfo user.Info) ([]logicalcluster.Name, error)
	
	// ValidateWorkspaceAccess checks if user can access the specified workspace
	ValidateWorkspaceAccess(ctx context.Context, userInfo user.Info, workspace logicalcluster.Name) error
}

// TMCAuthorizationProvider defines the interface for TMC virtual workspace authorization.
type TMCAuthorizationProvider interface {
	// Authorize checks if the user is authorized to perform the specified action
	Authorize(ctx context.Context, attr *TMCAuthorizationAttributes) (authorizer.Decision, string, error)
	
	// GetAllowedActions returns the actions allowed for the user on the resource
	GetAllowedActions(ctx context.Context, userInfo user.Info, gvr schema.GroupVersionResource) ([]string, error)
}

// TMCDiscoveryProvider defines the interface for TMC API discovery.
type TMCDiscoveryProvider interface {
	// GetServerGroups returns the API groups supported by TMC virtual workspace
	GetServerGroups(ctx context.Context) (*metav1.APIGroupList, error)
	
	// GetServerResources returns the resources for a specific group version
	GetServerResources(ctx context.Context, groupVersion string) (*metav1.APIResourceList, error)
	
	// GetServerVersion returns the server version information
	GetServerVersion() *version.Info
}

// TMCStorageProvider defines the interface for TMC resource storage operations.
type TMCStorageProvider interface {
	// GetStorage returns a storage instance for the specified resource
	GetStorage(gvr schema.GroupVersionResource) (TMCResourceStorage, error)
	
	// ListStorages returns all available storage instances
	ListStorages() map[schema.GroupVersionResource]TMCResourceStorage
	
	// IsResourceSupported checks if the resource is supported
	IsResourceSupported(gvr schema.GroupVersionResource) bool
}

// TMCResourceStorage defines the interface for individual resource storage.
type TMCResourceStorage interface {
	// New creates a new empty object for this resource type
	New() runtime.Object
	
	// Get retrieves a resource by name
	Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
	
	// List retrieves resources matching the given options
	List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error)
	
	// Create creates a new resource
	Create(ctx context.Context, obj runtime.Object, options *metav1.CreateOptions) (runtime.Object, error)
	
	// Update updates an existing resource
	Update(ctx context.Context, name string, obj runtime.Object, options *metav1.UpdateOptions) (runtime.Object, error)
	
	// Delete deletes a resource
	Delete(ctx context.Context, name string, options *metav1.DeleteOptions) error
	
	// GetGroupVersionResource returns the GVR for this storage
	GetGroupVersionResource() schema.GroupVersionResource
}

// TMCUserInfo contains TMC-specific user information.
type TMCUserInfo struct {
	// User is the basic user information
	User user.Info
	
	// TMCPermissions contains TMC-specific permissions
	TMCPermissions *TMCPermissions
	
	// AccessibleWorkspaces lists workspaces the user can access
	AccessibleWorkspaces []logicalcluster.Name
}

// TMCPermissions defines TMC-specific user permissions.
type TMCPermissions struct {
	// CanManageClusters indicates if user can manage cluster registrations
	CanManageClusters bool
	
	// CanCreatePlacements indicates if user can create workload placements
	CanCreatePlacements bool
	
	// CanViewSyncers indicates if user can view syncer configurations
	CanViewSyncers bool
	
	// CanManageWorkloadSync indicates if user can manage workload synchronization
	CanManageWorkloadSync bool
	
	// AdminWorkspaces lists workspaces where user has admin privileges
	AdminWorkspaces []logicalcluster.Name
}

// TMCAuthorizationAttributes contains authorization information for TMC operations.
type TMCAuthorizationAttributes struct {
	// User is the user making the request
	User user.Info
	
	// Verb is the operation being performed
	Verb string
	
	// Resource is the TMC resource being accessed
	Resource string
	
	// ResourceName is the name of the specific resource instance
	ResourceName string
	
	// Namespace is the namespace (for namespaced resources)
	Namespace string
	
	// Workspace is the target workspace
	Workspace logicalcluster.Name
	
	// GroupVersionResource is the full GVR
	GroupVersionResource schema.GroupVersionResource
}

// TMCVirtualWorkspaceConfig defines configuration for TMC virtual workspaces.
type TMCVirtualWorkspaceConfig struct {
	// Workspace is the target logical cluster
	Workspace logicalcluster.Name
	
	// PathPrefix is the URL path prefix
	PathPrefix string
	
	// EnabledResources specifies which resources are enabled
	EnabledResources []schema.GroupVersionResource
	
	// AuthenticationProvider provides authentication services
	AuthenticationProvider TMCAuthenticationProvider
	
	// AuthorizationProvider provides authorization services
	AuthorizationProvider TMCAuthorizationProvider
	
	// DiscoveryProvider provides API discovery services
	DiscoveryProvider TMCDiscoveryProvider
	
	// StorageProvider provides resource storage services
	StorageProvider TMCStorageProvider
}

// TMCVirtualWorkspaceStatus represents the status of a TMC virtual workspace.
type TMCVirtualWorkspaceStatus struct {
	// Ready indicates if the virtual workspace is ready
	Ready bool
	
	// Reason provides the reason for the current status
	Reason string
	
	// Message provides additional status information
	Message string
	
	// ActiveConnections tracks active client connections
	ActiveConnections int
	
	// SupportedResources lists currently supported resources
	SupportedResources []schema.GroupVersionResource
	
	// LastError contains the last error encountered
	LastError error
}

// Default implementations and constants

const (
	// TMCVirtualWorkspacePathPrefix is the default path prefix for TMC virtual workspaces
	TMCVirtualWorkspacePathPrefix = "/services/tmc"
	
	// TMCAPIGroup is the API group for TMC resources
	TMCAPIGroup = "tmc.kcp.io"
	
	// TMCAPIVersion is the API version for TMC resources
	TMCAPIVersion = "v1alpha1"
)

// DefaultTMCResources returns the default set of TMC resources.
func DefaultTMCResources() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: TMCAPIGroup, Version: TMCAPIVersion, Resource: "clusterregistrations"},
		{Group: TMCAPIGroup, Version: TMCAPIVersion, Resource: "workloadplacements"},
		{Group: TMCAPIGroup, Version: TMCAPIVersion, Resource: "syncerconfigs"},
		{Group: TMCAPIGroup, Version: TMCAPIVersion, Resource: "workloadsyncs"},
		{Group: TMCAPIGroup, Version: TMCAPIVersion, Resource: "syncertunnels"},
	}
}