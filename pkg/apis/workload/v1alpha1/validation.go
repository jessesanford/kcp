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

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

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

	// Validate connection state if present
	if status.ConnectionState != "" {
		switch status.ConnectionState {
		case ConnectionStateConnected, ConnectionStateDisconnected, ConnectionStateConnecting, ConnectionStateError:
			// Valid connection states
		default:
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("connectionState"),
				status.ConnectionState, []string{string(ConnectionStateConnected), string(ConnectionStateDisconnected), 
					string(ConnectionStateConnecting), string(ConnectionStateError)}))
		}
	}

	// Validate sync state if present
	if status.SyncState != "" {
		switch status.SyncState {
		case SyncStateReady, SyncStateNotReady, SyncStateSyncing, SyncStateError:
			// Valid sync states
		default:
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("syncState"),
				status.SyncState, []string{string(SyncStateReady), string(SyncStateNotReady), 
					string(SyncStateSyncing), string(SyncStateError)}))
		}
	}

	// Validate synced resources
	for i, resource := range status.SyncedResources {
		resourcePath := fldPath.Child("syncedResources").Index(i)
		allErrs = append(allErrs, validateSyncedResourceStatus(&resource, resourcePath)...)
	}

	// Validate health status
	if status.Health != nil {
		allErrs = append(allErrs, validateHealthStatus(status.Health, fldPath.Child("health"))...)
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

// validateSyncedResourceStatus validates a synced resource status entry
func validateSyncedResourceStatus(resource *SyncedResourceStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate version (required)
	if resource.Version == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "version is required"))
	}

	// Validate kind (required)
	if resource.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "kind is required"))
	}

	// Validate name (required)
	if resource.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name is required"))
	}

	// Validate group if provided (should be valid DNS subdomain or empty)
	if resource.Group != "" {
		if errs := validation.IsDNS1123Subdomain(resource.Group); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("group"),
				resource.Group, fmt.Sprintf("group must be a valid DNS subdomain: %s", strings.Join(errs, ", "))))
		}
	}

	// Validate namespace if provided (should be valid DNS label)
	if resource.Namespace != "" {
		if errs := validation.IsDNS1123Label(resource.Namespace); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"),
				resource.Namespace, fmt.Sprintf("namespace must be a valid DNS label: %s", strings.Join(errs, ", "))))
		}
	}

	// Validate sync state if present
	if resource.SyncState != "" {
		switch resource.SyncState {
		case SyncStateReady, SyncStateNotReady, SyncStateSyncing, SyncStateError:
			// Valid sync states
		default:
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("syncState"),
				resource.SyncState, []string{string(SyncStateReady), string(SyncStateNotReady), 
					string(SyncStateSyncing), string(SyncStateError)}))
		}
	}

	return allErrs
}

// validateHealthStatus validates a health status entry
func validateHealthStatus(health *HealthStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate status (required)
	switch health.Status {
	case HealthStatusHealthy, HealthStatusDegraded, HealthStatusUnhealthy, HealthStatusUnknown:
		// Valid health statuses
	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("status"), "health status is required"))
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("status"),
			health.Status, []string{string(HealthStatusHealthy), string(HealthStatusDegraded), 
				string(HealthStatusUnhealthy), string(HealthStatusUnknown)}))
	}

	// Validate individual health checks
	checkNames := make(map[string]bool)
	for i, check := range health.Checks {
		checkPath := fldPath.Child("checks").Index(i)
		allErrs = append(allErrs, validateHealthCheck(&check, checkPath)...)

		// Check for duplicate health check names
		if checkNames[check.Name] {
			allErrs = append(allErrs, field.Duplicate(checkPath.Child("name"), check.Name))
		}
		checkNames[check.Name] = true
	}

	return allErrs
}

// validateHealthCheck validates an individual health check
func validateHealthCheck(check *HealthCheck, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate name (required)
	if check.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "health check name is required"))
	}

	// Validate status
	switch check.Status {
	case HealthCheckStatusPassed, HealthCheckStatusFailed, HealthCheckStatusUnknown:
		// Valid health check statuses
	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("status"), "health check status is required"))
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("status"),
			check.Status, []string{string(HealthCheckStatusPassed), string(HealthCheckStatusFailed), 
				string(HealthCheckStatusUnknown)}))
	}

	return allErrs
}