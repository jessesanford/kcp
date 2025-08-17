/*
Copyright The KCP Authors.

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

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateAPIDiscovery validates an APIDiscovery
func ValidateAPIDiscovery(discovery *APIDiscovery) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate spec
	allErrs = append(allErrs, validateAPIDiscoverySpec(&discovery.Spec, field.NewPath("spec"))...)

	return allErrs
}

func validateAPIDiscoverySpec(spec *APIDiscoverySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate LocationRef
	if spec.LocationRef.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("locationRef", "name"), "location name is required"))
	}

	// Validate DiscoveryPolicy
	if spec.DiscoveryPolicy != nil {
		allErrs = append(allErrs, validateDiscoveryPolicy(spec.DiscoveryPolicy, fldPath.Child("discoveryPolicy"))...)
	}

	// Validate RefreshInterval
	if spec.RefreshInterval != nil && spec.RefreshInterval.Duration.Nanoseconds() <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("refreshInterval"), spec.RefreshInterval,
			"refresh interval must be positive"))
	}

	return allErrs
}

func validateDiscoveryPolicy(policy *DiscoveryPolicy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate resource filters
	seenFilters := make(map[string]bool)
	for i, filter := range policy.ResourceFilters {
		filterPath := fldPath.Child("resourceFilters").Index(i)

		// At least one selector must be specified
		if filter.Group == "" && filter.Version == "" && filter.Kind == "" {
			allErrs = append(allErrs, field.Required(filterPath, "at least one of group, version, or kind must be specified"))
		}

		// Check for duplicates
		key := fmt.Sprintf("%s/%s/%s", filter.Group, filter.Version, filter.Kind)
		if seenFilters[key] {
			allErrs = append(allErrs, field.Duplicate(filterPath, key))
		}
		seenFilters[key] = true
	}

	// Validate scope-specific requirements
	if policy.Scope == DiscoveryScopeMinimal && (policy.IncludeAlpha || policy.IncludeBeta) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("scope"), policy.Scope,
			"minimal scope cannot include alpha or beta APIs"))
	}

	return allErrs
}

// ValidateNegotiatedAPIResource validates a NegotiatedAPIResource
func ValidateNegotiatedAPIResource(resource *NegotiatedAPIResource) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate spec
	allErrs = append(allErrs, validateNegotiatedAPIResourceSpec(&resource.Spec, field.NewPath("spec"))...)

	return allErrs
}

func validateNegotiatedAPIResourceSpec(spec *NegotiatedAPIResourceSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate LocationRef
	if spec.LocationRef.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("locationRef", "name"), "location name is required"))
	}

	// Validate API reference
	allErrs = append(allErrs, validateAPIResourceRef(&spec.API, fldPath.Child("api"))...)

	// Validate requirements if specified
	if spec.Requirements != nil {
		allErrs = append(allErrs, validateAPIRequirements(spec.Requirements, fldPath.Child("requirements"))...)
	}

	return allErrs
}

func validateAPIResourceRef(api *APIResourceRef, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if api.Version == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "version is required"))
	}

	if api.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "kind is required"))
	}

	if api.Resource == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("resource"), "resource name is required"))
	}

	return allErrs
}

func validateAPIRequirements(req *APIRequirements, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate required verbs
	validVerbs := map[string]bool{
		"get": true, "list": true, "watch": true, "create": true,
		"update": true, "patch": true, "delete": true, "deletecollection": true,
	}

	for i, verb := range req.RequiredVerbs {
		if !validVerbs[verb] {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("requiredVerbs").Index(i), verb,
				"invalid verb"))
		}
	}

	return allErrs
}
