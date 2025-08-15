package interfaces

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AuthorizationProvider handles authorization for virtual workspace operations.
// It provides comprehensive access control mechanisms including permission checking,
// user permission retrieval, and cache management for optimal performance.
type AuthorizationProvider interface {
	// Authorize determines if a request is allowed within the virtual workspace context.
	// This method evaluates the authorization request against configured policies
	// and returns a decision with explanatory information.
	Authorize(ctx context.Context, req *AuthorizationRequest) (*AuthorizationDecision, error)

	// GetPermissions returns all permissions for a user within a specific workspace.
	// This enables UI and tooling to understand user capabilities.
	GetPermissions(ctx context.Context, workspace, user string) ([]Permission, error)

	// RefreshCache updates cached authorization data for a workspace.
	// This is useful when authorization policies change and cache invalidation is needed.
	RefreshCache(ctx context.Context, workspace string) error
}

// AuthorizationRequest contains all details needed for an authorization check.
// It captures the complete context of a request including user identity,
// target resource, and intended operation.
type AuthorizationRequest struct {
	// User making the request (authenticated username).
	User string

	// Groups the user belongs to for group-based authorization.
	Groups []string

	// Workspace being accessed for workspace-scoped authorization.
	Workspace string

	// Resource being accessed (Group/Version/Resource).
	Resource schema.GroupVersionResource

	// Verb being performed (get, list, create, update, delete, etc.).
	Verb string

	// ResourceName if accessing a specific named resource instance.
	// Empty for collection-level operations like list.
	ResourceName string
}

// AuthorizationDecision contains the result of an authorization check.
// It provides both the authorization result and explanatory information
// for auditing and debugging purposes.
type AuthorizationDecision struct {
	// Allowed indicates if the request is authorized to proceed.
	Allowed bool

	// Reason provides a human-readable explanation for the authorization decision.
	// This is valuable for debugging and audit logging.
	Reason string

	// EvaluationError contains any error encountered during authorization evaluation.
	// This distinguishes between "denied" and "error during evaluation".
	EvaluationError error
}

// Permission represents a granted permission within a virtual workspace.
// It defines what resources a user can access and what operations they can perform.
type Permission struct {
	// Resource specifies the Kubernetes resource this permission applies to.
	Resource schema.GroupVersionResource

	// Verbs lists the operations allowed on this resource.
	Verbs []string

	// ResourceNames restricts the permission to specific resource instances.
	// Empty slice means permission applies to all instances of the resource.
	ResourceNames []string
}