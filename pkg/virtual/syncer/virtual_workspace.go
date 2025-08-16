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
	"strings"

	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// Context keys for syncer identity
type contextKey int

const (
	syncerIdentityKey contextKey = iota
)

// syncerIdentity represents the identity of a syncer in the context
type syncerIdentity struct {
	syncerID  string
	workspace string
}

// AuthConfig contains configuration for syncer authentication and authorization.
type AuthConfig struct {
	// ValidateCertificate validates a syncer's client certificate
	ValidateCertificate func(userInfo user.Info) error

	// GetSyncTargetForSyncer retrieves the SyncTarget for a syncer
	GetSyncTargetForSyncer func(syncerID, workspace string) (*workloadv1alpha1.SyncTarget, error)
}

// SyncerVirtualWorkspace implements the virtual workspace for syncers
type SyncerVirtualWorkspace struct {
	authConfig *AuthConfig
}

// NewSyncerVirtualWorkspace creates a new syncer virtual workspace.
func NewSyncerVirtualWorkspace(authConfig *AuthConfig) (*SyncerVirtualWorkspace, error) {
	if authConfig == nil {
		return nil, fmt.Errorf("auth config is required")
	}

	if authConfig.ValidateCertificate == nil {
		return nil, fmt.Errorf("certificate validator is required")
	}

	if authConfig.GetSyncTargetForSyncer == nil {
		return nil, fmt.Errorf("sync target resolver is required")
	}

	return &SyncerVirtualWorkspace{
		authConfig: authConfig,
	}, nil
}

// ResolveRootPath parses a syncer URL path and extracts syncer identity.
// Expected format: /services/syncer/{syncer-id}/clusters/{workspace}[/{remainder}]
func (w *SyncerVirtualWorkspace) ResolveRootPath(urlPath string, ctx context.Context) (bool, string, context.Context) {
	const pathPrefix = "/services/syncer/"
	
	if !strings.HasPrefix(urlPath, pathPrefix) {
		return false, "", ctx
	}

	// Remove the prefix
	remainder := urlPath[len(pathPrefix):]
	
	// Split into components: {syncer-id}/clusters/{workspace}[/{remainder}]
	parts := strings.Split(remainder, "/")
	
	if len(parts) < 3 {
		return false, "", ctx
	}
	
	syncerID := parts[0]
	if syncerID == "" {
		return false, "", ctx
	}
	
	if parts[1] != "clusters" {
		return false, "", ctx
	}
	
	workspace := parts[2]
	if workspace == "" {
		return false, "", ctx
	}

	// Construct the prefix that should be stripped from requests
	prefix := fmt.Sprintf("%s%s/clusters/%s", pathPrefix, syncerID, workspace)
	
	// Add syncer identity to context
	newCtx := withSyncerIdentity(ctx, syncerID, workspace)
	
	klog.V(4).InfoS("Resolved syncer path", 
		"syncerID", syncerID, 
		"workspace", workspace,
		"prefix", prefix)
	
	return true, prefix, newCtx
}

// Authorize checks if the syncer is authorized to access a resource.
func (w *SyncerVirtualWorkspace) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	syncerID, workspace, ok := extractSyncerIdentity(ctx)
	if !ok {
		return authorizer.DecisionDeny, "no syncer identity", nil
	}

	// Validate the certificate
	userInfo := attrs.GetUser()
	if err := w.authConfig.ValidateCertificate(userInfo); err != nil {
		klog.V(4).InfoS("Certificate validation failed", 
			"syncerID", syncerID, 
			"error", err)
		return authorizer.DecisionDeny, "certificate validation failed", nil
	}

	// Get the SyncTarget for authorization
	syncTarget, err := w.authConfig.GetSyncTargetForSyncer(syncerID, workspace)
	if err != nil {
		klog.V(4).InfoS("Failed to get sync target", 
			"syncerID", syncerID, 
			"error", err)
		return authorizer.DecisionDeny, "sync target lookup failed", nil
	}

	if syncTarget == nil {
		klog.V(4).InfoS("Sync target not found", "syncerID", syncerID)
		return authorizer.DecisionDeny, "sync target not found", nil
	}

	// Check if the resource is supported
	resource := attrs.GetResource()
	if !w.isResourceSupported(syncTarget, resource) {
		klog.V(4).InfoS("Resource not supported", 
			"syncerID", syncerID, 
			"resource", resource)
		return authorizer.DecisionDeny, "resource not supported", nil
	}

	klog.V(4).InfoS("Authorization granted", 
		"syncerID", syncerID, 
		"resource", resource)
	
	return authorizer.DecisionAllow, "", nil
}

// IsReady checks if the virtual workspace is ready to serve requests.
func (w *SyncerVirtualWorkspace) IsReady() error {
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

// withSyncerIdentity adds syncer identity to the context.
func withSyncerIdentity(ctx context.Context, syncerID, workspace string) context.Context {
	identity := &syncerIdentity{
		syncerID:  syncerID,
		workspace: workspace,
	}
	return context.WithValue(ctx, syncerIdentityKey, identity)
}

// extractSyncerIdentity retrieves syncer identity from the context.
func extractSyncerIdentity(ctx context.Context) (syncerID, workspace string, ok bool) {
	identity, ok := ctx.Value(syncerIdentityKey).(*syncerIdentity)
	if !ok {
		return "", "", false
	}
	return identity.syncerID, identity.workspace, true
}

// isResourceSupported checks if a resource type is supported by the sync target.
func (w *SyncerVirtualWorkspace) isResourceSupported(syncTarget *workloadv1alpha1.SyncTarget, resource string) bool {
	// Check if the resource is in the supported list
	for _, supportedResource := range syncTarget.Spec.SupportedResourceTypes {
		if supportedResource == resource {
			return true
		}
	}
	
	// Always allow synctargets resource itself
	if resource == "synctargets" {
		return true
	}
	
	return false
}