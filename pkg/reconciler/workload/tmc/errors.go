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

package tmc

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TMCErrorType represents different categories of errors in TMC
type TMCErrorType string

const (
	// TMCErrorTypeResourceValidation indicates a resource validation error
	TMCErrorTypeResourceValidation TMCErrorType = "ResourceValidation"

	// TMCErrorTypeSyncFailure indicates a synchronization failure
	TMCErrorTypeSyncFailure TMCErrorType = "SyncFailure"

	// TMCErrorTypeClusterUnreachable indicates a cluster is unreachable
	TMCErrorTypeClusterUnreachable TMCErrorType = "ClusterUnreachable"

	// TMCErrorTypeRetryExhausted indicates retry attempts have been exhausted
	TMCErrorTypeRetryExhausted TMCErrorType = "RetryExhausted"

	// TMCErrorTypeConfiguration indicates a configuration error
	TMCErrorTypeConfiguration TMCErrorType = "Configuration"
)

// TMCError represents an error in the TMC system with rich context
type TMCError struct {
	// Type categorizes the error
	Type TMCErrorType `json:"type"`

	// Component identifies the component that generated the error
	Component string `json:"component"`

	// Operation identifies the operation that failed
	Operation string `json:"operation"`

	// Message provides a human-readable description
	Message string `json:"message"`

	// Cause is the underlying error that caused this error
	Cause error `json:"cause,omitempty"`

	// ClusterName identifies the cluster involved in the error
	ClusterName string `json:"clusterName,omitempty"`

	// LogicalCluster identifies the logical cluster context
	LogicalCluster string `json:"logicalCluster,omitempty"`

	// Resource provides resource context for the error
	Resource *ResourceRef `json:"resource,omitempty"`

	// Timestamp when the error occurred
	Timestamp time.Time `json:"timestamp"`
}

// ResourceRef provides a reference to a Kubernetes resource
type ResourceRef struct {
	// GVK is the GroupVersionKind of the resource
	GVK schema.GroupVersionKind `json:"gvk"`

	// Namespace is the namespace of the resource (if namespaced)
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the resource
	Name string `json:"name"`
}

// Error implements the error interface
func (e *TMCError) Error() string {
	msg := fmt.Sprintf("[%s] %s.%s: %s", e.Type, e.Component, e.Operation, e.Message)
	
	if e.ClusterName != "" {
		msg += fmt.Sprintf(" (cluster: %s)", e.ClusterName)
	}
	
	if e.Resource != nil {
		msg += fmt.Sprintf(" (resource: %s/%s %s)", e.Resource.GVK.Kind, e.Resource.Namespace, e.Resource.Name)
	}
	
	if e.Cause != nil {
		msg += fmt.Sprintf(" caused by: %v", e.Cause)
	}
	
	return msg
}

// Unwrap returns the underlying cause for error unwrapping
func (e *TMCError) Unwrap() error {
	return e.Cause
}

// TMCErrorBuilder provides a fluent interface for building TMC errors
type TMCErrorBuilder struct {
	error *TMCError
}

// NewTMCError creates a new TMC error builder
func NewTMCError(errorType TMCErrorType, component, operation string) *TMCErrorBuilder {
	return &TMCErrorBuilder{
		error: &TMCError{
			Type:      errorType,
			Component: component,
			Operation: operation,
			Timestamp: time.Now(),
		},
	}
}

// WithMessage sets the error message
func (b *TMCErrorBuilder) WithMessage(message string) *TMCErrorBuilder {
	b.error.Message = message
	return b
}

// WithCause sets the underlying cause
func (b *TMCErrorBuilder) WithCause(cause error) *TMCErrorBuilder {
	b.error.Cause = cause
	return b
}

// WithCluster sets the cluster context
func (b *TMCErrorBuilder) WithCluster(clusterName, logicalCluster string) *TMCErrorBuilder {
	b.error.ClusterName = clusterName
	b.error.LogicalCluster = logicalCluster
	return b
}

// WithResource sets the resource context
func (b *TMCErrorBuilder) WithResource(gvk schema.GroupVersionKind, namespace, name string) *TMCErrorBuilder {
	b.error.Resource = &ResourceRef{
		GVK:       gvk,
		Namespace: namespace,
		Name:      name,
	}
	return b
}

// Build creates the final TMC error
func (b *TMCErrorBuilder) Build() error {
	return b.error
}

// RetryStrategy defines retry behavior for operations
type RetryStrategy struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int

	// InitialDelay is the initial delay between retries
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// BackoffMultiplier is the multiplier applied to delay after each retry
	BackoffMultiplier float64

	// RetryableErrors defines which error types should be retried
	RetryableErrors []TMCErrorType
}

// DefaultRetryStrategy returns the default retry strategy
func DefaultRetryStrategy() *RetryStrategy {
	return &RetryStrategy{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableErrors: []TMCErrorType{
			TMCErrorTypeSyncFailure,
			TMCErrorTypeClusterUnreachable,
		},
	}
}

// ShouldRetry determines if an error should be retried
func (rs *RetryStrategy) ShouldRetry(err error, attempt int) bool {
	if attempt >= rs.MaxAttempts {
		return false
	}

	if tmcErr, ok := err.(*TMCError); ok {
		for _, retryableType := range rs.RetryableErrors {
			if tmcErr.Type == retryableType {
				return true
			}
		}
	}

	return false
}

// GetDelay calculates the delay for a given attempt
func (rs *RetryStrategy) GetDelay(attempt int) time.Duration {
	delay := time.Duration(float64(rs.InitialDelay) * pow(rs.BackoffMultiplier, float64(attempt)))
	if delay > rs.MaxDelay {
		delay = rs.MaxDelay
	}
	return delay
}

// ExecuteWithRetry executes an operation with retry logic
func ExecuteWithRetry(operation func() error, strategy *RetryStrategy) error {
	var lastErr error
	
	for attempt := 0; attempt < strategy.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		if !strategy.ShouldRetry(err, attempt) {
			break
		}
		
		if attempt < strategy.MaxAttempts-1 {
			time.Sleep(strategy.GetDelay(attempt))
		}
	}
	
	// If we've exhausted retries, wrap the error
	if tmcErr, ok := lastErr.(*TMCError); ok && strategy.MaxAttempts > 1 {
		return NewTMCError(TMCErrorTypeRetryExhausted, tmcErr.Component, tmcErr.Operation).
			WithMessage(fmt.Sprintf("Exhausted %d retry attempts", strategy.MaxAttempts)).
			WithCause(lastErr).
			Build()
	}
	
	return lastErr
}

// Simple power function for calculating exponential backoff
func pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	result := base
	for i := 1; i < int(exp); i++ {
		result *= base
	}
	return result
}