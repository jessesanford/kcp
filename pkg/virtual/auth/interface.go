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

// Package auth provides authorization contracts for virtual workspace providers.
// It defines pluggable authorization interfaces and implementations for controlling
// access to workspace resources through RBAC and custom providers.
package auth

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Provider handles authorization for virtual workspaces.
// It provides a pluggable interface for different authorization mechanisms
// including RBAC, ABAC, or custom authorization providers.
type Provider interface {
	// Name returns the provider name for identification and logging.
	Name() string

	// Initialize sets up the provider with the given configuration.
	// This method is called once during provider startup and should
	// establish necessary connections and load initial data.
	Initialize(ctx context.Context, config ProviderConfig) error

	// Authorize performs an authorization check for a given request.
	// Returns a decision indicating whether the request is allowed
	// along with reasoning and audit information.
	Authorize(ctx context.Context, req *Request) (*Decision, error)

	// GetPermissions returns all permissions for a user in a workspace.
	// This is used for UI display and permission introspection.
	GetPermissions(ctx context.Context, workspace, user string) ([]Permission, error)

	// RefreshCache updates cached authorization data for a workspace.
	// This should be called when RBAC rules change or permissions are updated.
	RefreshCache(ctx context.Context, workspace string) error

	// Close cleans up provider resources and closes connections.
	// Should be called during graceful shutdown.
	Close(ctx context.Context) error
}

// ProviderConfig contains configuration for authorization providers.
// It provides common settings that most providers will need.
type ProviderConfig struct {
	// KubeConfig path for accessing Kubernetes APIs.
	// If empty, uses in-cluster configuration.
	KubeConfig string

	// CacheEnabled enables permission caching to improve performance.
	CacheEnabled bool

	// CacheTTL is cache time-to-live in seconds.
	// Only used when CacheEnabled is true.
	CacheTTL int64

	// AuditEnabled enables audit logging for all authorization decisions.
	// Useful for security monitoring and compliance.
	AuditEnabled bool
}

// Request represents an authorization request for a specific action.
// It contains all information needed to make an authorization decision.
type Request struct {
	// User making the request (subject).
	User string

	// Groups the user belongs to.
	Groups []string

	// Workspace being accessed (namespace equivalent).
	Workspace string

	// Resource being accessed (deployment, pod, etc.).
	Resource schema.GroupVersionResource

	// ResourceName for access to a specific named resource.
	// Empty string means access to all resources of the type.
	ResourceName string

	// Verb being performed (get, list, create, update, delete, etc.).
	Verb string

	// Extra contains additional attributes for authorization.
	// Can include custom claims, IP addresses, or other context.
	Extra map[string][]string
}

// Decision represents the result of an authorization check.
// It includes the decision, reasoning, and audit information.
type Decision struct {
	// Allowed indicates whether the request is authorized.
	Allowed bool

	// Reason provides a human-readable explanation for the decision.
	// Should be suitable for logging and error messages.
	Reason string

	// EvaluationError contains any error that occurred during evaluation.
	// If non-nil, the request should be denied for safety.
	EvaluationError error

	// AuditAnnotations contains key-value pairs for audit logging.
	// These are included in audit logs for security monitoring.
	AuditAnnotations map[string]string
}

// Permission represents a granted permission for a user.
// It defines what actions a user can perform on which resources.
type Permission struct {
	// Resource this permission applies to.
	// Use wildcards (*) for broader permissions.
	Resource schema.GroupVersionResource

	// Verbs allowed on the resource (get, list, create, etc.).
	// Use ["*"] to allow all verbs.
	Verbs []string

	// ResourceNames for access to specific named resources.
	// Empty slice means access to all resources of the type.
	ResourceNames []string

	// NonResourceURLs for access to non-resource endpoints.
	// Used for health checks, metrics, etc.
	NonResourceURLs []string
}