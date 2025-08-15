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
	"crypto/tls"
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
)

// AuthContext contains authentication and authorization context for tunnel operations
type AuthContext struct {
	// Workspace identifies the logical cluster context
	Workspace logicalcluster.Name
	
	// UserInfo contains authenticated user information
	UserInfo *UserInfo
	
	// Permissions contains effective permissions for the user in workspace
	Permissions []Permission
	
	// AuthTime records when authentication occurred
	AuthTime time.Time
	
	// ExpiryTime indicates when authentication expires
	ExpiryTime time.Time
	
	// SessionID uniquely identifies the authentication session
	SessionID string
	
	// ClientIP contains the client IP address
	ClientIP string
	
	// UserAgent contains the client user agent string
	UserAgent string
	
	// Metadata contains additional authentication context
	Metadata map[string]interface{}
}

// UserInfo contains authenticated user identity information
type UserInfo struct {
	// Username identifies the authenticated user
	Username string
	
	// Groups contains group memberships for the user
	Groups []string
	
	// Extra contains additional user attributes
	Extra map[string][]string
	
	// UID provides a unique identifier for the user
	UID string
}

// Permission represents an authorization permission for tunnel operations
type Permission struct {
	// Resource specifies the resource type (e.g., "tunnels", "connections")
	Resource string
	
	// Verb specifies the operation (e.g., "create", "read", "update", "delete")
	Verb string
	
	// Namespace limits permission to specific namespace (empty for cluster-wide)
	Namespace string
	
	// ResourceName limits permission to specific resource instance
	ResourceName string
}

// TunnelAuthenticator handles authentication for tunnel connections.
// Implementations must be thread-safe and support concurrent authentication requests.
type TunnelAuthenticator interface {
	// Authenticate validates credentials and returns authentication context.
	// Returns ErrAuthenticationFailed for invalid credentials or other auth errors.
	//
	// The returned context contains user identity and permissions within
	// the specified workspace.
	Authenticate(ctx context.Context, credentials AuthCredentials, workspace logicalcluster.Name) (*AuthContext, error)
	
	// RefreshAuth renews authentication using refresh tokens or other renewal mechanisms.
	// Returns updated authentication context with new expiry time.
	//
	// Returns ErrAuthenticationExpired if refresh is not possible or fails.
	RefreshAuth(ctx context.Context, authCtx *AuthContext) (*AuthContext, error)
	
	// ValidateAuth checks if existing authentication context is still valid.
	// Performs lightweight validation without full re-authentication.
	//
	// Returns nil if context is valid, or appropriate error indicating the issue.
	ValidateAuth(ctx context.Context, authCtx *AuthContext) error
	
	// GetTLSConfig returns TLS configuration for secure tunnel connections.
	// Configuration includes client certificates, CA validation, and other TLS settings.
	//
	// Returns nil if TLS is not required for the authentication method.
	GetTLSConfig(credentials AuthCredentials) (*tls.Config, error)
	
	// SupportedMethods returns authentication methods supported by this authenticator
	SupportedMethods() []AuthMethod
}

// TunnelAuthorizer handles authorization for tunnel operations.
// Authorization decisions are made based on user identity and requested permissions.
type TunnelAuthorizer interface {
	// Authorize determines if a user is allowed to perform an operation.
	// Returns nil if authorized, or ErrUnauthorized with details if not.
	//
	// The decision is based on user permissions within the specified workspace
	// and the requested operation details.
	Authorize(ctx context.Context, authCtx *AuthContext, permission Permission) error
	
	// GetUserPermissions returns all permissions for a user in the specified workspace.
	// This can be used for UI permission checks and capability discovery.
	GetUserPermissions(ctx context.Context, authCtx *AuthContext) ([]Permission, error)
	
	// CanCreateTunnel checks if user can create tunnels in the specified workspace
	CanCreateTunnel(ctx context.Context, authCtx *AuthContext, workspace logicalcluster.Name) error
	
	// CanAccessTunnel checks if user can access an existing tunnel connection
	CanAccessTunnel(ctx context.Context, authCtx *AuthContext, connectionID ConnectionID) error
	
	// CanManageConnections checks if user can manage connection pools and lifecycle
	CanManageConnections(ctx context.Context, authCtx *AuthContext, workspace logicalcluster.Name) error
}

// TunnelAuthManager combines authentication and authorization for tunnel operations.
// Provides a unified interface for security operations with integrated caching and performance optimizations.
type TunnelAuthManager interface {
	// AuthenticateAndAuthorize performs both authentication and authorization in a single operation.
	// More efficient than separate calls when both are needed.
	//
	// Returns authenticated context if successful, or appropriate error.
	AuthenticateAndAuthorize(ctx context.Context, credentials AuthCredentials, workspace logicalcluster.Name, permission Permission) (*AuthContext, error)
	
	// SetAuthenticator configures the authenticator implementation to use
	SetAuthenticator(authenticator TunnelAuthenticator)
	
	// SetAuthorizer configures the authorizer implementation to use
	SetAuthorizer(authorizer TunnelAuthorizer)
	
	// GetAuthenticator returns the current authenticator implementation
	GetAuthenticator() TunnelAuthenticator
	
	// GetAuthorizer returns the current authorizer implementation  
	GetAuthorizer() TunnelAuthorizer
	
	// ClearCache clears any cached authentication/authorization decisions
	ClearCache()
	
	// GetCacheStats returns statistics about authentication/authorization cache performance
	GetCacheStats() AuthCacheStats
}

// AuthCacheStats provides information about authentication cache performance
type AuthCacheStats struct {
	// CacheHits tracks successful cache lookups
	CacheHits uint64
	
	// CacheMisses tracks cache lookups that required full authentication
	CacheMisses uint64
	
	// CacheEvictions tracks entries removed from cache
	CacheEvictions uint64
	
	// CacheSize contains current number of cached entries
	CacheSize int
	
	// HitRate provides cache hit rate as percentage
	HitRate float64
}

// AuthManagerFactory creates auth manager instances with specific configurations
type AuthManagerFactory interface {
	// CreateAuthManager creates a new authentication manager
	CreateAuthManager() TunnelAuthManager
	
	// CreateAuthenticator creates an authenticator for the specified method
	CreateAuthenticator(method AuthMethod) (TunnelAuthenticator, error)
	
	// CreateAuthorizer creates an authorizer with the specified configuration
	CreateAuthorizer() TunnelAuthorizer
}

// Authentication and authorization errors specific to advanced auth operations
var (
	// ErrAuthenticationFailed indicates invalid credentials or auth failure
	ErrAuthenticationFailed = fmt.Errorf("authentication failed")
	
	// ErrAuthenticationExpired indicates auth context has expired
	ErrAuthenticationExpired = fmt.Errorf("authentication expired")
	
	// ErrUnauthorized indicates user lacks required permissions
	ErrUnauthorized = fmt.Errorf("unauthorized")
	
	// ErrPermissionDenied indicates specific operation is not permitted
	ErrPermissionDenied = fmt.Errorf("permission denied")
)