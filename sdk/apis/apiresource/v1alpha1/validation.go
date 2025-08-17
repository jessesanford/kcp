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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	// validResourceName matches valid Kubernetes resource names (RFC 1123 subdomain)
	validResourceName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	// validKindName matches valid Kubernetes Kind names (must start with uppercase)
	validKindName = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)
	// validFieldPath matches valid JSONPath expressions
	validFieldPath = regexp.MustCompile(`^\.[a-zA-Z0-9\[\]\.]+$`)
)

// ValidateNegotiatedAPIResource validates a complete NegotiatedAPIResource object.
func ValidateNegotiatedAPIResource(resource *NegotiatedAPIResource) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate metadata
	allErrs = append(allErrs, validateObjectMeta(&resource.ObjectMeta, field.NewPath("metadata"))...)

	// Validate spec
	allErrs = append(allErrs, validateNegotiatedAPIResourceSpec(&resource.Spec, field.NewPath("spec"))...)

	// Validate status consistency
	allErrs = append(allErrs, validateNegotiatedAPIResourceStatus(&resource.Status, field.NewPath("status"))...)

	return allErrs
}

// validateObjectMeta validates the ObjectMeta for NegotiatedAPIResource.
func validateObjectMeta(meta *metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if meta.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name is required"))
	}

	// Validate name follows Kubernetes naming conventions
	if meta.Name != "" && !validResourceName.MatchString(meta.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), meta.Name, "name must follow Kubernetes naming conventions"))
	}

	return allErrs
}

// validateNegotiatedAPIResourceSpec validates the spec section.
func validateNegotiatedAPIResourceSpec(spec *NegotiatedAPIResourceSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate GroupVersion
	allErrs = append(allErrs, validateGroupVersion(spec.GroupVersion, fldPath.Child("groupVersion"))...)

	// Validate Resources
	if len(spec.Resources) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("resources"), "at least one resource is required"))
	}

	resourceNames := make(map[string]bool)
	for i, res := range spec.Resources {
		resPath := fldPath.Child("resources").Index(i)
		allErrs = append(allErrs, validateResourceNegotiation(&res, resPath)...)

		// Check for duplicate resource names
		if resourceNames[res.Resource] {
			allErrs = append(allErrs, field.Duplicate(resPath.Child("resource"), res.Resource))
		}
		resourceNames[res.Resource] = true
	}

	return allErrs
}

// validateGroupVersion validates a GroupVersionSpec.
func validateGroupVersion(gv GroupVersionSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if gv.Version == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "version is required"))
	}

	// Validate version format (should match Kubernetes version format)
	if gv.Version != "" && !regexp.MustCompile(`^v\d+([a-z]+\d*)?$`).MatchString(gv.Version) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("version"), gv.Version, "version must follow Kubernetes version format (e.g., v1, v1alpha1, v1beta1)"))
	}

	// Group can be empty for core APIs, but if present should be valid
	if gv.Group != "" && !regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`).MatchString(gv.Group) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("group"), gv.Group, "group must be a valid DNS subdomain"))
	}

	return allErrs
}

// validateResourceNegotiation validates a single ResourceNegotiation.
func validateResourceNegotiation(res *ResourceNegotiation, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate resource name
	if res.Resource == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("resource"), "resource name is required"))
	} else if !validResourceName.MatchString(res.Resource) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("resource"), res.Resource, "resource name must follow Kubernetes naming conventions"))
	}

	// Validate kind name
	if res.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "kind is required"))
	} else if !validKindName.MatchString(res.Kind) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), res.Kind, "kind must start with uppercase letter and contain only alphanumeric characters"))
	}

	// Validate scope
	if res.Scope != ClusterScoped && res.Scope != NamespacedScoped {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("scope"), res.Scope, []string{string(ClusterScoped), string(NamespacedScoped)}))
	}

	// Validate subresources
	for i, subresource := range res.Subresources {
		if subresource == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("subresources").Index(i), "subresource name cannot be empty"))
		} else if !validResourceName.MatchString(subresource) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("subresources").Index(i), subresource, "subresource name must follow Kubernetes naming conventions"))
		}
	}

	// Validate required fields
	for i, field := range res.RequiredFields {
		fieldPath := fldPath.Child("requiredFields").Index(i)
		allErrs = append(allErrs, validateFieldRequirement(&field, fieldPath)...)
	}

	return allErrs
}

// validateFieldRequirement validates a FieldRequirement.
func validateFieldRequirement(req *FieldRequirement, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate path
	if req.Path == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("path"), "field path is required"))
	} else if !validFieldPath.MatchString(req.Path) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("path"), req.Path, "field path must be a valid JSONPath starting with '.'"))
	}

	// Validate type
	if req.Type == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field type is required"))
	} else {
		validTypes := []string{"string", "integer", "number", "boolean", "object", "array"}
		isValid := false
		for _, validType := range validTypes {
			if req.Type == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), req.Type, validTypes))
		}
	}

	return allErrs
}

// validateNegotiatedAPIResourceStatus validates the status section.
func validateNegotiatedAPIResourceStatus(status *NegotiatedAPIResourceStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate phase if set
	if status.Phase != "" {
		validPhases := []string{
			string(NegotiationPending),
			string(NegotiationNegotiating),
			string(NegotiationCompatible),
			string(NegotiationIncompatible),
		}
		isValid := false
		for _, validPhase := range validPhases {
			if string(status.Phase) == validPhase {
				isValid = true
				break
			}
		}
		if !isValid {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("phase"), status.Phase, validPhases))
		}
	}

	// Validate compatible locations
	locationNames := make(map[string]bool)
	for i, location := range status.CompatibleLocations {
		locPath := fldPath.Child("compatibleLocations").Index(i)
		allErrs = append(allErrs, validateCompatibleLocation(&location, locPath)...)

		// Check for duplicate location names
		if locationNames[location.Name] {
			allErrs = append(allErrs, field.Duplicate(locPath.Child("name"), location.Name))
		}
		locationNames[location.Name] = true
	}

	// Validate incompatible locations and check for conflicts with compatible ones
	for i, location := range status.IncompatibleLocations {
		locPath := fldPath.Child("incompatibleLocations").Index(i)
		allErrs = append(allErrs, validateIncompatibleLocation(&location, locPath)...)

		// Check for conflict with compatible locations
		if locationNames[location.Name] {
			allErrs = append(allErrs, field.Forbidden(locPath.Child("name"), fmt.Sprintf("location %s cannot be both compatible and incompatible", location.Name)))
		}
	}

	return allErrs
}

// validateCompatibleLocation validates a CompatibleLocation.
func validateCompatibleLocation(location *CompatibleLocation, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if location.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "location name is required"))
	}

	// Validate supported versions
	for i, version := range location.SupportedVersions {
		if version == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("supportedVersions").Index(i), "version cannot be empty"))
		}
	}

	// Validate constraints
	for i, constraint := range location.Constraints {
		constraintPath := fldPath.Child("constraints").Index(i)
		if constraint.Type == "" {
			allErrs = append(allErrs, field.Required(constraintPath.Child("type"), "constraint type is required"))
		}
		if constraint.Value == "" {
			allErrs = append(allErrs, field.Required(constraintPath.Child("value"), "constraint value is required"))
		}
	}

	return allErrs
}

// validateIncompatibleLocation validates an IncompatibleLocation.
func validateIncompatibleLocation(location *IncompatibleLocation, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if location.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "location name is required"))
	}

	if location.Reason == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("reason"), "incompatibility reason is required"))
	}

	return allErrs
}
