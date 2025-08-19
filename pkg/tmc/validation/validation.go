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

package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kcp-dev/logicalcluster/v3"
)

// TMC validation constants
const (
	// MaxLocationLength is the maximum length for location strings
	MaxLocationLength = 253
	
	// MaxClusterNameLength is the maximum length for cluster names
	MaxClusterNameLength = 253
	
	// MaxLabelValueLength is the maximum length for label values
	MaxLabelValueLength = 63
	
	// MaxAnnotationValueLength is the maximum length for annotation values
	MaxAnnotationValueLength = 262144
)

// Regular expressions for validation
var (
	// ValidLocationRegex matches valid location identifiers
	ValidLocationRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	
	// ValidClusterNameRegex matches valid cluster names
	ValidClusterNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	
	// ValidSyncerNameRegex matches valid syncer names
	ValidSyncerNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
)

// ValidateWorkspace validates a logical cluster workspace name.
func ValidateWorkspace(workspace logicalcluster.Name, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	if workspace == "" {
		allErrs = append(allErrs, field.Required(fldPath, "workspace is required"))
		return allErrs
	}
	
	workspaceStr := string(workspace)
	if len(workspaceStr) > MaxClusterNameLength {
		allErrs = append(allErrs, field.TooLong(fldPath, workspaceStr, MaxClusterNameLength))
	}
	
	// Validate workspace format using KCP's validation
	if errs := validation.IsDNS1123Subdomain(workspaceStr); len(errs) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, workspaceStr, fmt.Sprintf("invalid workspace name: %s", strings.Join(errs, ", "))))
	}
	
	return allErrs
}

// ValidateLocation validates a location string for TMC.
func ValidateLocation(location string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	if location == "" {
		allErrs = append(allErrs, field.Required(fldPath, "location is required"))
		return allErrs
	}
	
	if len(location) > MaxLocationLength {
		allErrs = append(allErrs, field.TooLong(fldPath, location, MaxLocationLength))
	}
	
	if !ValidLocationRegex.MatchString(location) {
		allErrs = append(allErrs, field.Invalid(fldPath, location, "location must be a valid DNS-like identifier"))
	}
	
	return allErrs
}

// ValidateClusterName validates a cluster name for TMC.
func ValidateClusterName(clusterName string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	if clusterName == "" {
		allErrs = append(allErrs, field.Required(fldPath, "cluster name is required"))
		return allErrs
	}
	
	if len(clusterName) > MaxClusterNameLength {
		allErrs = append(allErrs, field.TooLong(fldPath, clusterName, MaxClusterNameLength))
	}
	
	if !ValidClusterNameRegex.MatchString(clusterName) {
		allErrs = append(allErrs, field.Invalid(fldPath, clusterName, "cluster name must be a valid DNS-1123 subdomain"))
	}
	
	return allErrs
}

// ValidateSyncerName validates a syncer name for TMC.
func ValidateSyncerName(syncerName string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	if syncerName == "" {
		allErrs = append(allErrs, field.Required(fldPath, "syncer name is required"))
		return allErrs
	}
	
	if len(syncerName) > MaxClusterNameLength {
		allErrs = append(allErrs, field.TooLong(fldPath, syncerName, MaxClusterNameLength))
	}
	
	if !ValidSyncerNameRegex.MatchString(syncerName) {
		allErrs = append(allErrs, field.Invalid(fldPath, syncerName, "syncer name must be a valid DNS-1123 subdomain"))
	}
	
	return allErrs
}

// ValidateURL validates a URL string.
func ValidateURL(urlStr string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	if urlStr == "" {
		allErrs = append(allErrs, field.Required(fldPath, "URL is required"))
		return allErrs
	}
	
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, urlStr, fmt.Sprintf("invalid URL: %v", err)))
		return allErrs
	}
	
	if parsedURL.Scheme == "" {
		allErrs = append(allErrs, field.Invalid(fldPath, urlStr, "URL scheme is required"))
	}
	
	if parsedURL.Host == "" {
		allErrs = append(allErrs, field.Invalid(fldPath, urlStr, "URL host is required"))
	}
	
	return allErrs
}

// ValidateLabels validates Kubernetes labels.
func ValidateLabels(labels map[string]string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	for key, value := range labels {
		keyPath := fldPath.Key(key)
		
		// Validate label key
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(keyPath, key, fmt.Sprintf("invalid label key: %s", strings.Join(errs, ", "))))
		}
		
		// Validate label value
		if len(value) > MaxLabelValueLength {
			allErrs = append(allErrs, field.TooLong(keyPath, value, MaxLabelValueLength))
		}
		
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(keyPath, value, fmt.Sprintf("invalid label value: %s", strings.Join(errs, ", "))))
		}
	}
	
	return allErrs
}

// ValidateAnnotations validates Kubernetes annotations.
func ValidateAnnotations(annotations map[string]string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	for key, value := range annotations {
		keyPath := fldPath.Key(key)
		
		// Validate annotation key
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(keyPath, key, fmt.Sprintf("invalid annotation key: %s", strings.Join(errs, ", "))))
		}
		
		// Validate annotation value length
		if len(value) > MaxAnnotationValueLength {
			allErrs = append(allErrs, field.TooLong(keyPath, value, MaxAnnotationValueLength))
		}
	}
	
	return allErrs
}

// ValidateResourceSelector validates a TMC resource selector.
func ValidateResourceSelector(selector map[string]string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	if len(selector) == 0 {
		return allErrs
	}
	
	// Validate selector as labels (same format)
	allErrs = append(allErrs, ValidateLabels(selector, fldPath)...)
	
	return allErrs
}

// ValidatePlacementConstraints validates TMC placement constraints.
func ValidatePlacementConstraints(constraints map[string]string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	// Known constraint keys
	knownConstraints := map[string]bool{
		"location":         true,
		"cluster":          true,
		"availability-zone": true,
		"node-selector":    true,
		"resource-requirements": true,
	}
	
	for key, value := range constraints {
		keyPath := fldPath.Key(key)
		
		// Check if constraint key is known (warn but don't fail)
		if !knownConstraints[key] {
			// Log warning about unknown constraint but don't fail validation
			continue
		}
		
		// Validate constraint value is not empty
		if value == "" {
			allErrs = append(allErrs, field.Required(keyPath, "constraint value cannot be empty"))
		}
		
		// Validate specific constraint formats
		switch key {
		case "location":
			allErrs = append(allErrs, ValidateLocation(value, keyPath)...)
		case "cluster":
			allErrs = append(allErrs, ValidateClusterName(value, keyPath)...)
		}
	}
	
	return allErrs
}

// ValidateTMCResourceName validates a TMC resource name.
func ValidateTMCResourceName(name string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	if name == "" {
		allErrs = append(allErrs, field.Required(fldPath, "name is required"))
		return allErrs
	}
	
	// Use Kubernetes DNS-1123 subdomain validation for resource names
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, name, fmt.Sprintf("invalid resource name: %s", strings.Join(errs, ", "))))
	}
	
	return allErrs
}

// ValidateNamespace validates a Kubernetes namespace name.
func ValidateNamespace(namespace string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	
	if namespace == "" {
		// Namespace can be empty for cluster-scoped resources
		return allErrs
	}
	
	if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, namespace, fmt.Sprintf("invalid namespace: %s", strings.Join(errs, ", "))))
	}
	
	return allErrs
}