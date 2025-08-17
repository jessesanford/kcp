package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidatePlacementPolicy validates a PlacementPolicy object for semantic correctness.
// It checks that all required fields are present, values are within acceptable ranges,
// and cross-field constraints are satisfied.
func ValidatePlacementPolicy(policy *PlacementPolicy) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate spec
	allErrs = append(allErrs, ValidatePlacementPolicySpec(&policy.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidatePlacementPolicySpec validates the specification section of a PlacementPolicy.
// It ensures the policy configuration is semantically valid and internally consistent.
func ValidatePlacementPolicySpec(spec *PlacementPolicySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate TargetWorkload - required field
	allErrs = append(allErrs, validateWorkloadSelector(&spec.TargetWorkload, fldPath.Child("targetWorkload"))...)

	// Validate Strategy - required field with enum constraint
	if spec.Strategy == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("strategy"), "placement strategy is required"))
	} else {
		allErrs = append(allErrs, validatePlacementStrategy(spec.Strategy, fldPath.Child("strategy"))...)
	}

	// Validate strategy-specific constraints
	allErrs = append(allErrs, validateStrategyConstraints(spec, fldPath)...)

	// Validate LocationSelectors if specified
	for i, selector := range spec.LocationSelectors {
		allErrs = append(allErrs, validateLocationSelector(&selector, fldPath.Child("locationSelectors").Index(i))...)
	}

	// Validate Tolerations if specified
	for i, toleration := range spec.Tolerations {
		allErrs = append(allErrs, validateToleration(&toleration, fldPath.Child("tolerations").Index(i))...)
	}

	// Validate SpreadConstraints if specified
	for i, constraint := range spec.SpreadConstraints {
		allErrs = append(allErrs, validateSpreadConstraint(&constraint, fldPath.Child("spreadConstraints").Index(i))...)
	}

	// Validate AffinityRules if specified
	if spec.AffinityRules != nil {
		allErrs = append(allErrs, validateAffinityRules(spec.AffinityRules, fldPath.Child("affinityRules"))...)
	}

	return allErrs
}

// validateWorkloadSelector validates that the workload selector has required fields.
func validateWorkloadSelector(selector *WorkloadSelector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if selector.APIVersion == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("apiVersion"), "apiVersion is required for workload targeting"))
	}

	if selector.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "kind is required for workload targeting"))
	}

	// Validate label selector if provided
	if selector.LabelSelector != nil {
		_, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("labelSelector"), selector.LabelSelector, fmt.Sprintf("invalid label selector: %v", err)))
		}
	}

	return allErrs
}

// validatePlacementStrategy validates that the placement strategy is one of the allowed values.
func validatePlacementStrategy(strategy PlacementStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	validStrategies := []PlacementStrategy{
		PlacementStrategySingleton,
		PlacementStrategyHighAvailability,
		PlacementStrategySpread,
		PlacementStrategyBinpack,
	}

	valid := false
	for _, validStrategy := range validStrategies {
		if strategy == validStrategy {
			valid = true
			break
		}
	}

	if !valid {
		allErrs = append(allErrs, field.NotSupported(fldPath, strategy, []string{
			string(PlacementStrategySingleton),
			string(PlacementStrategyHighAvailability),
			string(PlacementStrategySpread),
			string(PlacementStrategyBinpack),
		}))
	}

	return allErrs
}

// validateStrategyConstraints validates strategy-specific replica constraints.
func validateStrategyConstraints(spec *PlacementPolicySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if spec.Replicas == nil {
		return allErrs // defaults will be applied
	}

	replicas := *spec.Replicas

	switch spec.Strategy {
	case PlacementStrategySingleton:
		if replicas > 1 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), replicas,
				"singleton strategy requires replicas to be 0 or 1"))
		}

	case PlacementStrategyHighAvailability:
		if replicas < 2 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), replicas,
				"high availability strategy requires replicas >= 2 for redundancy"))
		}

	case PlacementStrategySpread:
		// No specific constraints for spread strategy

	case PlacementStrategyBinpack:
		// No specific constraints for binpack strategy
	}

	return allErrs
}

// validateLocationSelector validates location selector configuration.
func validateLocationSelector(selector *LocationSelector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// At least one selection method must be specified
	if selector.Name == "" && selector.LabelSelector == nil && selector.CellSelector == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, selector,
			"at least one of name, labelSelector, or cellSelector must be specified"))
	}

	// Validate label selector if provided
	if selector.LabelSelector != nil {
		_, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("labelSelector"), selector.LabelSelector,
				fmt.Sprintf("invalid label selector: %v", err)))
		}
	}

	// Validate cell selector if provided
	if selector.CellSelector != nil {
		allErrs = append(allErrs, validateCellSelector(selector.CellSelector, fldPath.Child("cellSelector"))...)
	}

	return allErrs
}

// validateCellSelector validates cell selector configuration.
func validateCellSelector(selector *CellSelector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// At least one selection method must be specified
	if len(selector.MatchLabels) == 0 && len(selector.RequiredDuringScheduling) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, selector,
			"at least one of matchLabels or requiredDuringScheduling must be specified"))
	}

	return allErrs
}

// validateToleration validates toleration configuration.
func validateToleration(toleration *Toleration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if toleration.Key == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("key"), "toleration key is required"))
	}

	// Validate operator if specified
	if toleration.Operator != "" {
		validOperators := []TolerationOperator{TolerationOpEqual, TolerationOpExists}
		valid := false
		for _, validOp := range validOperators {
			if toleration.Operator == validOp {
				valid = true
				break
			}
		}
		if !valid {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("operator"), toleration.Operator,
				[]string{string(TolerationOpEqual), string(TolerationOpExists)}))
		}

		// Value is only valid for Equal operator
		if toleration.Operator == TolerationOpExists && toleration.Value != "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("value"), toleration.Value,
				"value must be empty when operator is Exists"))
		}
	}

	return allErrs
}

// validateSpreadConstraint validates spread constraint configuration.
func validateSpreadConstraint(constraint *SpreadConstraint, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if constraint.TopologyKey == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("topologyKey"),
			"topologyKey is required for spread constraint"))
	}

	if constraint.MaxSkew < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxSkew"), constraint.MaxSkew,
			"maxSkew must be greater than 0"))
	}

	// Validate WhenUnsatisfiable enum
	validActions := []UnsatisfiableConstraintAction{DoNotSchedule, ScheduleAnyway}
	valid := false
	for _, validAction := range validActions {
		if constraint.WhenUnsatisfiable == validAction {
			valid = true
			break
		}
	}
	if !valid {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("whenUnsatisfiable"), constraint.WhenUnsatisfiable,
			[]string{string(DoNotSchedule), string(ScheduleAnyway)}))
	}

	return allErrs
}

// validateAffinityRules validates affinity and anti-affinity rules.
func validateAffinityRules(rules *AffinityRules, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate workload affinity terms
	for i, term := range rules.WorkloadAffinity {
		allErrs = append(allErrs, validateWorkloadAffinityTerm(&term, fldPath.Child("workloadAffinity").Index(i))...)
	}

	// Validate workload anti-affinity terms
	for i, term := range rules.WorkloadAntiAffinity {
		allErrs = append(allErrs, validateWorkloadAffinityTerm(&term, fldPath.Child("workloadAntiAffinity").Index(i))...)
	}

	return allErrs
}

// validateWorkloadAffinityTerm validates a workload affinity term.
func validateWorkloadAffinityTerm(term *WorkloadAffinityTerm, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if term.LabelSelector == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("labelSelector"),
			"labelSelector is required for workload affinity"))
	} else {
		_, err := metav1.LabelSelectorAsSelector(term.LabelSelector)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("labelSelector"), term.LabelSelector,
				fmt.Sprintf("invalid label selector: %v", err)))
		}
	}

	if term.TopologyKey == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("topologyKey"),
			"topologyKey is required for workload affinity"))
	}

	// Validate weight if specified
	if term.Weight != nil {
		weight := *term.Weight
		if weight < 1 || weight > 100 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("weight"), weight,
				"weight must be between 1 and 100"))
		}
	}

	return allErrs
}
