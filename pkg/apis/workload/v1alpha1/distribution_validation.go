package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateWorkloadDistribution validates a WorkloadDistribution resource
func ValidateWorkloadDistribution(dist *WorkloadDistribution) field.ErrorList {
	return validateWorkloadDistributionSpec(&dist.Spec, field.NewPath("spec"))
}

// ValidateWorkloadDistributionUpdate validates updates to a WorkloadDistribution resource
func ValidateWorkloadDistributionUpdate(newDist, oldDist *WorkloadDistribution) field.ErrorList {
	allErrs := ValidateWorkloadDistribution(newDist)

	// Validate immutable workload reference fields
	oldRef, newRef := oldDist.Spec.WorkloadRef, newDist.Spec.WorkloadRef
	refPath := field.NewPath("spec", "workloadRef")
	if newRef.APIVersion != oldRef.APIVersion {
		allErrs = append(allErrs, field.Invalid(refPath.Child("apiVersion"), newRef.APIVersion, "field is immutable"))
	}
	if newRef.Kind != oldRef.Kind {
		allErrs = append(allErrs, field.Invalid(refPath.Child("kind"), newRef.Kind, "field is immutable"))
	}
	if newRef.Name != oldRef.Name {
		allErrs = append(allErrs, field.Invalid(refPath.Child("name"), newRef.Name, "field is immutable"))
	}
	if newRef.Namespace != oldRef.Namespace {
		allErrs = append(allErrs, field.Invalid(refPath.Child("namespace"), newRef.Namespace, "field is immutable"))
	}

	return allErrs
}

func validateWorkloadDistributionSpec(spec *WorkloadDistributionSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate WorkloadRef (inline validation)
	refPath := fldPath.Child("workloadRef")
	if spec.WorkloadRef.APIVersion == "" {
		allErrs = append(allErrs, field.Required(refPath.Child("apiVersion"), "apiVersion is required"))
	}
	if spec.WorkloadRef.Kind == "" {
		allErrs = append(allErrs, field.Required(refPath.Child("kind"), "kind is required"))
	}
	if spec.WorkloadRef.Name == "" {
		allErrs = append(allErrs, field.Required(refPath.Child("name"), "name is required"))
	}

	// Validate TotalReplicas
	if spec.TotalReplicas < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("totalReplicas"), spec.TotalReplicas, "must be non-negative"))
	}

	// Validate Distributions (inline validation)
	totalExplicit := int32(0)
	locationNames := make(map[string]bool)
	for i, dist := range spec.Distributions {
		distPath := fldPath.Child("distributions").Index(i)
		
		// Inline location distribution validation
		if dist.LocationName == "" {
			allErrs = append(allErrs, field.Required(distPath.Child("locationName"), "locationName is required"))
		}
		if dist.Replicas < 0 {
			allErrs = append(allErrs, field.Invalid(distPath.Child("replicas"), dist.Replicas, "replicas must be non-negative"))
		}
		if dist.Priority != nil && (*dist.Priority < 0 || *dist.Priority > 100) {
			allErrs = append(allErrs, field.Invalid(distPath.Child("priority"), *dist.Priority, "priority must be between 0 and 100"))
		}

		if locationNames[dist.LocationName] {
			allErrs = append(allErrs, field.Duplicate(distPath.Child("locationName"), dist.LocationName))
		}
		locationNames[dist.LocationName] = true
		totalExplicit += dist.Replicas
	}

	// Validate total replicas consistency
	if len(spec.Distributions) > 0 && totalExplicit != spec.TotalReplicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("totalReplicas"), spec.TotalReplicas, "total replicas must match sum of explicit distributions"))
	}

	// Validate that either placement policy or explicit distributions are provided
	if spec.PlacementPolicyRef == nil && len(spec.Distributions) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, spec, "either placementPolicyRef or distributions must be specified"))
	}

	// Validate RolloutStrategy
	if spec.RolloutStrategy != nil {
		allErrs = append(allErrs, validateRolloutStrategy(spec.RolloutStrategy, fldPath.Child("rolloutStrategy"))...)
	}

	// Validate ResourceOverrides (simplified)
	overrideLocations := make(map[string]bool)
	for i, override := range spec.ResourceOverrides {
		overridePath := fldPath.Child("resourceOverrides").Index(i)
		if override.LocationName == "" {
			allErrs = append(allErrs, field.Required(overridePath.Child("locationName"), "locationName is required"))
		}
		if overrideLocations[override.LocationName] {
			allErrs = append(allErrs, field.Duplicate(overridePath.Child("locationName"), override.LocationName))
		}
		overrideLocations[override.LocationName] = true
	}

	return allErrs
}

func validateRolloutStrategy(strategy *RolloutStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch strategy.Type {
	case RolloutTypeRollingUpdate:
		if strategy.RollingUpdate == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("rollingUpdate"), "rollingUpdate config required"))
		} else if strategy.RollingUpdate.Partition != nil && *strategy.RollingUpdate.Partition < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("rollingUpdate", "partition"), *strategy.RollingUpdate.Partition, "partition must be non-negative"))
		}
		if strategy.BlueGreen != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("blueGreen"), "blueGreen config not allowed for RollingUpdate strategy"))
		}
	case RolloutTypeBlueGreen:
		if strategy.BlueGreen == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("blueGreen"), "blueGreen config required"))
		} else {
			if strategy.BlueGreen.ActiveService == "" {
				allErrs = append(allErrs, field.Required(fldPath.Child("blueGreen", "activeService"), "activeService is required"))
			}
			if strategy.BlueGreen.ScaleDownDelaySeconds != nil && *strategy.BlueGreen.ScaleDownDelaySeconds < 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("blueGreen", "scaleDownDelaySeconds"), *strategy.BlueGreen.ScaleDownDelaySeconds, "scaleDownDelaySeconds must be non-negative"))
			}
		}
		if strategy.RollingUpdate != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("rollingUpdate"), "rollingUpdate config not allowed for BlueGreen strategy"))
		}
	case RolloutTypeRecreate:
		if strategy.RollingUpdate != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("rollingUpdate"), "rollingUpdate config not allowed for Recreate strategy"))
		}
		if strategy.BlueGreen != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("blueGreen"), "blueGreen config not allowed for Recreate strategy"))
		}
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), strategy.Type, []string{string(RolloutTypeRollingUpdate), string(RolloutTypeRecreate), string(RolloutTypeBlueGreen)}))
	}

	return allErrs
}