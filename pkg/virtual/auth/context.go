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

package auth

import (
	"context"

	"k8s.io/apiserver/pkg/authentication/user"
)

// contextKey is the type for context keys to avoid collisions.
type contextKey string

const (
	// UserContextKey is the context key for storing user information.
	UserContextKey contextKey = "virtual-workspace-user"

	// WorkspaceContextKey is the context key for storing workspace information.
	WorkspaceContextKey contextKey = "virtual-workspace"

	// DecisionContextKey is the context key for storing authorization decisions.
	DecisionContextKey contextKey = "virtual-workspace-auth-decision"

	// ProviderContextKey is the context key for storing the active provider.
	ProviderContextKey contextKey = "virtual-workspace-auth-provider"
)

// WithUser adds user information to the context.
// This should be called early in the request processing pipeline
// to ensure user context is available for authorization.
func WithUser(ctx context.Context, userInfo user.Info) context.Context {
	return context.WithValue(ctx, UserContextKey, userInfo)
}

// GetUser retrieves user information from the context.
// Returns the user info and a boolean indicating if it was found.
func GetUser(ctx context.Context) (user.Info, bool) {
	userInfo, ok := ctx.Value(UserContextKey).(user.Info)
	return userInfo, ok
}

// WithWorkspace adds workspace information to the context.
// This identifies which virtual workspace is being accessed.
func WithWorkspace(ctx context.Context, workspace string) context.Context {
	return context.WithValue(ctx, WorkspaceContextKey, workspace)
}

// GetWorkspace retrieves workspace information from the context.
// Returns the workspace name and a boolean indicating if it was found.
func GetWorkspace(ctx context.Context) (string, bool) {
	workspace, ok := ctx.Value(WorkspaceContextKey).(string)
	return workspace, ok
}

// WithDecision adds an authorization decision to the context.
// This allows components to access the authorization result
// without re-evaluating permissions.
func WithDecision(ctx context.Context, decision *Decision) context.Context {
	return context.WithValue(ctx, DecisionContextKey, decision)
}

// GetDecision retrieves the authorization decision from the context.
// Returns the decision and a boolean indicating if it was found.
func GetDecision(ctx context.Context) (*Decision, bool) {
	decision, ok := ctx.Value(DecisionContextKey).(*Decision)
	return decision, ok
}

// WithProvider adds the authorization provider to the context.
// This allows tracking which provider made the authorization decision.
func WithProvider(ctx context.Context, provider Provider) context.Context {
	return context.WithValue(ctx, ProviderContextKey, provider)
}

// GetProvider retrieves the authorization provider from the context.
// Returns the provider and a boolean indicating if it was found.
func GetProvider(ctx context.Context) (Provider, bool) {
	provider, ok := ctx.Value(ProviderContextKey).(Provider)
	return provider, ok
}

// ExtractAuthInfo extracts all authentication and authorization information from the context.
// This is a convenience method that gathers all auth-related data in one call.
func ExtractAuthInfo(ctx context.Context) *AuthInfo {
	info := &AuthInfo{}

	// Extract user information
	if userInfo, ok := GetUser(ctx); ok {
		info.User = userInfo.GetName()
		info.Groups = userInfo.GetGroups()
		info.Extra = userInfo.GetExtra()
		info.UID = string(userInfo.GetUID())
	}

	// Extract workspace
	if workspace, ok := GetWorkspace(ctx); ok {
		info.Workspace = workspace
	}

	// Extract authorization decision
	if decision, ok := GetDecision(ctx); ok {
		info.Decision = decision
	}

	// Extract provider information
	if provider, ok := GetProvider(ctx); ok {
		info.ProviderName = provider.Name()
	}

	return info
}

// AuthInfo contains all authentication and authorization information
// extracted from a request context.
type AuthInfo struct {
	// User name of the authenticated user
	User string

	// Groups the user belongs to
	Groups []string

	// Extra contains additional user attributes
	Extra map[string][]string

	// UID is the unique identifier of the user
	UID string

	// Workspace being accessed
	Workspace string

	// Decision contains the authorization result
	Decision *Decision

	// ProviderName identifies which authorization provider was used
	ProviderName string
}

// IsAuthenticated returns true if the context contains valid user information.
func (ai *AuthInfo) IsAuthenticated() bool {
	return ai.User != ""
}

// IsAuthorized returns true if the authorization decision allows the request.
func (ai *AuthInfo) IsAuthorized() bool {
	return ai.Decision != nil && ai.Decision.Allowed
}

// HasGroup returns true if the user belongs to the specified group.
func (ai *AuthInfo) HasGroup(group string) bool {
	for _, g := range ai.Groups {
		if g == group {
			return true
		}
	}
	return false
}

// GetExtraValue returns the first value for the given extra key.
func (ai *AuthInfo) GetExtraValue(key string) string {
	if values, ok := ai.Extra[key]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}