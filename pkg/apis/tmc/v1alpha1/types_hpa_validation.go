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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// DefaultHorizontalPodAutoscalerPolicy sets default values for HPA policy.
func DefaultHorizontalPodAutoscalerPolicy(policy *HorizontalPodAutoscalerPolicy) {
	if policy.Spec.Strategy == "" {
		policy.Spec.Strategy = DistributedAutoScaling
	}

	if policy.Spec.MinReplicas == nil {
		minReplicas := int32(1)
		policy.Spec.MinReplicas = &minReplicas
	}

	if policy.Spec.ScaleDownPolicy == nil {
		defaultPolicy := BalancedScaleDown
		policy.Spec.ScaleDownPolicy = &defaultPolicy
	}

	if policy.Spec.ScaleUpPolicy == nil {
		defaultPolicy := LoadAwareScaleUp
		policy.Spec.ScaleUpPolicy = &defaultPolicy
	}

	// Set default behavior if not specified
	if policy.Spec.Behavior == nil {
		policy.Spec.Behavior = &HorizontalPodAutoscalerBehavior{}
	}

	// Default scale-up stabilization window
	if policy.Spec.Behavior.ScaleUp == nil {
		policy.Spec.Behavior.ScaleUp = &HPAScalingRules{}
	}
	if policy.Spec.Behavior.ScaleUp.StabilizationWindowSeconds == nil {
		stabilization := int32(0) // Scale up immediately
		policy.Spec.Behavior.ScaleUp.StabilizationWindowSeconds = &stabilization
	}

	// Default scale-down stabilization window
	if policy.Spec.Behavior.ScaleDown == nil {
		policy.Spec.Behavior.ScaleDown = &HPAScalingRules{}
	}
	if policy.Spec.Behavior.ScaleDown.StabilizationWindowSeconds == nil {
		stabilization := int32(300) // 5 minutes
		policy.Spec.Behavior.ScaleDown.StabilizationWindowSeconds = &stabilization
	}

	// Set default policies if none specified
	if len(policy.Spec.Behavior.ScaleUp.Policies) == 0 {
		policy.Spec.Behavior.ScaleUp.Policies = []HPAScalingPolicy{
			{
				Type:          PercentScalingPolicy,
				Value:         100, // Double replicas
				PeriodSeconds: 15,
			},
			{
				Type:          PodsScalingPolicy,
				Value:         4, // Add 4 pods
				PeriodSeconds: 15,
			},
		}
		maxPolicy := MaxPolicySelect
		policy.Spec.Behavior.ScaleUp.SelectPolicy = &maxPolicy
	}

	if len(policy.Spec.Behavior.ScaleDown.Policies) == 0 {
		policy.Spec.Behavior.ScaleDown.Policies = []HPAScalingPolicy{
			{
				Type:          PercentScalingPolicy,
				Value:         100, // Remove all replicas if needed
				PeriodSeconds: 15,
			},
		}
		maxPolicy := MaxPolicySelect
		policy.Spec.Behavior.ScaleDown.SelectPolicy = &maxPolicy
	}
}

// ValidateHorizontalPodAutoscalerPolicy validates HPA policy fields.
func ValidateHorizontalPodAutoscalerPolicy(policy *HorizontalPodAutoscalerPolicy) field.ErrorList {
	allErrs := field.ErrorList{}
	specPath := field.NewPath("spec")

	// Validate strategy
	if err := validateStrategy(policy.Spec.Strategy, specPath.Child("strategy")); err != nil {
		allErrs = append(allErrs, err)
	}

	// Validate replica constraints
	if errs := validateReplicaConstraints(policy.Spec, specPath); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	// Validate target reference
	if errs := validateTargetReference(policy.Spec.TargetRef, specPath.Child("targetRef")); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	// Validate metrics
	if errs := validateMetrics(policy.Spec.Metrics, specPath.Child("metrics")); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	// Validate behavior
	if policy.Spec.Behavior != nil {
		if errs := validateBehavior(*policy.Spec.Behavior, specPath.Child("behavior")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	// Validate cluster selector
	if policy.Spec.ClusterSelector != nil {
		if errs := metav1.ValidateLabelSelector(policy.Spec.ClusterSelector, metav1.LabelSelectorValidationOptions{}, specPath.Child("clusterSelector")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	return allErrs
}

// validateStrategy validates the auto-scaling strategy.
func validateStrategy(strategy AutoScalingStrategy, path *field.Path) *field.Error {
	switch strategy {
	case DistributedAutoScaling, CentralizedAutoScaling, HybridAutoScaling:
		return nil
	default:
		return field.Invalid(path, strategy, "must be one of: Distributed, Centralized, Hybrid")
	}
}

// validateReplicaConstraints validates min/max replica constraints.
func validateReplicaConstraints(spec HorizontalPodAutoscalerPolicySpec, specPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if spec.MinReplicas != nil && *spec.MinReplicas < 1 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("minReplicas"), *spec.MinReplicas, "must be greater than 0"))
	}

	if spec.MaxReplicas < 1 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("maxReplicas"), spec.MaxReplicas, "must be greater than 0"))
	}

	if spec.MinReplicas != nil && *spec.MinReplicas > spec.MaxReplicas {
		allErrs = append(allErrs, field.Invalid(specPath.Child("minReplicas"), *spec.MinReplicas, "must be less than or equal to maxReplicas"))
	}

	return allErrs
}

// validateTargetReference validates the target workload reference.
func validateTargetReference(ref CrossClusterObjectReference, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if ref.APIVersion == "" {
		allErrs = append(allErrs, field.Required(path.Child("apiVersion"), "must specify apiVersion"))
	}

	if ref.Kind == "" {
		allErrs = append(allErrs, field.Required(path.Child("kind"), "must specify kind"))
	}

	if ref.Name == "" {
		allErrs = append(allErrs, field.Required(path.Child("name"), "must specify name"))
	}

	// Validate supported kinds
	supportedKinds := []string{"Deployment", "ReplicaSet", "StatefulSet", "DaemonSet"}
	if ref.Kind != "" && !contains(supportedKinds, ref.Kind) {
		allErrs = append(allErrs, field.NotSupported(path.Child("kind"), ref.Kind, supportedKinds))
	}

	return allErrs
}

// validateMetrics validates metric specifications.
func validateMetrics(metrics []MetricSpec, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(metrics) == 0 {
		allErrs = append(allErrs, field.Required(path, "must specify at least one metric"))
	}

	for i, metric := range metrics {
		metricPath := path.Index(i)

		if errs := validateMetricSpec(metric, metricPath); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	return allErrs
}

// validateMetricSpec validates a single metric specification.
func validateMetricSpec(metric MetricSpec, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate type
	switch metric.Type {
	case ResourceMetricSourceType:
		if metric.Resource == nil {
			allErrs = append(allErrs, field.Required(path.Child("resource"), "must specify resource when type is Resource"))
		} else if errs := validateResourceMetric(*metric.Resource, path.Child("resource")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	case PodsMetricSourceType:
		if metric.Pods == nil {
			allErrs = append(allErrs, field.Required(path.Child("pods"), "must specify pods when type is Pods"))
		} else if errs := validatePodsMetric(*metric.Pods, path.Child("pods")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	case ObjectMetricSourceType:
		if metric.Object == nil {
			allErrs = append(allErrs, field.Required(path.Child("object"), "must specify object when type is Object"))
		} else if errs := validateObjectMetric(*metric.Object, path.Child("object")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	case ExternalMetricSourceType:
		if metric.External == nil {
			allErrs = append(allErrs, field.Required(path.Child("external"), "must specify external when type is External"))
		} else if errs := validateExternalMetric(*metric.External, path.Child("external")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	case ContainerResourceMetricSourceType:
		if metric.ContainerResource == nil {
			allErrs = append(allErrs, field.Required(path.Child("containerResource"), "must specify containerResource when type is ContainerResource"))
		} else if errs := validateContainerResourceMetric(*metric.ContainerResource, path.Child("containerResource")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	default:
		allErrs = append(allErrs, field.NotSupported(path.Child("type"), metric.Type, []string{
			string(ResourceMetricSourceType),
			string(PodsMetricSourceType),
			string(ObjectMetricSourceType),
			string(ExternalMetricSourceType),
			string(ContainerResourceMetricSourceType),
		}))
	}

	return allErrs
}

// validateResourceMetric validates resource metric source.
func validateResourceMetric(resource ResourceMetricSource, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if resource.Name == "" {
		allErrs = append(allErrs, field.Required(path.Child("name"), "must specify resource name"))
	}

	if errs := validateMetricTarget(resource.Target, path.Child("target")); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	return allErrs
}

// validatePodsMetric validates pods metric source.
func validatePodsMetric(pods PodsMetricSource, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if pods.Metric.Name == "" {
		allErrs = append(allErrs, field.Required(path.Child("metric", "name"), "must specify metric name"))
	}

	if errs := validateMetricTarget(pods.Target, path.Child("target")); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	return allErrs
}

// validateObjectMetric validates object metric source.
func validateObjectMetric(object ObjectMetricSource, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if object.Metric.Name == "" {
		allErrs = append(allErrs, field.Required(path.Child("metric", "name"), "must specify metric name"))
	}

	if errs := validateTargetReference(object.DescribedObject, path.Child("describedObject")); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if errs := validateMetricTarget(object.Target, path.Child("target")); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	return allErrs
}

// validateExternalMetric validates external metric source.
func validateExternalMetric(external ExternalMetricSource, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if external.Metric.Name == "" {
		allErrs = append(allErrs, field.Required(path.Child("metric", "name"), "must specify metric name"))
	}

	if errs := validateMetricTarget(external.Target, path.Child("target")); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	return allErrs
}

// validateContainerResourceMetric validates container resource metric source.
func validateContainerResourceMetric(containerResource ContainerResourceMetricSource, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if containerResource.Name == "" {
		allErrs = append(allErrs, field.Required(path.Child("name"), "must specify resource name"))
	}

	if containerResource.Container == "" {
		allErrs = append(allErrs, field.Required(path.Child("container"), "must specify container name"))
	}

	if errs := validateMetricTarget(containerResource.Target, path.Child("target")); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	return allErrs
}

// validateMetricTarget validates metric target specification.
func validateMetricTarget(target MetricTarget, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch target.Type {
	case UtilizationMetricType:
		if target.AverageUtilization == nil {
			allErrs = append(allErrs, field.Required(path.Child("averageUtilization"), "must specify averageUtilization when type is Utilization"))
		} else if *target.AverageUtilization < 1 || *target.AverageUtilization > 100 {
			allErrs = append(allErrs, field.Invalid(path.Child("averageUtilization"), *target.AverageUtilization, "must be between 1 and 100"))
		}
	case ValueMetricType:
		if target.Value == nil {
			allErrs = append(allErrs, field.Required(path.Child("value"), "must specify value when type is Value"))
		}
	case AverageValueMetricType:
		if target.AverageValue == nil {
			allErrs = append(allErrs, field.Required(path.Child("averageValue"), "must specify averageValue when type is AverageValue"))
		}
	default:
		allErrs = append(allErrs, field.NotSupported(path.Child("type"), target.Type, []string{
			string(UtilizationMetricType),
			string(ValueMetricType),
			string(AverageValueMetricType),
		}))
	}

	return allErrs
}

// validateBehavior validates scaling behavior configuration.
func validateBehavior(behavior HorizontalPodAutoscalerBehavior, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if behavior.ScaleUp != nil {
		if errs := validateScalingRules(*behavior.ScaleUp, path.Child("scaleUp")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	if behavior.ScaleDown != nil {
		if errs := validateScalingRules(*behavior.ScaleDown, path.Child("scaleDown")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	return allErrs
}

// validateScalingRules validates scaling rules configuration.
func validateScalingRules(rules HPAScalingRules, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if rules.StabilizationWindowSeconds != nil && *rules.StabilizationWindowSeconds < 0 {
		allErrs = append(allErrs, field.Invalid(path.Child("stabilizationWindowSeconds"), *rules.StabilizationWindowSeconds, "must be non-negative"))
	}

	if rules.SelectPolicy != nil {
		switch *rules.SelectPolicy {
		case MaxPolicySelect, MinPolicySelect, DisabledPolicySelect:
			// Valid
		default:
			allErrs = append(allErrs, field.NotSupported(path.Child("selectPolicy"), *rules.SelectPolicy, []string{
				string(MaxPolicySelect),
				string(MinPolicySelect),
				string(DisabledPolicySelect),
			}))
		}
	}

	for i, policy := range rules.Policies {
		policyPath := path.Child("policies").Index(i)

		if errs := validateScalingPolicy(policy, policyPath); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	return allErrs
}

// validateScalingPolicy validates individual scaling policy.
func validateScalingPolicy(policy HPAScalingPolicy, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch policy.Type {
	case PodsScalingPolicy, PercentScalingPolicy:
		// Valid
	default:
		allErrs = append(allErrs, field.NotSupported(path.Child("type"), policy.Type, []string{
			string(PodsScalingPolicy),
			string(PercentScalingPolicy),
		}))
	}

	if policy.Value <= 0 {
		allErrs = append(allErrs, field.Invalid(path.Child("value"), policy.Value, "must be greater than 0"))
	}

	if policy.PeriodSeconds <= 0 {
		allErrs = append(allErrs, field.Invalid(path.Child("periodSeconds"), policy.PeriodSeconds, "must be greater than 0"))
	}

	return allErrs
}

// contains checks if a string slice contains a specific string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetCondition returns the condition with the specified type.
func GetCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// SetCondition sets the condition on the status, replacing any existing condition of the same type.
func SetCondition(conditions *[]metav1.Condition, condition metav1.Condition) {
	if conditions == nil {
		*conditions = []metav1.Condition{}
	}

	existingCondition := GetCondition(*conditions, condition.Type)
	if existingCondition == nil {
		condition.LastTransitionTime = metav1.Now()
		*conditions = append(*conditions, condition)
		return
	}

	if existingCondition.Status != condition.Status {
		existingCondition.Status = condition.Status
		existingCondition.LastTransitionTime = metav1.Now()
	}

	existingCondition.Reason = condition.Reason
	existingCondition.Message = condition.Message
	existingCondition.ObservedGeneration = condition.ObservedGeneration
}

// RemoveCondition removes the condition with the specified type.
func RemoveCondition(conditions *[]metav1.Condition, conditionType string) {
	if conditions == nil {
		return
	}

	for i, condition := range *conditions {
		if condition.Type == conditionType {
			*conditions = append((*conditions)[:i], (*conditions)[i+1:]...)
			return
		}
	}
}

// ConditionReasons for HorizontalPodAutoscalerPolicy conditions.
const (
	// Ready condition reasons
	ReasonPolicyReady     = "PolicyReady"
	ReasonPolicyNotReady  = "PolicyNotReady"

	// Active condition reasons  
	ReasonScalingActive   = "ScalingActive"
	ReasonScalingInactive = "ScalingInactive"

	// TargetFound condition reasons
	ReasonTargetFound    = "TargetFound"
	ReasonTargetNotFound = "TargetNotFound"

	// MetricsAvailable condition reasons
	ReasonMetricsAvailable    = "MetricsAvailable"
	ReasonMetricsNotAvailable = "MetricsNotAvailable"

	// ScalingLimited condition reasons
	ReasonScalingLimited      = "ScalingLimited"
	ReasonScalingNotLimited   = "ScalingNotLimited"
)

// Policy validation helpers for common use cases.

// ValidateScaleTarget validates if a target is suitable for scaling.
func ValidateScaleTarget(ref CrossClusterObjectReference) error {
	switch ref.Kind {
	case "Deployment", "ReplicaSet", "StatefulSet":
		return nil
	default:
		return fmt.Errorf("unsupported target kind %q for auto-scaling", ref.Kind)
	}
}

// NormalizeMetricName normalizes metric names for consistency.
func NormalizeMetricName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", "-"))
}

// IsValidResourceName checks if a resource name is valid for scaling metrics.
func IsValidResourceName(name string) bool {
	validResources := []string{"cpu", "memory", "ephemeral-storage"}
	return contains(validResources, name)
}