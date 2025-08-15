// Package contracts defines request and response contracts for virtual workspace operations.
// These contracts establish standardized data structures for handling HTTP requests
// and responses within the virtual workspace system, ensuring consistent behavior
// across different implementations and enabling proper request context management.
package contracts

import (
	"net/http"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VirtualWorkspaceRequest wraps an HTTP request with virtual workspace context.
// This structure enriches standard HTTP requests with workspace-specific information
// including resource targeting, user identity, and action context.
type VirtualWorkspaceRequest struct {
	// Original HTTP request containing standard request information.
	// This maintains compatibility with existing HTTP handling infrastructure.
	*http.Request

	// Workspace being accessed by this request.
	// This establishes the workspace context for resource operations.
	Workspace string

	// Resource being accessed, specified as Group/Version/Resource.
	// This enables precise resource targeting within the workspace.
	Resource schema.GroupVersionResource

	// Action being performed on the resource (e.g., "get", "list", "create").
	// This corresponds to the HTTP method and resource operation semantics.
	Action string

	// User contains authenticated user information for this request.
	// This enables authorization decisions and audit logging.
	User UserInfo
}

// UserInfo contains authenticated user information extracted from the request.
// This structure provides a normalized view of user identity that abstracts
// away the underlying authentication mechanism.
type UserInfo struct {
	// Username of the authenticated user making the request.
	Username string

	// Groups the user belongs to, used for group-based authorization.
	Groups []string

	// Extra contains additional authentication information from the auth provider.
	// This enables extensibility for custom authentication schemes.
	Extra map[string][]string
}

// RequestContext provides additional context information for request processing.
// This structure supports distributed tracing, request correlation, and debugging
// across the virtual workspace system.
type RequestContext struct {
	// RequestID is a unique identifier for tracking this specific request.
	// This enables request correlation across distributed system components.
	RequestID string

	// CorrelationID links related requests in a distributed transaction.
	// This supports end-to-end tracing in complex request flows.
	CorrelationID string
}