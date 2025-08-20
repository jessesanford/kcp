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
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	validSyncModes = map[string]bool{
		"push":          true,
		"pull":          true,
		"bidirectional": true,
	}

	// resourceTypePattern validates Kubernetes resource type names
	resourceTypePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?\.[a-z]{2,})?$`)
)

// ValidateSyncTarget validates a SyncTarget object.
//
// +kubebuilder:webhook:path=/validate-workload-kcp-io-v1alpha1-synctarget,mutating=false,failurePolicy=fail,sideEffects=None,groups=workload.kcp.io,resources=synctargets,verbs=create;update,versions=v1alpha1,name=vsynctarget.kb.io,admissionReviewVersions=v1
func ValidateSyncTarget(syncTarget *SyncTarget) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateSyncTargetSpec(&syncTarget.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateSyncTargetUpdate validates an update to a SyncTarget object.
func ValidateSyncTargetUpdate(syncTarget, oldSyncTarget *SyncTarget) field.ErrorList {
	allErrs := ValidateSyncTarget(syncTarget)

	// Validate that immutable fields haven't changed
	specPath := field.NewPath("spec")
	if syncTarget.Spec.ClusterRef.Name != oldSyncTarget.Spec.ClusterRef.Name {
		allErrs = append(allErrs, field.Invalid(specPath.Child("clusterRef", "name"), syncTarget.Spec.ClusterRef.Name, "clusterRef.name is immutable"))
	}

	if syncTarget.Spec.ClusterRef.Workspace != oldSyncTarget.Spec.ClusterRef.Workspace {
		allErrs = append(allErrs, field.Invalid(specPath.Child("clusterRef", "workspace"), syncTarget.Spec.ClusterRef.Workspace, "clusterRef.workspace is immutable"))
	}

	return allErrs
}

// validateSyncTargetSpec validates the spec of a SyncTarget.
func validateSyncTargetSpec(spec *SyncTargetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate ClusterRef
	allErrs = append(allErrs, validateClusterReference(&spec.ClusterRef, fldPath.Child("clusterRef"))...)

	// Validate SyncerConfig
	if spec.SyncerConfig != nil {
		allErrs = append(allErrs, validateSyncerConfig(spec.SyncerConfig, fldPath.Child("syncerConfig"))...)
	}

	// Validate ResourceQuotas
	if spec.ResourceQuotas != nil {
		allErrs = append(allErrs, validateResourceQuotas(spec.ResourceQuotas, fldPath.Child("resourceQuotas"))...)
	}

	// Validate Selector
	if spec.Selector != nil {
		allErrs = append(allErrs, validateWorkloadSelector(spec.Selector, fldPath.Child("selector"))...)
	}

	// Validate SupportedResourceTypes
	allErrs = append(allErrs, validateSupportedResourceTypes(spec.SupportedResourceTypes, fldPath.Child("supportedResourceTypes"))...)

	return allErrs
}

// validateClusterReference validates a ClusterReference.
func validateClusterReference(clusterRef *ClusterReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if clusterRef.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name is required"))
	}

	// Validate name follows Kubernetes naming conventions
	if len(clusterRef.Name) > 253 {
		allErrs = append(allErrs, field.TooLong(fldPath.Child("name"), clusterRef.Name, 253))
	}

	// Workspace name validation (if specified)
	if clusterRef.Workspace != "" {
		if len(clusterRef.Workspace) > 253 {
			allErrs = append(allErrs, field.TooLong(fldPath.Child("workspace"), clusterRef.Workspace, 253))
		}
	}

	return allErrs
}

// validateSyncerConfig validates SyncerConfig fields.
func validateSyncerConfig(config *SyncerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate SyncMode
	if config.SyncMode != "" && !validSyncModes[config.SyncMode] {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("syncMode"), config.SyncMode, []string{"push", "pull", "bidirectional"}))
	}

	// Validate SyncInterval
	if config.SyncInterval != "" {
		if _, err := time.ParseDuration(config.SyncInterval); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("syncInterval"), config.SyncInterval, fmt.Sprintf("invalid duration: %v", err)))
		}
	}

	// Validate RetryBackoff
	if config.RetryBackoff != nil {
		allErrs = append(allErrs, validateRetryBackoffConfig(config.RetryBackoff, fldPath.Child("retryBackoff"))...)
	}

	return allErrs
}

// validateRetryBackoffConfig validates RetryBackoffConfig fields.
func validateRetryBackoffConfig(backoff *RetryBackoffConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if backoff.InitialInterval != "" {
		if _, err := time.ParseDuration(backoff.InitialInterval); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("initialInterval"), backoff.InitialInterval, fmt.Sprintf("invalid duration: %v", err)))
		}
	}

	if backoff.MaxInterval != "" {
		if _, err := time.ParseDuration(backoff.MaxInterval); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("maxInterval"), backoff.MaxInterval, fmt.Sprintf("invalid duration: %v", err)))
		}
	}

	if backoff.Multiplier != 0 && backoff.Multiplier <= 1.0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("multiplier"), backoff.Multiplier, "multiplier must be greater than 1.0"))
	}

	return allErrs
}

// validateResourceQuotas validates ResourceQuotas fields.
func validateResourceQuotas(quotas *ResourceQuotas, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate CPU quota
	if quotas.CPU != nil {
		if quotas.CPU.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("cpu"), quotas.CPU.String(), "CPU quota must be positive"))
		}
	}

	// Validate Memory quota
	if quotas.Memory != nil {
		if quotas.Memory.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("memory"), quotas.Memory.String(), "memory quota must be positive"))
		}
	}

	// Validate Storage quota
	if quotas.Storage != nil {
		if quotas.Storage.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("storage"), quotas.Storage.String(), "storage quota must be positive"))
		}
	}

	// Validate Pods quota
	if quotas.Pods != nil {
		if quotas.Pods.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("pods"), quotas.Pods.String(), "pods quota must be positive"))
		}
	}

	// Validate Custom quotas
	for name, quota := range quotas.Custom {
		if quota.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("custom", name), quota.String(), "custom quota must be positive"))
		}
		// Validate resource name format
		if _, err := resource.ParseQuantity(quota.String()); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("custom", name), quota.String(), fmt.Sprintf("invalid resource quantity: %v", err)))
		}
	}

	return allErrs
}

// validateWorkloadSelector validates WorkloadSelector fields.
func validateWorkloadSelector(selector *WorkloadSelector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate MatchLabels
	for key, value := range selector.MatchLabels {
		if len(key) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("matchLabels"), key, "label key cannot be empty"))
		}
		if len(key) > 63 {
			allErrs = append(allErrs, field.TooLong(fldPath.Child("matchLabels"), key, 63))
		}
		if len(value) > 63 {
			allErrs = append(allErrs, field.TooLong(fldPath.Child("matchLabels"), value, 63))
		}
	}

	// Validate Locations
	for i, location := range selector.Locations {
		if location == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("locations").Index(i), location, "location cannot be empty"))
		}
		if len(location) > 253 {
			allErrs = append(allErrs, field.TooLong(fldPath.Child("locations").Index(i), location, 253))
		}
	}

	return allErrs
}

// validateSupportedResourceTypes validates the supportedResourceTypes field.
func validateSupportedResourceTypes(resourceTypes []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	seen := make(map[string]bool)
	for i, resourceType := range resourceTypes {
		if resourceType == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), resourceType, "resource type cannot be empty"))
			continue
		}

		// Check for duplicates
		if seen[resourceType] {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i), resourceType))
			continue
		}
		seen[resourceType] = true

		// Validate resource type format
		if !isValidResourceType(resourceType) {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), resourceType, "invalid resource type format"))
		}
	}

	return allErrs
}

// isValidResourceType checks if a resource type follows valid conventions.
func isValidResourceType(resourceType string) bool {
	// Allow both simple names (like "pods", "services") and group-qualified names (like "deployments.apps")
	if strings.Contains(resourceType, ".") {
		return resourceTypePattern.MatchString(resourceType)
	}
	// Simple resource names should be lowercase with no special characters
	return regexp.MustCompile(`^[a-z][a-z0-9]*$`).MatchString(resourceType)
}