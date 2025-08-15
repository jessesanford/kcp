package contracts

import (
	"fmt"
)

// VirtualWorkspaceError represents errors in virtual workspace operations.
// This error type provides structured error information including error categorization,
// workspace context, and underlying cause information for proper error handling.
type VirtualWorkspaceError struct {
	// Type categorizes the error for appropriate handling logic.
	// This enables clients to implement different retry and recovery strategies.
	Type ErrorType

	// Message provides a human-readable description of the error.
	// This message should be suitable for logging and user display.
	Message string

	// Workspace identifies where the error occurred.
	// This context is essential for debugging multi-workspace systems.
	Workspace string

	// Cause contains the underlying error that triggered this virtual workspace error.
	// This preserves error context for debugging while providing structured information.
	Cause error
}

// ErrorType categorizes virtual workspace errors for appropriate handling.
// These error types align with standard HTTP status codes and Kubernetes API conventions.
type ErrorType string

const (
	// ErrorTypeNotFound indicates the requested resource or workspace was not found.
	ErrorTypeNotFound ErrorType = "NotFound"
	// ErrorTypeUnauthorized indicates the request lacks valid authentication.
	ErrorTypeUnauthorized ErrorType = "Unauthorized"
	// ErrorTypeInvalid indicates the request contains invalid data or parameters.
	ErrorTypeInvalid ErrorType = "Invalid"
	// ErrorTypeConflict indicates the request conflicts with current system state.
	ErrorTypeConflict ErrorType = "Conflict"
	// ErrorTypeInternal indicates an internal system error occurred.
	ErrorTypeInternal ErrorType = "Internal"
	// ErrorTypeTimeout indicates the operation exceeded its time limit.
	ErrorTypeTimeout ErrorType = "Timeout"
)

// Error implements the error interface, providing formatted error messages.
// The format includes error type, message, workspace context, and underlying cause.
func (e *VirtualWorkspaceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (workspace: %s): %v", e.Type, e.Message, e.Workspace, e.Cause)
	}
	return fmt.Sprintf("%s: %s (workspace: %s)", e.Type, e.Message, e.Workspace)
}

// IsRetryable indicates if the error represents a transient condition.
// This method helps clients implement appropriate retry logic for different error types.
func (e *VirtualWorkspaceError) IsRetryable() bool {
	return e.Type == ErrorTypeTimeout || e.Type == ErrorTypeInternal
}