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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SessionValidator represents a validation framework for placement sessions.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type SessionValidator struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec SessionValidatorSpec `json:"spec,omitempty"`
	// +optional
	Status SessionValidatorStatus `json:"status,omitempty"`
}

// SessionValidatorList contains a list of SessionValidator
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SessionValidatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SessionValidator `json:"items"`
}

// SessionValidatorSpec defines the desired state of SessionValidator
type SessionValidatorSpec struct {
	// ValidationRules defines the validation rules to apply
	ValidationRules []ValidationRule `json:"validationRules"`

	// ConflictDetection defines conflict detection policies
	// +optional
	ConflictDetection *ConflictDetectionPolicy `json:"conflictDetection,omitempty"`

	// ResourceValidation defines resource validation policies
	// +optional
	ResourceValidation *ResourceValidationPolicy `json:"resourceValidation,omitempty"`

	// DependencyValidation defines dependency validation policies
	// +optional
	DependencyValidation *DependencyValidationPolicy `json:"dependencyValidation,omitempty"`

	// ValidationScope defines the scope of validation
	// +optional
	ValidationScope *ValidationScope `json:"validationScope,omitempty"`
}

// SessionValidatorStatus defines the observed state of SessionValidator
type SessionValidatorStatus struct {
	// ValidationResults contains the results of recent validations
	// +optional
	ValidationResults []ValidationResult `json:"validationResults,omitempty"`

	// ConflictDetections contains recent conflict detections
	// +optional
	ConflictDetections []ConflictDetection `json:"conflictDetections,omitempty"`

	// LastValidationTime is when validation was last performed
	// +optional
	LastValidationTime *metav1.Time `json:"lastValidationTime,omitempty"`

	// ValidationMetrics contains metrics about validation operations
	// +optional
	ValidationMetrics *ValidationMetrics `json:"validationMetrics,omitempty"`
}

// ValidationRule defines a validation rule for sessions
type ValidationRule struct {
	// Name is the name of the validation rule
	Name string `json:"name"`

	// Type specifies the type of validation rule
	Type ValidationRuleType `json:"type"`

	// Condition defines the condition that triggers validation
	Condition ValidationCondition `json:"condition"`

	// Validator defines the validation logic
	Validator ValidatorConfiguration `json:"validator"`

	// Severity defines the severity of validation failures
	// +kubebuilder:validation:Enum=Info;Warning;Error;Critical
	// +kubebuilder:default="Error"
	// +optional
	Severity ValidationSeverity `json:"severity,omitempty"`

	// Enabled indicates whether this rule is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Description provides a description of the validation rule
	// +optional
	Description string `json:"description,omitempty"`
}

// ValidationRuleType defines the types of validation rules
// +kubebuilder:validation:Enum=SessionConfiguration;PlacementPolicy;ResourceConstraint;ConflictDetection;DependencyCheck
type ValidationRuleType string

const (
	// ValidationRuleTypeSessionConfiguration validates session configuration
	ValidationRuleTypeSessionConfiguration ValidationRuleType = "SessionConfiguration"
	// ValidationRuleTypePlacementPolicy validates placement policies
	ValidationRuleTypePlacementPolicy ValidationRuleType = "PlacementPolicy"
	// ValidationRuleTypeResourceConstraint validates resource constraints
	ValidationRuleTypeResourceConstraint ValidationRuleType = "ResourceConstraint"
	// ValidationRuleTypeConflictDetection validates for conflicts
	ValidationRuleTypeConflictDetection ValidationRuleType = "ConflictDetection"
	// ValidationRuleTypeDependencyCheck validates dependencies
	ValidationRuleTypeDependencyCheck ValidationRuleType = "DependencyCheck"
)

// ValidationCondition defines when a validation rule should be applied
type ValidationCondition struct {
	// Event specifies the event that triggers validation
	Event ValidationEvent `json:"event"`

	// Filters defines filters for the validation trigger
	// +optional
	Filters []ValidationFilter `json:"filters,omitempty"`

	// Schedule defines a schedule for periodic validation
	// +optional
	Schedule string `json:"schedule,omitempty"`
}

// ValidationEvent defines events that can trigger validation
// +kubebuilder:validation:Enum=SessionCreate;SessionUpdate;SessionDelete;DecisionCreate;DecisionUpdate;ConflictDetected;ResourceChange
type ValidationEvent string

const (
	// ValidationEventSessionCreate validates on session creation
	ValidationEventSessionCreate ValidationEvent = "SessionCreate"
	// ValidationEventSessionUpdate validates on session updates
	ValidationEventSessionUpdate ValidationEvent = "SessionUpdate"
	// ValidationEventSessionDelete validates on session deletion
	ValidationEventSessionDelete ValidationEvent = "SessionDelete"
	// ValidationEventDecisionCreate validates on decision creation
	ValidationEventDecisionCreate ValidationEvent = "DecisionCreate"
	// ValidationEventDecisionUpdate validates on decision updates
	ValidationEventDecisionUpdate ValidationEvent = "DecisionUpdate"
	// ValidationEventConflictDetected validates when conflicts are detected
	ValidationEventConflictDetected ValidationEvent = "ConflictDetected"
	// ValidationEventResourceChange validates on resource changes
	ValidationEventResourceChange ValidationEvent = "ResourceChange"
)

// ValidationFilter defines filters for validation triggers
type ValidationFilter struct {
	// Field specifies the field to filter on
	Field string `json:"field"`

	// Operator specifies the filter operator
	Operator FilterOperator `json:"operator"`

	// Value specifies the filter value
	Value string `json:"value"`
}

// FilterOperator defines filter operators
// +kubebuilder:validation:Enum=Equals;NotEquals;Contains;NotContains;In;NotIn;Exists;NotExists
type FilterOperator string

const (
	// FilterOperatorEquals represents equality operator
	FilterOperatorEquals FilterOperator = "Equals"
	// FilterOperatorNotEquals represents inequality operator
	FilterOperatorNotEquals FilterOperator = "NotEquals"
	// FilterOperatorContains represents contains operator
	FilterOperatorContains FilterOperator = "Contains"
	// FilterOperatorNotContains represents not contains operator
	FilterOperatorNotContains FilterOperator = "NotContains"
	// FilterOperatorIn represents in operator
	FilterOperatorIn FilterOperator = "In"
	// FilterOperatorNotIn represents not in operator
	FilterOperatorNotIn FilterOperator = "NotIn"
	// FilterOperatorExists represents exists operator
	FilterOperatorExists FilterOperator = "Exists"
	// FilterOperatorNotExists represents not exists operator
	FilterOperatorNotExists FilterOperator = "NotExists"
)

// ValidatorConfiguration defines the configuration for a validator
type ValidatorConfiguration struct {
	// Type specifies the type of validator
	Type ValidatorType `json:"type"`

	// Parameters contains validator-specific parameters
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// Script contains custom validation script (for custom validators)
	// +optional
	Script string `json:"script,omitempty"`

	// Timeout defines the timeout for validation execution
	// +kubebuilder:default="30s"
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`

	// RetryPolicy defines the retry policy for validation
	// +optional
	RetryPolicy *ValidationRetryPolicy `json:"retryPolicy,omitempty"`
}

// ValidatorType defines the types of validators
// +kubebuilder:validation:Enum=Required;Range;Format;Custom;Reference;Uniqueness;Consistency
type ValidatorType string

const (
	// ValidatorTypeRequired validates required fields
	ValidatorTypeRequired ValidatorType = "Required"
	// ValidatorTypeRange validates value ranges
	ValidatorTypeRange ValidatorType = "Range"
	// ValidatorTypeFormat validates field formats
	ValidatorTypeFormat ValidatorType = "Format"
	// ValidatorTypeCustom executes custom validation logic
	ValidatorTypeCustom ValidatorType = "Custom"
	// ValidatorTypeReference validates object references
	ValidatorTypeReference ValidatorType = "Reference"
	// ValidatorTypeUniqueness validates uniqueness constraints
	ValidatorTypeUniqueness ValidatorType = "Uniqueness"
	// ValidatorTypeConsistency validates data consistency
	ValidatorTypeConsistency ValidatorType = "Consistency"
)

// ValidationRetryPolicy defines retry policies for validation
type ValidationRetryPolicy struct {
	// MaxRetries specifies the maximum number of retries
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	// +optional
	MaxRetries int32 `json:"maxRetries,omitempty"`

	// RetryDelay specifies the delay between retries
	// +kubebuilder:default="1s"
	// +optional
	RetryDelay metav1.Duration `json:"retryDelay,omitempty"`

	// BackoffMultiplier specifies the multiplier for exponential backoff
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=2
	// +optional
	BackoffMultiplier float64 `json:"backoffMultiplier,omitempty"`
}

// ValidationSeverity defines the severity levels for validation results
// +kubebuilder:validation:Enum=Info;Warning;Error;Critical
type ValidationSeverity string

const (
	// ValidationSeverityInfo represents informational validation results
	ValidationSeverityInfo ValidationSeverity = "Info"
	// ValidationSeverityWarning represents warning validation results
	ValidationSeverityWarning ValidationSeverity = "Warning"
	// ValidationSeverityError represents error validation results
	ValidationSeverityError ValidationSeverity = "Error"
	// ValidationSeverityCritical represents critical validation results
	ValidationSeverityCritical ValidationSeverity = "Critical"
)

// ConflictDetectionPolicy defines policies for conflict detection
type ConflictDetectionPolicy struct {
	// Enabled indicates whether conflict detection is enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// DetectionScope defines the scope of conflict detection
	// +optional
	DetectionScope *ConflictDetectionScope `json:"detectionScope,omitempty"`

	// ConflictTypes defines the types of conflicts to detect
	// +optional
	ConflictTypes []ConflictType `json:"conflictTypes,omitempty"`

	// ResolutionStrategies defines strategies for conflict resolution
	// +optional
	ResolutionStrategies []ConflictResolutionStrategy `json:"resolutionStrategies,omitempty"`

	// NotificationPolicy defines how conflicts should be reported
	// +optional
	NotificationPolicy *ConflictNotificationPolicy `json:"notificationPolicy,omitempty"`
}

// ConflictDetectionScope defines the scope of conflict detection
type ConflictDetectionScope struct {
	// IncludeNamespaces defines namespaces to include in conflict detection
	// +optional
	IncludeNamespaces []string `json:"includeNamespaces,omitempty"`

	// ExcludeNamespaces defines namespaces to exclude from conflict detection
	// +optional
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`

	// IncludeClusters defines clusters to include in conflict detection
	// +optional
	IncludeClusters []string `json:"includeClusters,omitempty"`

	// ExcludeClusters defines clusters to exclude from conflict detection
	// +optional
	ExcludeClusters []string `json:"excludeClusters,omitempty"`

	// ResourceTypes defines resource types to include in conflict detection
	// +optional
	ResourceTypes []string `json:"resourceTypes,omitempty"`
}

// ConflictResolutionStrategy defines a strategy for resolving conflicts
type ConflictResolutionStrategy struct {
	// ConflictType specifies the type of conflict this strategy applies to
	ConflictType ConflictType `json:"conflictType"`

	// Strategy specifies the resolution strategy
	Strategy ConflictResolutionType `json:"strategy"`

	// Priority specifies the priority of this strategy
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=500
	// +optional
	Priority int32 `json:"priority,omitempty"`

	// Parameters contains strategy-specific parameters
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// ConflictNotificationPolicy defines how conflicts should be reported
type ConflictNotificationPolicy struct {
	// Enabled indicates whether notifications are enabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Channels defines notification channels
	// +optional
	Channels []NotificationChannelRef `json:"channels,omitempty"`

	// SeverityFilter defines the minimum severity for notifications
	// +kubebuilder:default="Warning"
	// +optional
	SeverityFilter ValidationSeverity `json:"severityFilter,omitempty"`
}

// NotificationChannelRef references a notification channel
type NotificationChannelRef struct {
	// Name is the name of the notification channel
	Name string `json:"name"`

	// Namespace is the namespace of the notification channel
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ResourceValidationPolicy defines policies for resource validation
type ResourceValidationPolicy struct {
	// ValidateCapacity indicates whether to validate resource capacity
	// +kubebuilder:default=true
	// +optional
	ValidateCapacity bool `json:"validateCapacity,omitempty"`

	// ValidateAvailability indicates whether to validate resource availability
	// +kubebuilder:default=true
	// +optional
	ValidateAvailability bool `json:"validateAvailability,omitempty"`

	// CapacityThresholds defines capacity thresholds for validation
	// +optional
	CapacityThresholds map[string]ResourceThreshold `json:"capacityThresholds,omitempty"`

	// ReservationPolicy defines how resources should be reserved
	// +optional
	ReservationPolicy *ResourceReservationPolicy `json:"reservationPolicy,omitempty"`
}

// ResourceThreshold defines a threshold for resource validation
type ResourceThreshold struct {
	// WarningThreshold defines the warning threshold (0-100%)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=80
	// +optional
	WarningThreshold int32 `json:"warningThreshold,omitempty"`

	// ErrorThreshold defines the error threshold (0-100%)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=95
	// +optional
	ErrorThreshold int32 `json:"errorThreshold,omitempty"`
}

// ResourceReservationPolicy defines how resources should be reserved
type ResourceReservationPolicy struct {
	// ReservationMode defines the reservation mode
	// +kubebuilder:validation:Enum=None;Soft;Hard
	// +kubebuilder:default="Soft"
	// +optional
	ReservationMode ResourceReservationMode `json:"reservationMode,omitempty"`

	// ReservationDuration defines how long to hold reservations
	// +kubebuilder:default="5m"
	// +optional
	ReservationDuration metav1.Duration `json:"reservationDuration,omitempty"`

	// AllowOversubscription indicates whether oversubscription is allowed
	// +kubebuilder:default=false
	// +optional
	AllowOversubscription bool `json:"allowOversubscription,omitempty"`
}

// ResourceReservationMode defines resource reservation modes
// +kubebuilder:validation:Enum=None;Soft;Hard
type ResourceReservationMode string

const (
	// ResourceReservationModeNone indicates no resource reservation
	ResourceReservationModeNone ResourceReservationMode = "None"
	// ResourceReservationModeSoft indicates soft resource reservation
	ResourceReservationModeSoft ResourceReservationMode = "Soft"
	// ResourceReservationModeHard indicates hard resource reservation
	ResourceReservationModeHard ResourceReservationMode = "Hard"
)

// DependencyValidationPolicy defines policies for dependency validation
type DependencyValidationPolicy struct {
	// ValidateDependencies indicates whether to validate dependencies
	// +kubebuilder:default=true
	// +optional
	ValidateDependencies bool `json:"validateDependencies,omitempty"`

	// DependencyTypes defines the types of dependencies to validate
	// +optional
	DependencyTypes []DependencyType `json:"dependencyTypes,omitempty"`

	// CircularDependencyDetection indicates whether to detect circular dependencies
	// +kubebuilder:default=true
	// +optional
	CircularDependencyDetection bool `json:"circularDependencyDetection,omitempty"`

	// MaxDependencyDepth defines the maximum depth for dependency validation
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxDependencyDepth int32 `json:"maxDependencyDepth,omitempty"`
}

// DependencyType defines the types of dependencies
// +kubebuilder:validation:Enum=ServiceDependency;ConfigDependency;VolumeDependency;NetworkDependency;Custom
type DependencyType string

const (
	// DependencyTypeServiceDependency represents service dependencies
	DependencyTypeServiceDependency DependencyType = "ServiceDependency"
	// DependencyTypeConfigDependency represents configuration dependencies
	DependencyTypeConfigDependency DependencyType = "ConfigDependency"
	// DependencyTypeVolumeDependency represents volume dependencies
	DependencyTypeVolumeDependency DependencyType = "VolumeDependency"
	// DependencyTypeNetworkDependency represents network dependencies
	DependencyTypeNetworkDependency DependencyType = "NetworkDependency"
	// DependencyTypeCustom represents custom dependencies
	DependencyTypeCustom DependencyType = "Custom"
)

// ValidationScope defines the scope of validation
type ValidationScope struct {
	// IncludeNamespaces defines namespaces to include in validation
	// +optional
	IncludeNamespaces []string `json:"includeNamespaces,omitempty"`

	// ExcludeNamespaces defines namespaces to exclude from validation
	// +optional
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`

	// IncludeClusters defines clusters to include in validation
	// +optional
	IncludeClusters []string `json:"includeClusters,omitempty"`

	// ExcludeClusters defines clusters to exclude from validation
	// +optional
	ExcludeClusters []string `json:"excludeClusters,omitempty"`

	// ResourceSelectors defines resource selectors for validation scope
	// +optional
	ResourceSelectors []ResourceSelector `json:"resourceSelectors,omitempty"`
}

// ResourceSelector defines a selector for resources
type ResourceSelector struct {
	// APIVersion is the API version of the resource
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the resource
	Kind string `json:"kind"`

	// LabelSelector selects resources based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// FieldSelector selects resources based on fields
	// +optional
	FieldSelector string `json:"fieldSelector,omitempty"`
}

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	// ValidationID is the unique identifier for the validation
	ValidationID string `json:"validationID"`

	// RuleName is the name of the validation rule that was applied
	RuleName string `json:"ruleName"`

	// ValidationTime is when the validation was performed
	ValidationTime metav1.Time `json:"validationTime"`

	// Status is the status of the validation
	Status ValidationResultStatus `json:"status"`

	// Severity is the severity of the validation result
	Severity ValidationSeverity `json:"severity"`

	// Message contains a human-readable message about the validation result
	Message string `json:"message"`

	// Details contains detailed information about the validation
	// +optional
	Details map[string]string `json:"details,omitempty"`

	// AffectedObjects contains references to objects affected by the validation
	// +optional
	AffectedObjects []ObjectReference `json:"affectedObjects,omitempty"`
}

// ValidationResultStatus defines the status of a validation result
// +kubebuilder:validation:Enum=Passed;Failed;Warning;Skipped
type ValidationResultStatus string

const (
	// ValidationResultStatusPassed indicates validation passed
	ValidationResultStatusPassed ValidationResultStatus = "Passed"
	// ValidationResultStatusFailed indicates validation failed
	ValidationResultStatusFailed ValidationResultStatus = "Failed"
	// ValidationResultStatusWarning indicates validation generated a warning
	ValidationResultStatusWarning ValidationResultStatus = "Warning"
	// ValidationResultStatusSkipped indicates validation was skipped
	ValidationResultStatusSkipped ValidationResultStatus = "Skipped"
)

// ConflictDetection represents a detected conflict
type ConflictDetection struct {
	// ConflictID is the unique identifier for the conflict
	ConflictID string `json:"conflictID"`

	// DetectionTime is when the conflict was detected
	DetectionTime metav1.Time `json:"detectionTime"`

	// ConflictType describes the type of conflict
	ConflictType ConflictType `json:"conflictType"`

	// ConflictingSessions contains the sessions involved in the conflict
	// +optional
	ConflictingSessions []SessionReference `json:"conflictingSessions,omitempty"`

	// ConflictingDecisions contains the decisions involved in the conflict
	// +optional
	ConflictingDecisions []PlacementReference `json:"conflictingDecisions,omitempty"`

	// Severity is the severity of the conflict
	Severity ValidationSeverity `json:"severity"`

	// Description provides a description of the conflict
	Description string `json:"description"`

	// ResolutionStatus indicates the status of conflict resolution
	// +optional
	ResolutionStatus ConflictStatus `json:"resolutionStatus,omitempty"`
}

// ValidationMetrics contains metrics about validation operations
type ValidationMetrics struct {
	// TotalValidations is the total number of validations performed
	// +optional
	TotalValidations int32 `json:"totalValidations,omitempty"`

	// SuccessfulValidations is the number of successful validations
	// +optional
	SuccessfulValidations int32 `json:"successfulValidations,omitempty"`

	// FailedValidations is the number of failed validations
	// +optional
	FailedValidations int32 `json:"failedValidations,omitempty"`

	// WarningValidations is the number of validations that generated warnings
	// +optional
	WarningValidations int32 `json:"warningValidations,omitempty"`

	// ConflictsDetected is the total number of conflicts detected
	// +optional
	ConflictsDetected int32 `json:"conflictsDetected,omitempty"`

	// ConflictsResolved is the number of conflicts resolved
	// +optional
	ConflictsResolved int32 `json:"conflictsResolved,omitempty"`

	// AverageValidationTime is the average time for validation operations
	// +optional
	AverageValidationTime *metav1.Duration `json:"averageValidationTime,omitempty"`
}