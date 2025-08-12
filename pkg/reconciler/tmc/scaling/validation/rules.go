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
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// TMC API types for validation (local definitions to avoid circular dependencies)

type WorkloadScalingPolicySpec struct {
	WorkloadSelector    WorkloadSelector             `json:"workloadSelector"`
	ClusterSelector     ClusterSelector              `json:"clusterSelector"`
	MinReplicas         int32                        `json:"minReplicas"`
	MaxReplicas         int32                        `json:"maxReplicas"`
	ScalingMetrics      []ScalingMetric              `json:"scalingMetrics"`
	ScalingBehavior     *ScalingBehavior             `json:"scalingBehavior,omitempty"`
	ClusterDistribution *ClusterDistributionPolicy   `json:"clusterDistribution,omitempty"`
}

type WorkloadSelector struct {
	LabelSelector     *metav1.LabelSelector `json:"labelSelector,omitempty"`
	WorkloadTypes     []WorkloadType        `json:"workloadTypes,omitempty"`
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

type WorkloadType struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

type ClusterSelector struct {
	LabelSelector    *metav1.LabelSelector `json:"labelSelector,omitempty"`
	LocationSelector []string              `json:"locationSelector,omitempty"`
	ClusterNames     []string              `json:"clusterNames,omitempty"`
}

type ScalingMetric struct {
	Type           ScalingMetricType `json:"type"`
	TargetValue    intstr.IntOrString `json:"targetValue"`
	MetricSelector *MetricSelector   `json:"metricSelector,omitempty"`
}

type ScalingMetricType string

const (
	CPUUtilizationMetric    ScalingMetricType = "CPUUtilization"
	MemoryUtilizationMetric ScalingMetricType = "MemoryUtilization"
	RequestsPerSecondMetric ScalingMetricType = "RequestsPerSecond"
	QueueLengthMetric       ScalingMetricType = "QueueLength"
	CustomMetric            ScalingMetricType = "Custom"
)

type MetricSelector struct {
	MetricName      string                   `json:"metricName"`
	Selector        *metav1.LabelSelector    `json:"selector,omitempty"`
	AggregationType *MetricAggregationType   `json:"aggregationType,omitempty"`
}

type MetricAggregationType string

const (
	AverageAggregation MetricAggregationType = "Average"
	MaximumAggregation MetricAggregationType = "Maximum"
	MinimumAggregation MetricAggregationType = "Minimum"
	SumAggregation     MetricAggregationType = "Sum"
)

type ScalingBehavior struct {
	ScaleUp   *ScalingDirection `json:"scaleUp,omitempty"`
	ScaleDown *ScalingDirection `json:"scaleDown,omitempty"`
}

type ScalingDirection struct {
	Policies                   []ScalingPolicy      `json:"policies,omitempty"`
	StabilizationWindowSeconds *int32               `json:"stabilizationWindowSeconds,omitempty"`
	SelectPolicy               *ScalingPolicySelect `json:"selectPolicy,omitempty"`
}

type ScalingPolicy struct {
	Type          ScalingPolicyType `json:"type"`
	Value         int32             `json:"value"`
	PeriodSeconds int32             `json:"periodSeconds"`
}

type ScalingPolicyType string
const (
	PodsScalingPolicy    ScalingPolicyType = "Pods"
	PercentScalingPolicy ScalingPolicyType = "Percent"
)

type ScalingPolicySelect string
const (
	MaxPolicySelect     ScalingPolicySelect = "Max"
	MinPolicySelect     ScalingPolicySelect = "Min"
	DisabledPolicySelect ScalingPolicySelect = "Disabled"
)

type ClusterDistributionPolicy struct {
	Strategy              DistributionStrategy  `json:"strategy"`
	Preferences           []ClusterPreference   `json:"preferences,omitempty"`
	MinReplicasPerCluster *int32                `json:"minReplicasPerCluster,omitempty"`
	MaxReplicasPerCluster *int32                `json:"maxReplicasPerCluster,omitempty"`
}

type DistributionStrategy string
const (
	EvenDistribution      DistributionStrategy = "Even"
	WeightedDistribution  DistributionStrategy = "Weighted"
	PreferredDistribution DistributionStrategy = "Preferred"
)

type ClusterPreference struct {
	ClusterName string `json:"clusterName"`
	Weight      int32  `json:"weight"`
}

// Validation rule implementations

func (v *ConfigurationValidator) validateLabelSelector(selector *metav1.LabelSelector, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	if selector == nil {
		return
	}
	
	for key, value := range selector.MatchLabels {
		if !v.isValidLabelKey(key) {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(fldPath.Child("matchLabels").Key(key), key, "invalid label key format"))
		}
		if !v.isValidLabelValue(value) {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(fldPath.Child("matchLabels").Key(key), value, "invalid label value format"))
		}
	}
	
	for i, expr := range selector.MatchExpressions {
		exprPath := fldPath.Child("matchExpressions").Index(i)
		if !v.isValidLabelKey(expr.Key) {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(exprPath.Child("key"), expr.Key, "invalid label key format"))
		}
		if !v.isValidLabelOperator(expr.Operator) {
			result.Valid = false
			result.Errors = append(result.Errors, field.NotSupported(exprPath.Child("operator"), expr.Operator, getSupportedLabelOperators()))
		}
		if v.operatorRequiresValues(expr.Operator) && len(expr.Values) == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, field.Required(exprPath.Child("values"), fmt.Sprintf("values required for operator %s", expr.Operator)))
		}
		for j, value := range expr.Values {
			if !v.isValidLabelValue(value) {
				result.Valid = false
				result.Errors = append(result.Errors, field.Invalid(exprPath.Child("values").Index(j), value, "invalid label value format"))
			}
		}
	}
}

func (v *ConfigurationValidator) validateLocationSelector(locations []string, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	if len(locations) == 0 {
		return
	}
	
	seenLocations := make(map[string]bool)
	for i, location := range locations {
		locPath := fldPath.Index(i)
		if location == "" {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(locPath, location, "location cannot be empty"))
			continue
		}
		if seenLocations[location] {
			result.Valid = false
			result.Errors = append(result.Errors, field.Duplicate(locPath, location))
			continue
		}
		seenLocations[location] = true
		if !v.isValidLocationName(location) {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(locPath, location, "invalid location format, should match pattern: region/zone"))
		}
	}
}

func (v *ConfigurationValidator) validateClusterNames(names []string, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	seenNames := make(map[string]bool)
	for i, name := range names {
		namePath := fldPath.Index(i)
		if name == "" {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(namePath, name, "cluster name cannot be empty"))
			continue
		}
		if seenNames[name] {
			result.Valid = false
			result.Errors = append(result.Errors, field.Duplicate(namePath, name))
			continue
		}
		seenNames[name] = true
		if !v.isValidClusterName(name) {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(namePath, name, "invalid cluster name format"))
		}
	}
	if len(names) > int(v.maxClusters) {
		result.Valid = false
		result.Errors = append(result.Errors, field.TooMany(fldPath, len(names), int(v.maxClusters)))
	}
}

// Additional validation helpers and utilities

func (v *ConfigurationValidator) validateMetricTargetValue(targetValue intstr.IntOrString, metricType ScalingMetricType, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	switch metricType {
	case CPUUtilizationMetric, MemoryUtilizationMetric:
		if targetValue.Type == intstr.String {
			if !strings.HasSuffix(targetValue.StrVal, "%") {
				result.Valid = false
				result.Errors = append(result.Errors, field.Invalid(fldPath, targetValue, "utilization metrics should specify percentage values (e.g., '80%')"))
				return
			}
			percentStr := strings.TrimSuffix(targetValue.StrVal, "%")
			if !v.isValidPercentage(percentStr) {
				result.Valid = false
				result.Errors = append(result.Errors, field.Invalid(fldPath, targetValue, "invalid percentage value, must be between 1-100"))
			}
		} else {
			if targetValue.IntVal < 1 || targetValue.IntVal > 100 {
				result.Valid = false
				result.Errors = append(result.Errors, field.Invalid(fldPath, targetValue, "utilization percentage must be between 1-100"))
			}
		}
	case RequestsPerSecondMetric, QueueLengthMetric:
		if targetValue.Type == intstr.String {
			result.Warnings = append(result.Warnings, "string values for rate/count metrics may cause unexpected behavior")
		} else {
			if targetValue.IntVal <= 0 {
				result.Valid = false
				result.Errors = append(result.Errors, field.Invalid(fldPath, targetValue, "rate/count metrics must have positive target values"))
			}
		}
	}
}

func (v *ConfigurationValidator) validateMetricSelector(selector *MetricSelector, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	if selector.MetricName == "" {
		result.Valid = false
		result.Errors = append(result.Errors, field.Required(fldPath.Child("metricName"), "metricName is required"))
	}
}

func (v *ConfigurationValidator) validateScalingPolicies(policies []ScalingPolicy, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	for i, policy := range policies {
		policyPath := fldPath.Index(i)
		if !v.isValidPolicyType(policy.Type) {
			result.Valid = false
			result.Errors = append(result.Errors, field.NotSupported(policyPath.Child("type"), policy.Type, getSupportedPolicyTypes()))
		}
		if policy.Value <= 0 {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(policyPath.Child("value"), policy.Value, "policy value must be positive"))
		}
		if policy.PeriodSeconds <= 0 {
			result.Valid = false
			result.Errors = append(result.Errors, field.Invalid(policyPath.Child("periodSeconds"), policy.PeriodSeconds, "period must be positive"))
		}
	}
}

func (v *ConfigurationValidator) validateClusterDistribution(distribution *ClusterDistributionPolicy, fldPath *field.Path, result *ScalingPolicyValidationResult) {
	if !v.isValidDistributionStrategy(distribution.Strategy) {
		result.Valid = false
		result.Errors = append(result.Errors, field.NotSupported(fldPath.Child("strategy"), distribution.Strategy, getSupportedDistributionStrategies()))
	}
}

func (v *ConfigurationValidator) validateCrossFieldConstraints(spec *WorkloadScalingPolicySpec, ctx *ValidationContext, result *ScalingPolicyValidationResult) {
	// Cross-field validation logic here
}

// Utility validation functions

func (v *ConfigurationValidator) isValidLabelKey(key string) bool {
	return len(key) <= 253 && regexp.MustCompile(`^[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?)*$`).MatchString(key)
}

func (v *ConfigurationValidator) isValidLabelValue(value string) bool {
	return len(value) <= 63 && (value == "" || regexp.MustCompile(`^[a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?$`).MatchString(value))
}

func (v *ConfigurationValidator) isValidLabelOperator(op metav1.LabelSelectorOperator) bool {
	return op == metav1.LabelSelectorOpIn || op == metav1.LabelSelectorOpNotIn || op == metav1.LabelSelectorOpExists || op == metav1.LabelSelectorOpDoesNotExist
}

func (v *ConfigurationValidator) operatorRequiresValues(op metav1.LabelSelectorOperator) bool {
	return op == metav1.LabelSelectorOpIn || op == metav1.LabelSelectorOpNotIn
}

func (v *ConfigurationValidator) isValidLocationName(location string) bool {
	parts := strings.Split(location, "/")
	return len(parts) == 2 && regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`).MatchString(parts[0]) && regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`).MatchString(parts[1])
}

func (v *ConfigurationValidator) isValidClusterName(name string) bool {
	return len(name) <= 253 && regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`).MatchString(name)
}

func (v *ConfigurationValidator) isValidPercentage(percentStr string) bool {
	if len(percentStr) == 0 {
		return false
	}
	var val int
	for _, r := range percentStr {
		if r < '0' || r > '9' {
			return false
		}
		val = val*10 + int(r-'0')
	}
	return val >= 1 && val <= 100
}

func (v *ConfigurationValidator) isValidAggregationType(aggType MetricAggregationType) bool {
	return aggType == AverageAggregation || aggType == MaximumAggregation || aggType == MinimumAggregation || aggType == SumAggregation
}

func (v *ConfigurationValidator) isValidPolicyType(policyType ScalingPolicyType) bool {
	return policyType == PodsScalingPolicy || policyType == PercentScalingPolicy
}

func (v *ConfigurationValidator) isValidDistributionStrategy(strategy DistributionStrategy) bool {
	return strategy == EvenDistribution || strategy == WeightedDistribution || strategy == PreferredDistribution
}

func getSupportedLabelOperators() []string {
	return []string{"In", "NotIn", "Exists", "DoesNotExist"}
}

func getSupportedAggregationTypes() []string {
	return []string{"Average", "Maximum", "Minimum", "Sum"}
}

func getSupportedPolicyTypes() []string {
	return []string{"Pods", "Percent"}
}

func getSupportedDistributionStrategies() []string {
	return []string{"Even", "Weighted", "Preferred"}
}