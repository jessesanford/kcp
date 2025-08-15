package v1alpha1

import (
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateAPIResource validates an APIResource object
func ValidateAPIResource(ar *APIResource) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate spec
	allErrs = append(allErrs, ValidateAPIResourceSpec(&ar.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateAPIResourceSpec validates an APIResourceSpec
func ValidateAPIResourceSpec(spec *APIResourceSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate GroupVersion
	if spec.GroupVersion.Group == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("groupVersion", "group"), "group is required"))
	}
	if spec.GroupVersion.Version == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("groupVersion", "version"), "version is required"))
	}

	// Validate Resources
	if len(spec.Resources) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("resources"), "at least one resource is required"))
	}

	for i, resource := range spec.Resources {
		allErrs = append(allErrs, ValidateResourceDefinition(&resource, fldPath.Child("resources").Index(i))...)
	}

	// Validate VirtualWorkspace reference
	if spec.VirtualWorkspace.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("virtualWorkspace", "name"), "virtual workspace name is required"))
	}

	return allErrs
}

// ValidateResourceDefinition validates a ResourceDefinition
func ValidateResourceDefinition(rd *ResourceDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate required fields
	if rd.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "resource name is required"))
	}
	if rd.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "kind is required"))
	}
	if len(rd.Verbs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("verbs"), "at least one verb is required"))
	}

	// Validate verbs
	validVerbs := map[string]bool{
		"get": true, "list": true, "watch": true,
		"create": true, "update": true, "patch": true,
		"delete": true, "deletecollection": true,
	}

	for i, verb := range rd.Verbs {
		if !validVerbs[strings.ToLower(verb)] {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("verbs").Index(i), verb, "invalid verb"))
		}
	}

	return allErrs
}

// ValidateVirtualWorkspace validates a VirtualWorkspace object
func ValidateVirtualWorkspace(vw *VirtualWorkspace) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate spec
	allErrs = append(allErrs, ValidateVirtualWorkspaceSpec(&vw.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateVirtualWorkspaceSpec validates a VirtualWorkspaceSpec
func ValidateVirtualWorkspaceSpec(spec *VirtualWorkspaceSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate URL
	if spec.URL == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("url"), "URL is required"))
	} else if _, err := url.Parse(spec.URL); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("url"), spec.URL, "invalid URL"))
	}

	// Validate rate limiting configuration if present
	if spec.RateLimiting != nil {
		allErrs = append(allErrs, ValidateRateLimitConfig(spec.RateLimiting, fldPath.Child("rateLimiting"))...)
	}

	// Validate caching configuration if present
	if spec.Caching != nil {
		allErrs = append(allErrs, ValidateCacheConfig(spec.Caching, fldPath.Child("caching"))...)
	}

	return allErrs
}

// ValidateRateLimitConfig validates rate limiting configuration
func ValidateRateLimitConfig(config *RateLimitConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.QPS <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("qps"), config.QPS, "QPS must be positive"))
	}

	if config.Burst <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("burst"), config.Burst, "burst must be positive"))
	}

	return allErrs
}

// ValidateCacheConfig validates cache configuration
func ValidateCacheConfig(config *CacheConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.TTLSeconds <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ttlSeconds"), config.TTLSeconds, "TTL must be positive"))
	}

	if config.MaxSize != 0 && config.MaxSize <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxSize"), config.MaxSize, "maxSize must be positive when specified"))
	}

	return allErrs
}