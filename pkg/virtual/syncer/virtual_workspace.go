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
	"fmt"
	"regexp"

	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	reststorage "k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/virtual/framework/fixedgvs"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// syncerPathPattern matches syncer paths like /services/syncer/<syncer-id>/clusters/<workspace>/...
var syncerPathPattern = regexp.MustCompile(`^/services/syncer/([^/]+)/clusters/([^/]+)(/.*)?$`)

// SyncerVirtualWorkspace implements a virtual workspace for syncer endpoints.
// It provides the API surface that syncers use to connect to KCP, handling:
//   - Certificate-based authentication
//   - Workspace isolation and routing
//   - Resource transformation between KCP and syncer formats
//   - API discovery and permissions
type SyncerVirtualWorkspace struct {
	// fixedGVs provides the underlying virtual workspace framework
	fixedGVs *fixedgvs.FixedGroupVersionsVirtualWorkspace
	
	// authConfig holds syncer authentication configuration
	authConfig *AuthConfig
}

// AuthConfig contains configuration for syncer authentication and authorization.
type AuthConfig struct {
	// ValidateCertificate validates syncer client certificates
	ValidateCertificate func(user.Info) error
	
	// GetSyncTargetForSyncer maps syncer identity to SyncTarget
	GetSyncTargetForSyncer func(syncerID string, workspace string) (*workloadv1alpha1.SyncTarget, error)
}

// NewSyncerVirtualWorkspace creates a new syncer virtual workspace.
func NewSyncerVirtualWorkspace(authConfig *AuthConfig) (*SyncerVirtualWorkspace, error) {
	if authConfig == nil {
		return nil, fmt.Errorf("authConfig is required")
	}

	workspace := &SyncerVirtualWorkspace{
		authConfig: authConfig,
	}

	// Create the underlying fixed group versions virtual workspace
	fixedGVs := &fixedgvs.FixedGroupVersionsVirtualWorkspace{
		RootPathResolver: workspace,
		Authorizer:       workspace,
		ReadyChecker:     workspace,
		GroupVersionAPISets: []fixedgvs.GroupVersionAPISet{
			{
				GroupVersion: workloadv1alpha1.SchemeGroupVersion,
				AddToScheme:  workloadv1alpha1.AddToScheme,
				BootstrapRestResources: workspace.bootstrapSyncerResources,
			},
		},
	}

	workspace.fixedGVs = fixedGVs
	return workspace, nil
}

// ResolveRootPath implements the RootPathResolver interface.
// It matches paths of the form /services/syncer/<syncer-id>/clusters/<workspace>/...
func (w *SyncerVirtualWorkspace) ResolveRootPath(urlPath string, requestContext context.Context) (bool, string, context.Context) {
	matches := syncerPathPattern.FindStringSubmatch(urlPath)
	if matches == nil {
		return false, "", requestContext
	}

	syncerID := matches[1]
	workspace := matches[2]
	remainder := matches[3]

	klog.V(4).InfoS("Resolving syncer path", "syncerID", syncerID, "workspace", workspace, "remainder", remainder)

	// Add syncer identity to context
	ctx := withSyncerIdentity(requestContext, syncerID, workspace)
	
	// Prefix to strip includes everything before the kubernetes-like API path
	prefixToStrip := fmt.Sprintf("/services/syncer/%s/clusters/%s", syncerID, workspace)
	
	return true, prefixToStrip, ctx
}

// Authorize implements the authorizer.Authorizer interface.
// It validates that the syncer has permission to access the requested workspace and resources.
func (w *SyncerVirtualWorkspace) Authorize(ctx context.Context, attr authorizer.Attributes) (authorizer.Decision, string, error) {
	syncerID, workspace, ok := extractSyncerIdentity(ctx)
	if !ok {
		return authorizer.DecisionDeny, "no syncer identity in context", nil
	}

	// Validate user certificate
	user := attr.GetUser()
	if user == nil {
		return authorizer.DecisionDeny, "no user information", nil
	}

	if err := w.authConfig.ValidateCertificate(user); err != nil {
		klog.V(4).InfoS("Certificate validation failed", "syncerID", syncerID, "error", err)
		return authorizer.DecisionDeny, fmt.Sprintf("certificate validation failed: %v", err), nil
	}

	// Get the SyncTarget for this syncer
	syncTarget, err := w.authConfig.GetSyncTargetForSyncer(syncerID, workspace)
	if err != nil {
		klog.V(4).InfoS("Failed to get SyncTarget", "syncerID", syncerID, "workspace", workspace, "error", err)
		return authorizer.DecisionDeny, fmt.Sprintf("failed to get sync target: %v", err), nil
	}

	if syncTarget == nil {
		return authorizer.DecisionDeny, "sync target not found", nil
	}

	// Validate that syncer has access to the requested resource type
	resource := attr.GetResource()
	if !w.isResourceAllowed(syncTarget, resource) {
		return authorizer.DecisionDeny, fmt.Sprintf("resource %s not allowed for sync target", resource), nil
	}

	klog.V(4).InfoS("Authorizing syncer request", "syncerID", syncerID, "workspace", workspace, "resource", resource, "verb", attr.GetVerb())
	return authorizer.DecisionAllow, "", nil
}

// IsReady implements the ReadyChecker interface.
func (w *SyncerVirtualWorkspace) IsReady() error {
	// Basic validation - ensure auth config is present
	if w.authConfig == nil {
		return fmt.Errorf("auth config not initialized")
	}
	
	if w.authConfig.ValidateCertificate == nil {
		return fmt.Errorf("certificate validator not configured")
	}
	
	if w.authConfig.GetSyncTargetForSyncer == nil {
		return fmt.Errorf("sync target resolver not configured")
	}
	
	return nil
}

// Register implements the VirtualWorkspace interface by delegating to the fixed GVs workspace.
func (w *SyncerVirtualWorkspace) Register(name string, rootAPIServerConfig genericapiserver.CompletedConfig, delegateAPIServer genericapiserver.DelegationTarget) (genericapiserver.DelegationTarget, error) {
	return w.fixedGVs.Register(name, rootAPIServerConfig, delegateAPIServer)
}

// isResourceAllowed checks if the sync target supports the requested resource type.
func (w *SyncerVirtualWorkspace) isResourceAllowed(syncTarget *workloadv1alpha1.SyncTarget, resource string) bool {
	if syncTarget.Spec.SupportedResourceTypes == nil || len(syncTarget.Spec.SupportedResourceTypes) == 0 {
		// No restrictions - all resources allowed
		return true
	}

	for _, supportedType := range syncTarget.Spec.SupportedResourceTypes {
		if supportedType == resource {
			return true
		}
	}

	return false
}

// bootstrapSyncerResources sets up the REST storage for syncer resources.
func (w *SyncerVirtualWorkspace) bootstrapSyncerResources(rootAPIServerConfig genericapiserver.CompletedConfig) (map[string]fixedgvs.RestStorageBuilder, error) {
	builders := make(map[string]fixedgvs.RestStorageBuilder)
	
	// Add storage builders for resources that syncers need to access
	builders["synctargets"] = func(apiGroupAPIServerConfig genericapiserver.CompletedConfig) (reststorage.Storage, error) {
		return NewSyncTargetStorage(w.authConfig), nil
	}

	// Additional resource storages can be added here as needed
	
	return builders, nil
}