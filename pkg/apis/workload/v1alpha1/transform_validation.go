package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateWorkloadTransform validates a WorkloadTransform
func ValidateWorkloadTransform(transform *WorkloadTransform) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate spec
	allErrs = append(allErrs, validateWorkloadTransformSpec(&transform.Spec, field.NewPath("spec"))...)

	return allErrs
}

func validateWorkloadTransformSpec(spec *WorkloadTransformSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate TargetRef
	if spec.TargetRef.APIVersion == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("targetRef", "apiVersion"), "apiVersion is required"))
	}
	if spec.TargetRef.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("targetRef", "kind"), "kind is required"))
	}

	// Must specify either name or labelSelector
	if spec.TargetRef.Name == "" && spec.TargetRef.LabelSelector == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("targetRef"), "must specify name or labelSelector"))
	}

	// Validate Transforms
	if len(spec.Transforms) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("transforms"), "at least one transform is required"))
	}

	for i, transform := range spec.Transforms {
		allErrs = append(allErrs, validateTransform(&transform, fldPath.Child("transforms").Index(i))...)
	}

	return allErrs
}

func validateTransform(transform *Transform, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate type-specific fields
	switch transform.Type {
	case TransformTypeJSONPatch:
		if len(transform.JSONPatch) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("jsonPatch"), "jsonPatch operations required"))
		}
		for j, op := range transform.JSONPatch {
			if op.Path == "" {
				allErrs = append(allErrs, field.Required(fldPath.Child("jsonPatch").Index(j).Child("path"), "path is required"))
			}
			if op.Op == "copy" || op.Op == "move" {
				if op.From == "" {
					allErrs = append(allErrs, field.Required(fldPath.Child("jsonPatch").Index(j).Child("from"), "from is required for copy/move"))
				}
			}
		}
	case TransformTypeStrategicMerge:
		if transform.StrategicMerge == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("strategicMerge"), "strategicMerge patch required"))
		}
	case TransformTypeReplace:
		if transform.Replace == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("replace"), "replace operation required"))
		} else if transform.Replace.Path == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("replace", "path"), "path is required"))
		}
	case TransformTypeRemove:
		if transform.Remove == nil || len(transform.Remove.Paths) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("remove", "paths"), "paths required for remove"))
		}
	case TransformTypeAnnotate:
		if len(transform.Annotations) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("annotations"), "annotations required"))
		}
	case TransformTypeLabel:
		if len(transform.Labels) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("labels"), "labels required"))
		}
	}

	return allErrs
}