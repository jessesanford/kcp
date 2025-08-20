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
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	// ValidKubernetesNameRegex matches valid Kubernetes resource names
	ValidKubernetesNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	
	// ValidClusterNameRegex matches valid cluster names (allows dots for FQDN)
	ValidClusterNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)
)

// ValidateTMCConfig validates a TMCConfig resource.
func ValidateTMCConfig(config *TMCConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	
	// Validate metadata
	if config.Name == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("metadata", "name"), "name is required"))
	} else if !ValidKubernetesNameRegex.MatchString(config.Name) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "name"), config.Name, "must be a valid Kubernetes resource name"))
	}
	
	// Validate spec
	allErrs = append(allErrs, ValidateTMCConfigSpec(&config.Spec, field.NewPath("spec"))...)
	
	return allErrs
}

// ValidateTMCConfigSpec validates a TMCConfigSpec.
func ValidateTMCConfigSpec(spec *TMCConfigSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	
	// Validate feature flags
	if spec.FeatureFlags != nil {
		for flag := range spec.FeatureFlags {
			if flag == "" {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("featureFlags"), flag, "feature flag name cannot be empty"))
			}
		}
	}
	
	return allErrs
}

// ValidateResourceIdentifier validates a ResourceIdentifier.
func ValidateResourceIdentifier(id *ResourceIdentifier, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	
	// Group can be empty for core resources, so we don't require it
	
	// Version is required
	if id.Version == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "version is required"))
	} else if !isValidAPIVersion(id.Version) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("version"), id.Version, "must be a valid API version (e.g., v1, v1alpha1, v1beta1)"))
	}
	
	// Resource is required
	if id.Resource == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("resource"), "resource is required"))
	} else if !ValidKubernetesNameRegex.MatchString(id.Resource) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("resource"), id.Resource, "must be a valid Kubernetes resource name"))
	}
	
	// Optional namespace validation
	if id.Namespace != "" && !ValidKubernetesNameRegex.MatchString(id.Namespace) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), id.Namespace, "must be a valid Kubernetes namespace name"))
	}
	
	// Optional name validation
	if id.Name != "" && !ValidKubernetesNameRegex.MatchString(id.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), id.Name, "must be a valid Kubernetes resource name"))
	}
	
	return allErrs
}

// ValidateClusterIdentifier validates a ClusterIdentifier.
func ValidateClusterIdentifier(id *ClusterIdentifier, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	
	// Name is required
	if id.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name is required"))
	} else if !ValidClusterNameRegex.MatchString(id.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), id.Name, "must be a valid cluster name"))
	}
	
	// Validate optional fields
	if id.Region != "" && !ValidKubernetesNameRegex.MatchString(id.Region) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("region"), id.Region, "must be a valid region name"))
	}
	
	if id.Zone != "" && !ValidKubernetesNameRegex.MatchString(id.Zone) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("zone"), id.Zone, "must be a valid zone name"))
	}
	
	if id.Provider != "" && !isValidProvider(id.Provider) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("provider"), id.Provider, "must be a valid cloud provider"))
	}
	
	if id.Environment != "" && !isValidEnvironment(id.Environment) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("environment"), id.Environment, "must be a valid environment type"))
	}
	
	// Validate labels
	for key, value := range id.Labels {
		if key == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("labels"), key, "label key cannot be empty"))
		}
		if len(key) > 253 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("labels"), key, "label key must be no more than 253 characters"))
		}
		if len(value) > 253 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("labels"), value, "label value must be no more than 253 characters"))
		}
	}
	
	return allErrs
}

// isValidAPIVersion checks if the version follows Kubernetes API version format.
func isValidAPIVersion(version string) bool {
	// Match patterns like v1, v1alpha1, v1beta1, etc.
	versionPattern := regexp.MustCompile(`^v\d+(alpha\d+|beta\d+)?$`)
	return versionPattern.MatchString(version)
}

// isValidProvider checks if the provider is a known cloud provider.
func isValidProvider(provider string) bool {
	validProviders := map[string]bool{
		"aws":    true,
		"gcp":    true,
		"azure":  true,
		"alibaba": true,
		"ibm":    true,
		"oracle": true,
		"baremetal": true,
		"onprem": true,
	}
	return validProviders[strings.ToLower(provider)]
}

// isValidEnvironment checks if the environment is a valid type.
func isValidEnvironment(env string) bool {
	validEnvironments := map[string]bool{
		"prod":       true,
		"production": true,
		"staging":    true,
		"dev":        true,
		"development": true,
		"test":       true,
		"testing":    true,
		"qa":         true,
		"sandbox":    true,
	}
	return validEnvironments[strings.ToLower(env)]
}

// ValidateTMCStatus validates a TMCStatus.
func ValidateTMCStatus(status *TMCStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	
	// Validate phase if present
	if status.Phase != "" && !isValidPhase(status.Phase) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("phase"), status.Phase, "must be a valid phase"))
	}
	
	// Validate conditions
	conditionTypes := make(map[string]bool)
	for i, condition := range status.Conditions {
		conditionPath := fldPath.Child("conditions").Index(i)
		
		// Check for duplicate condition types
		if conditionTypes[condition.Type] {
			allErrs = append(allErrs, field.Duplicate(conditionPath.Child("type"), condition.Type))
		}
		conditionTypes[condition.Type] = true
		
		// Validate condition fields
		if condition.Type == "" {
			allErrs = append(allErrs, field.Required(conditionPath.Child("type"), "type is required"))
		}
		
		if condition.Status == "" {
			allErrs = append(allErrs, field.Required(conditionPath.Child("status"), "status is required"))
		}
	}
	
	return allErrs
}

// isValidPhase checks if the phase is a valid TMC resource phase.
func isValidPhase(phase string) bool {
	validPhases := map[string]bool{
		"Pending":     true,
		"Running":     true,
		"Succeeded":   true,
		"Failed":      true,
		"Unknown":     true,
		"Terminating": true,
	}
	return validPhases[phase]
}