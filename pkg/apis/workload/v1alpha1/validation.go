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

	// Validate connection if specified
	if spec.Connection != nil {
		allErrs = append(allErrs, validateSyncTargetConnection(spec.Connection, fldPath.Child("connection"))...)
	}

	// Validate credentials if specified
	if spec.Credentials != nil {
		allErrs = append(allErrs, validateSyncTargetCredentials(spec.Credentials, fldPath.Child("credentials"))...)
	}

	// Validate capabilities if specified
	if spec.Capabilities != nil {
		allErrs = append(allErrs, validateSyncTargetCapabilities(spec.Capabilities, fldPath.Child("capabilities"))...)
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

// validateSyncTargetConnection validates a SyncTarget connection
func validateSyncTargetConnection(conn *SyncTargetConnection, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate URL
	if conn.URL == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("url"), "connection URL is required"))
	} else {
		parsedURL, err := url.Parse(conn.URL)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("url"),
				conn.URL, fmt.Sprintf("invalid URL format: %v", err)))
		} else if parsedURL.Scheme == "" || parsedURL.Host == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("url"),
				conn.URL, "URL must have scheme and host"))
		}
	}

	// Validate server name if provided (should be valid DNS name)
	if conn.ServerName != "" {
		if errs := validation.IsDNS1123Subdomain(conn.ServerName); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("serverName"),
				conn.ServerName, fmt.Sprintf("server name must be a valid DNS name: %s", strings.Join(errs, ", "))))
		}
	}

	return allErrs
}

// validateSyncTargetCredentials validates SyncTarget credentials
func validateSyncTargetCredentials(creds *SyncTargetCredentials, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate auth type
	if creds.Type == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "authentication type is required"))
	} else {
		switch creds.Type {
		case SyncTargetAuthTypeToken:
			if creds.Token == nil {
				allErrs = append(allErrs, field.Required(fldPath.Child("token"), "token credentials are required for token auth"))
			} else {
				allErrs = append(allErrs, validateTokenCredentials(creds.Token, fldPath.Child("token"))...)
			}
		case SyncTargetAuthTypeCertificate:
			if creds.Certificate == nil {
				allErrs = append(allErrs, field.Required(fldPath.Child("certificate"), "certificate credentials are required for certificate auth"))
			} else {
				allErrs = append(allErrs, validateCertificateCredentials(creds.Certificate, fldPath.Child("certificate"))...)
			}
		case SyncTargetAuthTypeServiceAccount:
			if creds.ServiceAccount == nil {
				allErrs = append(allErrs, field.Required(fldPath.Child("serviceAccount"), "service account credentials are required for service account auth"))
			} else {
				allErrs = append(allErrs, validateServiceAccountCredentials(creds.ServiceAccount, fldPath.Child("serviceAccount"))...)
			}
		default:
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"),
				creds.Type, []string{string(SyncTargetAuthTypeToken), string(SyncTargetAuthTypeCertificate), string(SyncTargetAuthTypeServiceAccount)}))
		}
	}

	return allErrs
}

// validateTokenCredentials validates token-based credentials
func validateTokenCredentials(token *TokenCredentials, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if token.Value == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("value"), "token value is required"))
	}

	return allErrs
}

// validateCertificateCredentials validates certificate-based credentials
func validateCertificateCredentials(cert *CertificateCredentials, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(cert.ClientCert) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("clientCert"), "client certificate is required"))
	}

	if len(cert.ClientKey) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("clientKey"), "client key is required"))
	}

	return allErrs
}

// validateServiceAccountCredentials validates service account-based credentials
func validateServiceAccountCredentials(sa *ServiceAccountCredentials, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if sa.Namespace == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "service account namespace is required"))
	} else {
		if errs := validation.IsDNS1123Label(sa.Namespace); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"),
				sa.Namespace, fmt.Sprintf("namespace must be a valid DNS label: %s", strings.Join(errs, ", "))))
		}
	}

	if sa.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "service account name is required"))
	} else {
		if errs := validation.IsDNS1123Label(sa.Name); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"),
				sa.Name, fmt.Sprintf("name must be a valid DNS label: %s", strings.Join(errs, ", "))))
		}
	}

	return allErrs
}

// validateSyncTargetCapabilities validates SyncTarget capabilities
func validateSyncTargetCapabilities(caps *SyncTargetCapabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate max workloads if specified
	if caps.MaxWorkloads != nil && *caps.MaxWorkloads < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxWorkloads"),
			*caps.MaxWorkloads, "maxWorkloads must be non-negative"))
	}

	// Validate supported resource types
	for i, resourceType := range caps.SupportedResourceTypes {
		resourcePath := fldPath.Child("supportedResourceTypes").Index(i)
		allErrs = append(allErrs, validateResourceTypeSupport(&resourceType, resourcePath)...)
	}

	// Validate features (should be non-empty strings)
	for i, feature := range caps.Features {
		if feature == "" {
			allErrs = append(allErrs, field.Required(fldPath.Child("features").Index(i),
				"feature name cannot be empty"))
		}
	}

	return allErrs
}

// validateResourceTypeSupport validates a resource type support entry
func validateResourceTypeSupport(rts *ResourceTypeSupport, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if rts.Version == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "version is required"))
	}

	if rts.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "kind is required"))
	}

	// Validate group if provided (should be valid DNS subdomain or empty)
	if rts.Group != "" {
		if errs := validation.IsDNS1123Subdomain(rts.Group); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("group"),
				rts.Group, fmt.Sprintf("group must be a valid DNS subdomain: %s", strings.Join(errs, ", "))))
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
