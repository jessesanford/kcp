package contracts

import (
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VirtualWorkspaceResponse represents a response from virtual workspace operations.
// This structure provides a standardized format for returning operation results,
// including the response object, operation status, and metadata for client processing.
type VirtualWorkspaceResponse struct {
	// Object contains the Kubernetes object being returned by the operation.
	// This may be a single object, a list, or a status object depending on the operation.
	Object runtime.Object

	// Status indicates the overall result of the operation.
	// This provides high-level success/failure information for client handling.
	Status ResponseStatus

	// Metadata contains additional information about the response.
	// This includes pagination tokens, resource versions, and warnings.
	Metadata ResponseMetadata
}

// ResponseStatus indicates the result of a virtual workspace operation.
// This enumeration provides standardized status reporting across all operations.
type ResponseStatus string

const (
	// ResponseStatusSuccess indicates the operation completed successfully.
	ResponseStatusSuccess ResponseStatus = "Success"
	// ResponseStatusPartial indicates the operation completed with some failures.
	ResponseStatusPartial ResponseStatus = "Partial"
	// ResponseStatusError indicates the operation failed completely.
	ResponseStatusError ResponseStatus = "Error"
)

// ResponseMetadata contains metadata about the response for client processing.
// This structure supports pagination, optimistic concurrency, and client warnings
// following Kubernetes API conventions.
type ResponseMetadata struct {
	// ResourceVersion enables optimistic concurrency control.
	// Clients can use this value for conditional operations and watch resumption.
	ResourceVersion string

	// Continue token enables pagination for large result sets.
	// Clients include this token in subsequent requests to retrieve more results.
	Continue string

	// RemainingItemCount indicates how many more items are available.
	// This helps clients understand the total scope of paginated results.
	RemainingItemCount *int64

	// Warnings contains non-fatal issues encountered during processing.
	// These warnings inform clients about potential problems without failing the operation.
	Warnings []string
}

// ErrorResponse represents an error response from virtual workspace operations.
// This structure provides detailed error information following Kubernetes API conventions.
type ErrorResponse struct {
	// Status embeds the standard Kubernetes status object.
	// This ensures compatibility with existing Kubernetes client tooling.
	metav1.Status

	// Details provides additional context-specific error information.
	// This supplements the standard status with virtual workspace specific details.
	Details string
}