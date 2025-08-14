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
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog/v2"
)

// contextKey is a custom type to avoid context key collisions
type contextKey string

const (
	syncerIDContextKey    contextKey = "syncerID"
	workspaceContextKey   contextKey = "workspace"
)

// SyncTarget represents a minimal sync target for testing purposes
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	
	Spec SyncTargetSpec `json:"spec,omitempty"`
}

// SyncTargetSpec defines the desired state of SyncTarget
type SyncTargetSpec struct {
	SupportedResourceTypes []string `json:"supportedResourceTypes,omitempty"`
}

// AuthConfig provides authentication and authorization configuration for the syncer virtual workspace.
// It defines the callback functions needed to validate syncer certificates and resolve sync targets.
//
// ValidateCertificate should verify that the provided user info represents a valid syncer certificate,
// checking the certificate chain, expiration, and syncer identity.
//
// GetSyncTargetForSyncer should return the SyncTarget resource for the given syncer and workspace,
// which contains the list of supported resource types and configuration for that syncer.
type AuthConfig struct {
	ValidateCertificate    func(user.Info) error
	GetSyncTargetForSyncer func(syncerID, workspace string) (*SyncTarget, error)
}

// SyncerVirtualWorkspace implements a virtual workspace for syncer operations with REST storage capabilities.
// 
// This provides the foundational REST storage layer for the virtual workspace, enabling syncers to
// interact with KCP through a standardized API while maintaining proper workspace isolation and security.
//
// The virtual workspace handles:
// - URL path resolution and parsing for syncer-specific endpoints
// - Authentication and authorization of syncer requests  
// - Resource transformation between KCP and syncer formats
// - Metrics collection and retry logic for reliability
//
// Example usage:
//
//	authConfig := &AuthConfig{
//		ValidateCertificate: func(userInfo user.Info) error {
//			// Validate syncer certificate
//			return validateSyncerCert(userInfo)
//		},
//		GetSyncTargetForSyncer: func(syncerID, workspace string) (*SyncTarget, error) {
//			// Resolve sync target from syncer ID and workspace
//			return getSyncTarget(syncerID, workspace)
//		},
//	}
//	
//	workspace, err := NewSyncerVirtualWorkspace(authConfig)
//	if err != nil {
//		return fmt.Errorf("failed to create virtual workspace: %w", err)
//	}
//	
//	// Use in KCP virtual workspace framework
//	if accepted, prefix, ctx := workspace.ResolveRootPath(requestPath, ctx); accepted {
//		// Handle syncer request with proper context
//	}
type SyncerVirtualWorkspace struct {
	authConfig    *AuthConfig
	pathRegex     *regexp.Regexp
	transformers  map[string]*ResourceTransformer
	transformerMu sync.RWMutex
}

// NewSyncerVirtualWorkspace creates a new syncer virtual workspace with REST capabilities
func NewSyncerVirtualWorkspace(authConfig *AuthConfig) (*SyncerVirtualWorkspace, error) {
	if authConfig == nil {
		return nil, fmt.Errorf("auth config cannot be nil")
	}
	if authConfig.ValidateCertificate == nil {
		return nil, fmt.Errorf("certificate validator cannot be nil")
	}
	if authConfig.GetSyncTargetForSyncer == nil {
		return nil, fmt.Errorf("sync target resolver cannot be nil")
	}

	// Compile regex for syncer path parsing: /services/syncer/{syncerID}/clusters/{workspace}/...
	pathRegex, err := regexp.Compile(`^/services/syncer/([^/]+)/clusters/([^/]+)(/.*)?$`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile path regex: %w", err)
	}

	return &SyncerVirtualWorkspace{
		authConfig:   authConfig,
		pathRegex:    pathRegex,
		transformers: make(map[string]*ResourceTransformer),
	}, nil
}

// ResolveRootPath parses the incoming URL path and extracts syncer and workspace information
func (s *SyncerVirtualWorkspace) ResolveRootPath(urlPath string, ctx context.Context) (bool, string, context.Context) {
	matches := s.pathRegex.FindStringSubmatch(urlPath)
	if matches == nil {
		return false, "", ctx
	}

	syncerID := matches[1]
	workspace := matches[2]
	
	// Validate syncer ID and workspace are not empty
	if syncerID == "" || workspace == "" {
		return false, "", ctx
	}

	// Create the prefix that should be stripped from further requests
	prefix := fmt.Sprintf("/services/syncer/%s/clusters/%s", syncerID, workspace)
	
	// Add syncer identity to context for authorization and transformation
	newCtx := withSyncerIdentity(ctx, syncerID, workspace)

	klog.V(4).InfoS("Resolved syncer virtual workspace path",
		"syncerID", syncerID,
		"workspace", workspace,
		"prefix", prefix)

	return true, prefix, newCtx
}

// Authorize validates that the request is authorized for the specific syncer and resource
func (s *SyncerVirtualWorkspace) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	syncerID, workspace, ok := extractSyncerIdentity(ctx)
	if !ok {
		return authorizer.DecisionDeny, "syncer identity not found in context", nil
	}

	// Validate certificate and user identity
	if err := s.authConfig.ValidateCertificate(attrs.GetUser()); err != nil {
		klog.V(2).InfoS("Certificate validation failed", 
			"syncerID", syncerID, 
			"user", attrs.GetUser().GetName(),
			"error", err)
		return authorizer.DecisionDeny, fmt.Sprintf("certificate validation failed: %v", err), nil
	}

	// Validate user identity matches syncer pattern
	expectedUser := fmt.Sprintf("system:syncer:%s", syncerID)
	if attrs.GetUser().GetName() != expectedUser {
		return authorizer.DecisionDeny, fmt.Sprintf("invalid user for syncer %s", syncerID), nil
	}

	// Get sync target for resource validation
	syncTarget, err := s.authConfig.GetSyncTargetForSyncer(syncerID, workspace)
	if err != nil {
		klog.V(2).InfoS("Failed to get sync target",
			"syncerID", syncerID,
			"workspace", workspace,
			"error", err)
		return authorizer.DecisionDeny, fmt.Sprintf("sync target not found: %v", err), nil
	}

	if syncTarget == nil {
		return authorizer.DecisionDeny, "sync target not found", nil
	}

	// Validate resource is supported by this syncer
	resource := attrs.GetResource()
	if !isResourceSupported(syncTarget, resource) {
		return authorizer.DecisionDeny, fmt.Sprintf("resource %s not supported by syncer", resource), nil
	}

	klog.V(4).InfoS("Authorized syncer request",
		"syncerID", syncerID,
		"workspace", workspace,
		"resource", resource,
		"verb", attrs.GetVerb())

	return authorizer.DecisionAllow, "authorized", nil
}

// IsReady checks if the virtual workspace is ready to handle requests
func (s *SyncerVirtualWorkspace) IsReady() error {
	if s.authConfig == nil {
		return fmt.Errorf("auth config not configured")
	}
	if s.authConfig.ValidateCertificate == nil {
		return fmt.Errorf("certificate validator not configured")
	}
	if s.authConfig.GetSyncTargetForSyncer == nil {
		return fmt.Errorf("sync target resolver not configured")
	}
	return nil
}

// Helper functions

// withSyncerIdentity adds syncer identity to the context
func withSyncerIdentity(ctx context.Context, syncerID, workspace string) context.Context {
	ctx = context.WithValue(ctx, syncerIDContextKey, syncerID)
	ctx = context.WithValue(ctx, workspaceContextKey, workspace)
	return ctx
}

// extractSyncerIdentity extracts syncer identity from the context
func extractSyncerIdentity(ctx context.Context) (syncerID, workspace string, ok bool) {
	syncerIDValue := ctx.Value(syncerIDContextKey)
	workspaceValue := ctx.Value(workspaceContextKey)
	
	if syncerIDValue == nil || workspaceValue == nil {
		return "", "", false
	}
	
	syncerID, ok1 := syncerIDValue.(string)
	workspace, ok2 := workspaceValue.(string)
	
	return syncerID, workspace, ok1 && ok2
}

// isResourceSupported checks if a resource is supported by the sync target
func isResourceSupported(syncTarget *SyncTarget, resource string) bool {
	for _, supportedResource := range syncTarget.Spec.SupportedResourceTypes {
		if supportedResource == resource {
			return true
		}
	}
	return false
}

// getOrCreateTransformer gets or creates a resource transformer for a syncer
func (s *SyncerVirtualWorkspace) getOrCreateTransformer(syncerID, workspace string) *ResourceTransformer {
	s.transformerMu.Lock()
	defer s.transformerMu.Unlock()

	key := fmt.Sprintf("%s:%s", syncerID, workspace)
	transformer, exists := s.transformers[key]
	if !exists {
		transformer = NewResourceTransformer(syncerID, workspace)
		s.transformers[key] = transformer
	}
	return transformer
}

// NewDefaultAuthConfig creates a default auth config for testing
func NewDefaultAuthConfig() *testAuthConfig {
	return &testAuthConfig{
		syncTargets: make(map[string]*SyncTarget),
	}
}

// testAuthConfig provides a test implementation of AuthConfig
type testAuthConfig struct {
	syncTargets map[string]*SyncTarget
	mu          sync.RWMutex
}

func (t *testAuthConfig) ValidateCertificate(user.Info) error {
	return nil
}

func (t *testAuthConfig) GetSyncTargetForSyncer(syncerID, workspace string) (*SyncTarget, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	syncTarget, exists := t.syncTargets[syncerID]
	if !exists {
		return nil, fmt.Errorf("sync target not found for syncer %s", syncerID)
	}
	return syncTarget, nil
}

func (t *testAuthConfig) RegisterSyncTarget(syncerID string, syncTarget *SyncTarget) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.syncTargets[syncerID] = syncTarget
}