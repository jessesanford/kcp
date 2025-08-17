package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateWorkloadStatusAggregation validates status aggregation
func ValidateWorkloadStatusAggregation(agg *WorkloadStatusAggregation) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate spec
	allErrs = append(allErrs, validateStatusAggregationSpec(&agg.Spec, field.NewPath("spec"))...)

	return allErrs
}

func validateStatusAggregationSpec(spec *WorkloadStatusAggregationSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate WorkloadRef
	if spec.WorkloadRef.APIVersion == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("workloadRef", "apiVersion"), "apiVersion is required"))
	}
	if spec.WorkloadRef.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("workloadRef", "kind"), "kind is required"))
	}
	if spec.WorkloadRef.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("workloadRef", "name"), "name is required"))
	}

	// Validate StatusFields
	if len(spec.StatusFields) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("statusFields"), "at least one status field is required"))
	}

	for i, statusField := range spec.StatusFields {
		fieldPath := fldPath.Child("statusFields").Index(i)
		if statusField.Path == "" {
			allErrs = append(allErrs, field.Required(fieldPath.Child("path"), "path is required"))
		}
		
		// Validate aggregation type
		validTypes := []AggregationType{
			AggregationTypeSum,
			AggregationTypeAverage,
			AggregationTypeMin,
			AggregationTypeMax,
			AggregationTypeLatest,
			AggregationTypeAll,
			AggregationTypeCount,
		}
		
		validType := false
		for _, validAggType := range validTypes {
			if statusField.AggregationType == validAggType {
				validType = true
				break
			}
		}
		
		if !validType {
			allErrs = append(allErrs, field.Invalid(fieldPath.Child("aggregationType"), 
				statusField.AggregationType, "invalid aggregation type"))
		}
	}

	// Validate AggregationPolicy
	if spec.AggregationPolicy != nil {
		if spec.AggregationPolicy.RequireAllLocations && spec.AggregationPolicy.MinLocations > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("aggregationPolicy"),
				spec.AggregationPolicy, "cannot specify both requireAllLocations and minLocations"))
		}
		
		if spec.AggregationPolicy.MinLocations < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("aggregationPolicy", "minLocations"),
				spec.AggregationPolicy.MinLocations, "minLocations must be >= 0"))
		}
		
		// Validate strategy
		validStrategies := []AggregationStrategy{
			AggregationStrategyOptimistic,
			AggregationStrategyPessimistic,
			AggregationStrategyMajority,
		}
		
		validStrategy := false
		for _, validStrat := range validStrategies {
			if spec.AggregationPolicy.Strategy == validStrat {
				validStrategy = true
				break
			}
		}
		
		if !validStrategy {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("aggregationPolicy", "strategy"),
				spec.AggregationPolicy.Strategy, "invalid aggregation strategy"))
		}
	}

	// Validate HealthPolicy
	if spec.HealthPolicy != nil {
		if len(spec.HealthPolicy.HealthyConditions) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("healthPolicy", "healthyConditions"),
				"at least one healthy condition is required"))
		}
		
		// Validate healthy conditions
		for i, condition := range spec.HealthPolicy.HealthyConditions {
			condPath := fldPath.Child("healthPolicy", "healthyConditions").Index(i)
			if condition.Type == "" {
				allErrs = append(allErrs, field.Required(condPath.Child("type"), "condition type is required"))
			}
		}
		
		// Validate unhealthy conditions
		for i, condition := range spec.HealthPolicy.UnhealthyConditions {
			condPath := fldPath.Child("healthPolicy", "unhealthyConditions").Index(i)
			if condition.Type == "" {
				allErrs = append(allErrs, field.Required(condPath.Child("type"), "condition type is required"))
			}
		}
	}

	return allErrs
}