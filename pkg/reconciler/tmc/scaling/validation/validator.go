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

package validation

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// TMC API group and version constants
const (
	TMCAPIGroup   = "tmc.kcp.io"
	TMCAPIVersion = "v1alpha1"
)

// ConfigurationValidator validates TMC scaling configurations
type ConfigurationValidator struct {
	// enabledFeatures controls which validation features are enabled
	enabledFeatures []string
	
	// maxReplicas defines the absolute maximum replicas allowed
	maxReplicas int32
	
	// maxClusters defines the maximum number of clusters allowed
	maxClusters int32
}

// NewConfigurationValidator creates a new configuration validator
func NewConfigurationValidator(opts ...ValidatorOption) *ConfigurationValidator {
	validator := &ConfigurationValidator{
		maxReplicas: 10000,  // reasonable default
		maxClusters: 100,    // reasonable default
	}
	
	for _, opt := range opts {
		opt(validator)
	}
	
	return validator
}

// ValidatorOption configures a ConfigurationValidator
type ValidatorOption func(*ConfigurationValidator)

// WithMaxReplicas sets the maximum replicas limit
func WithMaxReplicas(max int32) ValidatorOption {
	return func(v *ConfigurationValidator) {
		v.maxReplicas = max
	}
}

// WithMaxClusters sets the maximum clusters limit  
func WithMaxClusters(max int32) ValidatorOption {
	return func(v *ConfigurationValidator) {
		v.maxClusters = max
	}
}

// WithEnabledFeatures sets the enabled validation features
func WithEnabledFeatures(features []string) ValidatorOption {
	return func(v *ConfigurationValidator) {
		v.enabledFeatures = features
	}
}

// ValidationContext provides context for validation operations
type ValidationContext struct {
	// FieldPath is the current field path being validated
	FieldPath *field.Path
	
	// WorkloadNamespace is the namespace of the workload being validated
	WorkloadNamespace string
	
	// AllowCrossNamespace indicates if cross-namespace references are allowed
	AllowCrossNamespace bool
	
	// AvailableClusters lists clusters available for placement
	AvailableClusters []string
}

// ScalingPolicyValidationResult contains validation results for a scaling policy
type ScalingPolicyValidationResult struct {
	// Valid indicates whether the configuration is valid
	Valid bool
	
	// Errors contains field-specific validation errors
	Errors field.ErrorList
	
	// Warnings contains non-critical validation warnings
	Warnings []string
	
	// RecommendedChanges suggests improvements to the configuration
	RecommendedChanges []string
}

// ValidateWorkloadScalingPolicy validates a complete WorkloadScalingPolicy specification
func (v *ConfigurationValidator) ValidateWorkloadScalingPolicy(spec *WorkloadScalingPolicySpec, ctx *ValidationContext) *ScalingPolicyValidationResult {
	result := &ScalingPolicyValidationResult{
		Valid: true,
		Errors: field.ErrorList{},
		Warnings: []string{},
		RecommendedChanges: []string{},
	}
	
	if spec == nil {
		result.Valid = false
		result.Errors = append(result.Errors, field.Required(ctx.FieldPath, "spec is required"))
		return result
	}
	
	// Validate replica constraints
	v.validateReplicaConstraints(spec, ctx, result)
	
	// Validate workload selector
	v.validateWorkloadSelector(&spec.WorkloadSelector, ctx.FieldPath.Child("workloadSelector"), result)
	
	// Validate cluster selector  
	v.validateClusterSelector(&spec.ClusterSelector, ctx.FieldPath.Child("clusterSelector"), result)
	
	// Validate scaling metrics
	v.validateScalingMetrics(spec.ScalingMetrics, ctx.FieldPath.Child("scalingMetrics"), result)
	
	// Validate scaling behavior
	if spec.ScalingBehavior != nil {
		v.validateScalingBehavior(spec.ScalingBehavior, ctx.FieldPath.Child("scalingBehavior"), result)
	}
	
	// Validate cluster distribution policy
	if spec.ClusterDistribution != nil {
		v.validateClusterDistribution(spec.ClusterDistribution, ctx.FieldPath.Child("clusterDistribution"), result)
	}
	
	// Perform cross-field validation
	v.validateCrossFieldConstraints(spec, ctx, result)
	
	return result
}

// validateReplicaConstraints validates replica-related constraints
func (v *ConfigurationValidator) validateReplicaConstraints(spec *WorkloadScalingPolicySpec, ctx *ValidationContext, result *ScalingPolicyValidationResult) {
	fldPath := ctx.FieldPath
	
	// Validate minimum replicas
	if spec.MinReplicas < 0 {
		result.Valid = false
		result.Errors = append(result.Errors, field.Invalid(fldPath.Child("minReplicas"), spec.MinReplicas, "must be non-negative"))
	}
	
	// Validate maximum replicas  
	if spec.MaxReplicas <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, field.Invalid(fldPath.Child("maxReplicas"), spec.MaxReplicas, "must be positive"))
	}
	
	// Validate max replicas doesn't exceed system limit
	if spec.MaxReplicas > v.maxReplicas {
		result.Valid = false
		result.Errors = append(result.Errors, field.Invalid(fldPath.Child("maxReplicas"), spec.MaxReplicas, 
			fmt.Sprintf("exceeds maximum allowed replicas (%d)", v.maxReplicas)))
	}
	
	// Validate min <= max relationship
	if spec.MinReplicas > spec.MaxReplicas {
		result.Valid = false
		result.Errors = append(result.Errors, field.Invalid(fldPath.Child("minReplicas"), spec.MinReplicas, 
			"must not be greater than maxReplicas"))
	}
	
	// Add warnings for potentially problematic configurations
	if spec.MaxReplicas > 1000 {
		result.Warnings = append(result.Warnings, "maxReplicas is very high, consider if this is necessary")
	}
	
	if spec.MinReplicas == 0 {
		result.Warnings = append(result.Warnings, "minReplicas is 0, workload may be completely scaled down")
	}
}

// validateWorkloadSelector validates the workload selector configuration
func (v *ConfigurationValidator) validateWorkloadSelector(selector *WorkloadSelector, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	if selector == nil {
		result.Valid = false
		result.Errors = append(result.Errors, field.Required(fldPath, "workloadSelector is required"))
		return
	}
	
	// At least one selection method must be specified
	hasSelection := false
	
	if selector.LabelSelector != nil {
		hasSelection = true
		v.validateLabelSelector(selector.LabelSelector, fldPath.Child("labelSelector"), result)
	}
	
	if len(selector.WorkloadTypes) > 0 {
		hasSelection = true
		v.validateWorkloadTypes(selector.WorkloadTypes, fldPath.Child("workloadTypes"), result)
	}
	
	if selector.NamespaceSelector != nil {
		hasSelection = true
		v.validateLabelSelector(selector.NamespaceSelector, fldPath.Child("namespaceSelector"), result)
	}
	
	if !hasSelection {
		result.Valid = false
		result.Errors = append(result.Errors, field.Required(fldPath, 
			"at least one of labelSelector, workloadTypes, or namespaceSelector must be specified"))
	}
}

// validateWorkloadTypes validates workload type specifications
func (v *ConfigurationValidator) validateWorkloadTypes(types []WorkloadType, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	for i, wt := range types {
		typePath := fldPath.Index(i)
		
		if wt.APIVersion == "" {
			result.Valid = false
			result.Errors = append(result.Errors, field.Required(typePath.Child("apiVersion"), "apiVersion is required"))
		} else if !v.isValidAPIVersion(wt.APIVersion) {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(typePath.Child("apiVersion"), wt.APIVersion, 
				"invalid API version format"))
		}
		
		if wt.Kind == "" {
			result.Valid = false
			result.Errors = append(result.Errors, field.Required(typePath.Child("kind"), "kind is required"))
		} else if !v.isValidKind(wt.Kind) {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(typePath.Child("kind"), wt.Kind, 
				"invalid kind format"))
		}
	}
}

// validateClusterSelector validates cluster selector configuration
func (v *ConfigurationValidator) validateClusterSelector(selector *ClusterSelector, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	if selector == nil {
		result.Valid = false
		result.Errors = append(result.Errors, field.Required(fldPath, "clusterSelector is required"))
		return
	}
	
	// At least one selection method must be specified
	hasSelection := false
	
	if selector.LabelSelector != nil {
		hasSelection = true
		v.validateLabelSelector(selector.LabelSelector, fldPath.Child("labelSelector"), result)
	}
	
	if len(selector.LocationSelector) > 0 {
		hasSelection = true
		v.validateLocationSelector(selector.LocationSelector, fldPath.Child("locationSelector"), result)
	}
	
	if len(selector.ClusterNames) > 0 {
		hasSelection = true
		v.validateClusterNames(selector.ClusterNames, fldPath.Child("clusterNames"), result)
	}
	
	if !hasSelection {
		result.Valid = false
		result.Errors = append(result.Errors, field.Required(fldPath, 
			"at least one of labelSelector, locationSelector, or clusterNames must be specified"))
	}
}

// validateScalingMetrics validates scaling metrics configuration
func (v *ConfigurationValidator) validateScalingMetrics(metrics []ScalingMetric, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	if len(metrics) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, field.Required(fldPath, "at least one scaling metric is required"))
		return
	}
	
	// Track metric types to detect duplicates
	seenTypes := make(map[ScalingMetricType]bool)
	
	for i, metric := range metrics {
		metricPath := fldPath.Index(i)
		
		// Validate metric type
		if !v.isValidMetricType(metric.Type) {
			result.Valid = false
			result.Errors = append(result.Errors, field.NotSupported(metricPath.Child("type"), metric.Type, 
				getSupportedMetricTypes()))
		}
		
		// Check for duplicate metric types
		if seenTypes[metric.Type] {
			result.Valid = false
			result.Errors = append(result.Errors, field.Duplicate(metricPath.Child("type"), metric.Type))
		}
		seenTypes[metric.Type] = true
		
		// Validate target value
		v.validateMetricTargetValue(metric.TargetValue, metric.Type, metricPath.Child("targetValue"), result)
		
		// Validate metric selector for custom metrics
		if metric.Type == CustomMetric {
			if metric.MetricSelector == nil {
				result.Valid = false
				result.Errors = append(result.Errors, field.Required(metricPath.Child("metricSelector"), 
					"metricSelector is required for custom metrics"))
			} else {
				v.validateMetricSelector(metric.MetricSelector, metricPath.Child("metricSelector"), result)
			}
		}
	}
	
	// Warn about potentially problematic metric combinations
	if len(metrics) > 3 {
		result.Warnings = append(result.Warnings, "using many scaling metrics may cause instability")
	}
}

// validateScalingBehavior validates scaling behavior policies
func (v *ConfigurationValidator) validateScalingBehavior(behavior *ScalingBehavior, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	if behavior.ScaleUp != nil {
		v.validateScalingDirection(behavior.ScaleUp, fldPath.Child("scaleUp"), result)
	}
	
	if behavior.ScaleDown != nil {
		v.validateScalingDirection(behavior.ScaleDown, fldPath.Child("scaleDown"), result)
	}
	
	// Both directions being disabled would prevent any scaling
	if behavior.ScaleUp != nil && behavior.ScaleDown != nil {
		if v.isScalingDisabled(behavior.ScaleUp) && v.isScalingDisabled(behavior.ScaleDown) {
			result.Warnings = append(result.Warnings, "both scale up and scale down are disabled, no scaling will occur")
		}
	}
}

// validateScalingDirection validates scaling direction policies
func (v *ConfigurationValidator) validateScalingDirection(direction *ScalingDirection, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	// Validate stabilization window
	if direction.StabilizationWindowSeconds != nil {
		if *direction.StabilizationWindowSeconds < 0 {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(fldPath.Child("stabilizationWindowSeconds"), 
				*direction.StabilizationWindowSeconds, "must be non-negative"))
		} else if *direction.StabilizationWindowSeconds > 3600 { // 1 hour
			result.Warnings = append(result.Warnings, "very long stabilization window may delay scaling responses")
		}
	}
	
	// Validate select policy
	if direction.SelectPolicy != nil {
		if !v.isValidSelectPolicy(*direction.SelectPolicy) {
			result.Valid = false
			result.Errors = append(result.Errors, field.NotSupported(fldPath.Child("selectPolicy"), 
				*direction.SelectPolicy, getSupportedSelectPolicies()))
		}
	}
	
	// Validate scaling policies
	if len(direction.Policies) > 0 {
		v.validateScalingPolicies(direction.Policies, fldPath.Child("policies"), result)
	}
}

// isScalingDisabled checks if scaling is disabled in a direction
func (v *ConfigurationValidator) isScalingDisabled(direction *ScalingDirection) bool {
	return direction.SelectPolicy != nil && *direction.SelectPolicy == DisabledPolicySelect
}

// Helper methods for validation

// isValidAPIVersion validates API version format
func (v *ConfigurationValidator) isValidAPIVersion(apiVersion string) bool {
	if apiVersion == "" {
		return false
	}
	
	// Parse as GroupVersion
	_, err := schema.ParseGroupVersion(apiVersion)
	return err == nil
}

// isValidKind validates Kubernetes kind format
func (v *ConfigurationValidator) isValidKind(kind string) bool {
	if kind == "" {
		return false
	}
	
	// Kind should start with uppercase and contain only letters and numbers
	if len(kind) == 0 || (kind[0] < 'A' || kind[0] > 'Z') {
		return false
	}
	
	for _, r := range kind {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	
	return true
}

// isValidMetricType validates scaling metric types
func (v *ConfigurationValidator) isValidMetricType(metricType ScalingMetricType) bool {
	switch metricType {
	case CPUUtilizationMetric, MemoryUtilizationMetric, RequestsPerSecondMetric, QueueLengthMetric, CustomMetric:
		return true
	default:
		return false
	}
}

// getSupportedMetricTypes returns list of supported metric types for error messages
func getSupportedMetricTypes() []string {
	return []string{
		string(CPUUtilizationMetric),
		string(MemoryUtilizationMetric), 
		string(RequestsPerSecondMetric),
		string(QueueLengthMetric),
		string(CustomMetric),
	}
}

// isValidSelectPolicy validates scaling policy select values
func (v *ConfigurationValidator) isValidSelectPolicy(policy ScalingPolicySelect) bool {
	switch policy {
	case MaxPolicySelect, MinPolicySelect, DisabledPolicySelect:
		return true
	default:
		return false
	}
}

// getSupportedSelectPolicies returns list of supported select policies
func getSupportedSelectPolicies() []string {
	return []string{
		string(MaxPolicySelect),
		string(MinPolicySelect),
		string(DisabledPolicySelect),
	}
}

// ExtractErrorMessages extracts error messages from validation result
func (r *ScalingPolicyValidationResult) ExtractErrorMessages() []string {
	messages := make([]string, len(r.Errors))
	for i, err := range r.Errors {
		messages[i] = err.Error()
	}
	return messages
}

// HasCriticalErrors checks if result contains critical validation errors
func (r *ScalingPolicyValidationResult) HasCriticalErrors() bool {
	return !r.Valid || len(r.Errors) > 0
}

// GetSummary returns a summary of the validation result
func (r *ScalingPolicyValidationResult) GetSummary() string {
	var summary strings.Builder
	
	if r.Valid {
		summary.WriteString("Configuration is valid")
	} else {
		summary.WriteString("Configuration has errors")
	}
	
	if len(r.Errors) > 0 {
		summary.WriteString(fmt.Sprintf(" (%d errors)", len(r.Errors)))
	}
	
	if len(r.Warnings) > 0 {
		summary.WriteString(fmt.Sprintf(" (%d warnings)", len(r.Warnings)))
	}
	
	return summary.String()
}