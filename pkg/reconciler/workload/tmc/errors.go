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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
)

// TMCErrorType represents different categories of TMC errors
type TMCErrorType string

const (
	// Resource errors
	TMCErrorTypeResourceNotFound   TMCErrorType = "ResourceNotFound"
	TMCErrorTypeResourceConflict   TMCErrorType = "ResourceConflict"
	TMCErrorTypeResourceValidation TMCErrorType = "ResourceValidation"
	TMCErrorTypeResourcePermission TMCErrorType = "ResourcePermission"

	// Cluster errors
	TMCErrorTypeClusterUnreachable TMCErrorType = "ClusterUnreachable"
	TMCErrorTypeClusterUnavailable TMCErrorType = "ClusterUnavailable"
	TMCErrorTypeClusterAuth        TMCErrorType = "ClusterAuthentication"
	TMCErrorTypeClusterConfig      TMCErrorType = "ClusterConfiguration"

	// Placement errors
	TMCErrorTypePlacementConstraint TMCErrorType = "PlacementConstraint"
	TMCErrorTypePlacementCapacity   TMCErrorType = "PlacementCapacity"
	TMCErrorTypePlacementPolicy     TMCErrorType = "PlacementPolicy"

	// Sync errors
	TMCErrorTypeSyncFailure  TMCErrorType = "SyncFailure"
	TMCErrorTypeSyncConflict TMCErrorType = "SyncConflict"
	TMCErrorTypeSyncTimeout  TMCErrorType = "SyncTimeout"

	// Migration errors
	TMCErrorTypeMigrationFailure  TMCErrorType = "MigrationFailure"
	TMCErrorTypeMigrationTimeout  TMCErrorType = "MigrationTimeout"
	TMCErrorTypeMigrationRollback TMCErrorType = "MigrationRollback"

	// Aggregation errors
	TMCErrorTypeAggregationFailure  TMCErrorType = "AggregationFailure"
	TMCErrorTypeAggregationConflict TMCErrorType = "AggregationConflict"

	// Projection errors
	TMCErrorTypeProjectionFailure   TMCErrorType = "ProjectionFailure"
	TMCErrorTypeProjectionTransform TMCErrorType = "ProjectionTransform"

	// System errors
	TMCErrorTypeInternal            TMCErrorType = "InternalError"
	TMCErrorTypeConfiguration       TMCErrorType = "ConfigurationError"
	TMCErrorTypeNetworkConnectivity TMCErrorType = "NetworkConnectivity"
)

// TMCErrorSeverity represents the severity level of an error
type TMCErrorSeverity string

const (
	TMCErrorSeverityLow      TMCErrorSeverity = "Low"
	TMCErrorSeverityMedium   TMCErrorSeverity = "Medium"
	TMCErrorSeverityHigh     TMCErrorSeverity = "High"
	TMCErrorSeverityCritical TMCErrorSeverity = "Critical"
)

// TMCError represents a categorized TMC error with context and recovery information
type TMCError struct {
	Type         TMCErrorType
	Severity     TMCErrorSeverity
	Component    string
	Operation    string
	Message      string
	Cause        error
	Timestamp    time.Time
	Context      map[string]interface{}
	Retryable    bool
	RecoveryHint string

	// Cluster context
	ClusterName    string
	LogicalCluster string

	// Resource context
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
}

// Error implements the error interface
func (e *TMCError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s in %s.%s: %s (caused by: %v)",
			e.Type, e.Severity, e.Message, e.Component, e.Operation, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s in %s.%s: %s",
		e.Type, e.Severity, e.Message, e.Component, e.Operation, e.Message)
}

// Unwrap returns the underlying cause
func (e *TMCError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether this error should be retried
func (e *TMCError) IsRetryable() bool {
	return e.Retryable
}

// GetRecoveryActions returns suggested recovery actions
func (e *TMCError) GetRecoveryActions() []string {
	actions := make([]string, 0)

	switch e.Type {
	case TMCErrorTypeClusterUnreachable:
		actions = append(actions, []string{
			"Check cluster connectivity",
			"Verify cluster endpoint configuration",
			"Check network policies and firewall rules",
		}...)
	case TMCErrorTypeClusterAuth:
		actions = append(actions, []string{
			"Verify cluster credentials",
			"Check RBAC permissions",
			"Refresh authentication tokens",
		}...)
	case TMCErrorTypeResourceConflict:
		actions = append(actions, []string{
			"Check for resource version conflicts",
			"Verify resource ownership",
			"Consider conflict resolution strategy",
		}...)
	case TMCErrorTypePlacementConstraint:
		actions = append(actions, []string{
			"Review placement constraints",
			"Check cluster capacity",
			"Verify cluster labels and selectors",
		}...)
	case TMCErrorTypeMigrationFailure:
		actions = append(actions, []string{
			"Check source and target cluster health",
			"Verify resource dependencies",
			"Consider rollback strategy",
		}...)
	default:
		if e.RecoveryHint != "" {
			actions = append(actions, e.RecoveryHint)
		}
	}

	if e.Retryable {
		actions = append(actions, "Retry the operation")
	}

	return actions
}

// TMCErrorBuilder helps construct TMC errors with proper categorization
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
			Context:   make(map[string]interface{}),
			Retryable: determineRetryable(errorType),
			Severity:  determineSeverity(errorType),
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

// WithSeverity sets the error severity
func (b *TMCErrorBuilder) WithSeverity(severity TMCErrorSeverity) *TMCErrorBuilder {
	b.error.Severity = severity
	return b
}

// WithCluster sets cluster context
func (b *TMCErrorBuilder) WithCluster(clusterName, logicalCluster string) *TMCErrorBuilder {
	b.error.ClusterName = clusterName
	b.error.LogicalCluster = logicalCluster
	return b
}

// WithResource sets resource context
func (b *TMCErrorBuilder) WithResource(gvk schema.GroupVersionKind, namespace, name string) *TMCErrorBuilder {
	b.error.GVK = gvk
	b.error.Namespace = namespace
	b.error.Name = name
	return b
}

// WithContext adds contextual information
func (b *TMCErrorBuilder) WithContext(key string, value interface{}) *TMCErrorBuilder {
	b.error.Context[key] = value
	return b
}

// WithRetryable sets whether the error is retryable
func (b *TMCErrorBuilder) WithRetryable(retryable bool) *TMCErrorBuilder {
	b.error.Retryable = retryable
	return b
}

// WithRecoveryHint sets a recovery hint
func (b *TMCErrorBuilder) WithRecoveryHint(hint string) *TMCErrorBuilder {
	b.error.RecoveryHint = hint
	return b
}

// Build constructs the final TMC error
func (b *TMCErrorBuilder) Build() *TMCError {
	return b.error
}

// Helper functions

func determineRetryable(errorType TMCErrorType) bool {
	retryableErrors := map[TMCErrorType]bool{
		TMCErrorTypeClusterUnreachable:  true,
		TMCErrorTypeClusterUnavailable:  true,
		TMCErrorTypeSyncTimeout:         true,
		TMCErrorTypeMigrationTimeout:    true,
		TMCErrorTypeAggregationFailure:  true,
		TMCErrorTypeProjectionFailure:   true,
		TMCErrorTypeNetworkConnectivity: true,
		TMCErrorTypeInternal:            true,
	}
	return retryableErrors[errorType]
}

func determineSeverity(errorType TMCErrorType) TMCErrorSeverity {
	severityMap := map[TMCErrorType]TMCErrorSeverity{
		// Critical errors
		TMCErrorTypeClusterAuth:       TMCErrorSeverityCritical,
		TMCErrorTypeMigrationRollback: TMCErrorSeverityCritical,
		TMCErrorTypeInternal:          TMCErrorSeverityCritical,

		// High severity errors
		TMCErrorTypeClusterUnreachable:  TMCErrorSeverityHigh,
		TMCErrorTypeMigrationFailure:    TMCErrorSeverityHigh,
		TMCErrorTypeResourcePermission:  TMCErrorSeverityHigh,
		TMCErrorTypePlacementConstraint: TMCErrorSeverityHigh,

		// Medium severity errors
		TMCErrorTypeResourceConflict:   TMCErrorSeverityMedium,
		TMCErrorTypeSyncFailure:        TMCErrorSeverityMedium,
		TMCErrorTypeAggregationFailure: TMCErrorSeverityMedium,
		TMCErrorTypeProjectionFailure:  TMCErrorSeverityMedium,

		// Low severity errors
		TMCErrorTypeResourceNotFound:   TMCErrorSeverityLow,
		TMCErrorTypeResourceValidation: TMCErrorSeverityLow,
		TMCErrorTypeSyncTimeout:        TMCErrorSeverityLow,
	}

	if severity, exists := severityMap[errorType]; exists {
		return severity
	}
	return TMCErrorSeverityMedium
}

// Error conversion utilities

// ConvertKubernetesError converts a Kubernetes API error to a TMC error
func ConvertKubernetesError(err error, component, operation string) *TMCError {
	if err == nil {
		return nil
	}

	var errorType TMCErrorType
	var retryable bool

	switch {
	case errors.IsNotFound(err):
		errorType = TMCErrorTypeResourceNotFound
		retryable = false
	case errors.IsAlreadyExists(err):
		errorType = TMCErrorTypeResourceConflict
		retryable = true
	case errors.IsConflict(err):
		errorType = TMCErrorTypeResourceConflict
		retryable = true
	case errors.IsForbidden(err):
		errorType = TMCErrorTypeResourcePermission
		retryable = false
	case errors.IsUnauthorized(err):
		errorType = TMCErrorTypeClusterAuth
		retryable = false
	case errors.IsTimeout(err):
		errorType = TMCErrorTypeSyncTimeout
		retryable = true
	case errors.IsServerTimeout(err):
		errorType = TMCErrorTypeClusterUnavailable
		retryable = true
	case errors.IsServiceUnavailable(err):
		errorType = TMCErrorTypeClusterUnavailable
		retryable = true
	case errors.IsTooManyRequests(err):
		errorType = TMCErrorTypeClusterUnavailable
		retryable = true
	case errors.IsInternalError(err):
		errorType = TMCErrorTypeInternal
		retryable = true
	default:
		errorType = TMCErrorTypeInternal
		retryable = true
	}

	return NewTMCError(errorType, component, operation).
		WithCause(err).
		WithMessage(err.Error()).
		WithRetryable(retryable).
		Build()
}

// RetryStrategy defines how errors should be retried
type RetryStrategy struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []TMCErrorType
}

// DefaultRetryStrategy returns a default retry strategy for TMC operations
func DefaultRetryStrategy() *RetryStrategy {
	return &RetryStrategy{
		MaxRetries:    5,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []TMCErrorType{
			TMCErrorTypeClusterUnreachable,
			TMCErrorTypeClusterUnavailable,
			TMCErrorTypeSyncTimeout,
			TMCErrorTypeAggregationFailure,
			TMCErrorTypeProjectionFailure,
			TMCErrorTypeNetworkConnectivity,
			TMCErrorTypeInternal,
		},
	}
}

// ShouldRetry determines if an error should be retried based on the strategy
func (rs *RetryStrategy) ShouldRetry(err error, attempt int) bool {
	if attempt >= rs.MaxRetries {
		return false
	}

	tmcErr, ok := err.(*TMCError)
	if !ok {
		// For non-TMC errors, use basic retry logic
		return attempt < rs.MaxRetries
	}

	if !tmcErr.IsRetryable() {
		return false
	}

	// Check if error type is in retryable list
	for _, retryableType := range rs.RetryableErrors {
		if tmcErr.Type == retryableType {
			return true
		}
	}

	return false
}

// GetDelay calculates the delay before the next retry attempt
func (rs *RetryStrategy) GetDelay(attempt int) time.Duration {
	delay := float64(rs.InitialDelay) * pow(rs.BackoffFactor, float64(attempt))
	if delay > float64(rs.MaxDelay) {
		delay = float64(rs.MaxDelay)
	}
	return time.Duration(delay)
}

// RetryableOperation represents an operation that can be retried
type RetryableOperation func() error

// ExecuteWithRetry executes an operation with the specified retry strategy
func ExecuteWithRetry(operation RetryableOperation, strategy *RetryStrategy) error {
	var lastErr error

	for attempt := 0; attempt < strategy.MaxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		if !strategy.ShouldRetry(err, attempt) {
			break
		}

		if attempt < strategy.MaxRetries-1 {
			delay := strategy.GetDelay(attempt)
			time.Sleep(delay)
		}
	}

	return lastErr
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// CircuitBreaker implements circuit breaker pattern for TMC operations
type CircuitBreaker struct {
	name            string
	maxFailures     int
	resetTimeout    time.Duration
	state           CircuitBreakerState
	failures        int
	lastFailureTime time.Time
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        CircuitBreakerClosed,
	}
}

// Execute executes an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(operation func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if we should open the circuit breaker
	if cb.state == CircuitBreakerClosed && cb.failures >= cb.maxFailures {
		cb.state = CircuitBreakerOpen
		cb.lastFailureTime = time.Now()
	}

	// Check if we should transition to half-open
	if cb.state == CircuitBreakerOpen && time.Since(cb.lastFailureTime) > cb.resetTimeout {
		cb.state = CircuitBreakerHalfOpen
	}

	// Fail fast if circuit breaker is open
	if cb.state == CircuitBreakerOpen {
		return NewTMCError(TMCErrorTypeClusterUnavailable, "circuit-breaker", "execute").
			WithMessage(fmt.Sprintf("Circuit breaker %s is open", cb.name)).
			WithRetryable(false).
			Build()
	}

	// Execute the operation
	err := operation()
	if err != nil {
		cb.failures++
		cb.lastFailureTime = time.Now()

		if cb.state == CircuitBreakerHalfOpen {
			cb.state = CircuitBreakerOpen
		}
		return err
	}

	// Reset on success
	cb.failures = 0
	cb.state = CircuitBreakerClosed
	return nil
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// pow is a simple integer power function
func pow(base float64, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// ErrorCondition represents an error condition that can be added to resource status
type ErrorCondition struct {
	Type               string
	Status             metav1.ConditionStatus
	LastTransitionTime metav1.Time
	Reason             string
	Message            string
	ErrorType          TMCErrorType
	Severity           TMCErrorSeverity
}

// ToCondition converts a TMC error to a Kubernetes condition
func (e *TMCError) ToCondition(conditionType string) ErrorCondition {
	status := metav1.ConditionFalse
	if e.Severity == TMCErrorSeverityLow {
		status = metav1.ConditionTrue
	}

	return ErrorCondition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Time{Time: e.Timestamp},
		Reason:             string(e.Type),
		Message:            e.Message,
		ErrorType:          e.Type,
		Severity:           e.Severity,
	}
}
