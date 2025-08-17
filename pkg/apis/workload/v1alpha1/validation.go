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
	"net/url"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateSyncTarget validates a SyncTarget object and returns any validation errors.
// It performs comprehensive validation of the SyncTarget spec and metadata.
func ValidateSyncTarget(target *SyncTarget) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate object metadata
	allErrs = append(allErrs, validateSyncTargetMetadata(&target.ObjectMeta, field.NewPath("metadata"))...)

	// Validate spec
	allErrs = append(allErrs, validateSyncTargetSpec(&target.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateSyncTargetUpdate validates an update to a SyncTarget object.
// It ensures that immutable fields are not changed and that the update is valid.
func ValidateSyncTargetUpdate(newTarget, oldTarget *SyncTarget) field.ErrorList {
	allErrs := ValidateSyncTarget(newTarget)

	// Validate that name hasn't changed (this should be caught by apiserver, but let's be explicit)
	if newTarget.Name != oldTarget.Name {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "name"),
			newTarget.Name, "name is immutable"))
	}

	return allErrs
}

// validateSyncTargetMetadata validates the metadata of a SyncTarget
func validateSyncTargetMetadata(metadata *metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate name
	if metadata.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name is required"))
	} else {
		// Validate name format follows DNS-1123 subdomain rules
		if errs := validation.IsDNS1123Subdomain(metadata.Name); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"),
				metadata.Name, fmt.Sprintf("name must be a valid DNS subdomain: %s", strings.Join(errs, ", "))))
		}
	}

	return allErrs
}

// validateSyncTargetSpec validates the spec of a SyncTarget
func validateSyncTargetSpec(spec *SyncTargetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate cells (required and at least one)
	if len(spec.Cells) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("cells"), "at least one cell is required"))
	} else {
		cellNames := make(map[string]bool)
		for i, cell := range spec.Cells {
			cellPath := fldPath.Child("cells").Index(i)
			allErrs = append(allErrs, validateCell(&cell, cellPath)...)

			// Check for duplicate cell names
			if cellNames[cell.Name] {
				allErrs = append(allErrs, field.Duplicate(cellPath.Child("name"), cell.Name))
			}
			cellNames[cell.Name] = true
		}
	}

	// Validate supported API exports
	for i, apiExport := range spec.SupportedAPIExports {
		apiExportPath := fldPath.Child("supportedAPIExports").Index(i)
		allErrs = append(allErrs, validateAPIExportReference(&apiExport, apiExportPath)...)
	}

	// Validate EvictAfter duration if specified
	if spec.EvictAfter != nil {
		if spec.EvictAfter.Duration < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("evictAfter"),
				spec.EvictAfter.Duration.String(), "evictAfter duration must be non-negative"))
		}
	}

	return allErrs
}

// validateCell validates a cell within a SyncTarget spec
func validateCell(cell *Cell, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate cell name
	if cell.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "cell name is required"))
	} else {
		// Cell name should follow DNS-1123 label rules
		if errs := validation.IsDNS1123Label(cell.Name); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"),
				cell.Name, fmt.Sprintf("cell name must be a valid DNS label: %s", strings.Join(errs, ", "))))
		}
	}

	// Validate labels
	for key, value := range cell.Labels {
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("labels").Key(key),
				key, fmt.Sprintf("label key is invalid: %s", strings.Join(errs, ", "))))
		}
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("labels").Key(key),
				value, fmt.Sprintf("label value is invalid: %s", strings.Join(errs, ", "))))
		}
	}

	// Validate taints
	taintKeys := make(map[string]bool)
	for i, taint := range cell.Taints {
		taintPath := fldPath.Child("taints").Index(i)
		allErrs = append(allErrs, validateTaint(&taint, taintPath)...)

		// Check for duplicate taint keys
		if taintKeys[taint.Key] {
			allErrs = append(allErrs, field.Duplicate(taintPath.Child("key"), taint.Key))
		}
		taintKeys[taint.Key] = true
	}

	return allErrs
}

// validateTaint validates a taint on a cell
func validateTaint(taint *Taint, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate taint key
	if taint.Key == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("key"), "taint key is required"))
	} else {
		if errs := validation.IsQualifiedName(taint.Key); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("key"),
				taint.Key, fmt.Sprintf("taint key is invalid: %s", strings.Join(errs, ", "))))
		}
	}

	// Validate taint value (optional, but if present must be valid)
	if taint.Value != "" {
		if errs := validation.IsValidLabelValue(taint.Value); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("value"),
				taint.Value, fmt.Sprintf("taint value is invalid: %s", strings.Join(errs, ", "))))
		}
	}

	// Validate taint effect
	if taint.Effect == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("effect"), "taint effect is required"))
	} else {
		switch taint.Effect {
		case TaintEffectNoSchedule, TaintEffectPreferNoSchedule, TaintEffectNoExecute:
			// Valid effects
		default:
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("effect"),
				taint.Effect, []string{string(TaintEffectNoSchedule), string(TaintEffectPreferNoSchedule), string(TaintEffectNoExecute)}))
		}
	}

	return allErrs
}

// validateAPIExportReference validates an APIExport reference
func validateAPIExportReference(ref *APIExportReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate workspace
	if ref.Workspace == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("workspace"), "workspace is required"))
	} else {
		// Workspace should be a valid logical cluster path
		if !isValidLogicalClusterPath(ref.Workspace) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("workspace"),
				ref.Workspace, "workspace must be a valid logical cluster path"))
		}
	}

	// Validate name
	if ref.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "APIExport name is required"))
	} else {
		if errs := validation.IsDNS1123Subdomain(ref.Name); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"),
				ref.Name, fmt.Sprintf("APIExport name must be a valid DNS subdomain: %s", strings.Join(errs, ", "))))
		}
	}

	return allErrs
}

// ValidateSyncTargetStatus validates the status of a SyncTarget
func ValidateSyncTargetStatus(status *SyncTargetStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate virtual workspaces
	for i, vw := range status.VirtualWorkspaces {
		vwPath := fldPath.Child("virtualWorkspaces").Index(i)
		allErrs = append(allErrs, validateVirtualWorkspace(&vw, vwPath)...)
	}

	// Validate syncer identity if present
	if status.SyncerIdentity != "" {
		if errs := validation.IsDNS1123Subdomain(status.SyncerIdentity); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("syncerIdentity"),
				status.SyncerIdentity, fmt.Sprintf("syncer identity must be a valid DNS subdomain: %s", strings.Join(errs, ", "))))
		}
	}

	return allErrs
}

// validateVirtualWorkspace validates a virtual workspace entry
func validateVirtualWorkspace(vw *VirtualWorkspace, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if vw.URL == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("url"), "virtual workspace URL is required"))
	} else {
		// Validate URL format
		if _, err := url.Parse(vw.URL); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("url"),
				vw.URL, fmt.Sprintf("virtual workspace URL is invalid: %v", err)))
		}
	}

	return allErrs
}

// isValidLogicalClusterPath checks if a string is a valid logical cluster path
func isValidLogicalClusterPath(path string) bool {
	if path == "" {
		return false
	}

	// Basic validation - should start with root: or be a simple path
	if strings.HasPrefix(path, "root:") {
		return len(path) > 5 // More than just "root:"
	}

	// Simple path validation - should be valid DNS names separated by colons
	parts := strings.Split(path, ":")
	for _, part := range parts {
		if part == "" {
			return false
		}
		if errs := validation.IsDNS1123Label(part); len(errs) > 0 {
			return false
		}
	}

	return true
}
