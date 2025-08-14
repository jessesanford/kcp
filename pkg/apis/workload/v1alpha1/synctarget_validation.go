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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateCreate validates a SyncTarget on creation.
// It ensures all required fields are present and valid.
func (st *SyncTarget) ValidateCreate() error {
	allErrs := field.ErrorList{}

	// Validate spec
	if errs := st.validateSyncTargetSpec(); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) > 0 {
		return allErrs.ToAggregate()
	}
	return nil
}

// ValidateUpdate validates a SyncTarget on update.
// It ensures that immutable fields are not changed and validates updated fields.
func (st *SyncTarget) ValidateUpdate(old runtime.Object) error {
	allErrs := field.ErrorList{}
	oldSyncTarget, ok := old.(*SyncTarget)
	if !ok {
		return fmt.Errorf("expected SyncTarget, got %T", old)
	}

	// Validate that immutable fields haven't changed
	if errs := st.validateImmutableFields(oldSyncTarget); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	// Validate the current spec
	if errs := st.validateSyncTargetSpec(); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) > 0 {
		return allErrs.ToAggregate()
	}
	return nil
}

// validateSyncTargetSpec validates the SyncTarget specification.
func (st *SyncTarget) validateSyncTargetSpec() field.ErrorList {
	allErrs := field.ErrorList{}
	specPath := field.NewPath("spec")

	// Validate cluster reference
	if errs := st.validateClusterReference(specPath.Child("clusterRef")); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	// Validate syncer config if present
	if st.Spec.SyncerConfig != nil {
		if errs := st.validateSyncerConfig(specPath.Child("syncerConfig")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	// Validate resource quotas if present
	if st.Spec.ResourceQuotas != nil {
		if errs := st.validateResourceQuotas(specPath.Child("resourceQuotas")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	// Validate workload selector if present
	if st.Spec.Selector != nil {
		if errs := st.validateWorkloadSelector(specPath.Child("selector")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	return allErrs
}

// validateClusterReference validates the cluster reference.
func (st *SyncTarget) validateClusterReference(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if st.Spec.ClusterRef.Name == "" {
		allErrs = append(allErrs, field.Required(
			fldPath.Child("name"),
			"cluster reference name is required"))
	}

	return allErrs
}

// validateSyncerConfig validates the syncer configuration.
func (st *SyncTarget) validateSyncerConfig(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	config := st.Spec.SyncerConfig

	// Validate sync mode
	if config.SyncMode != "" {
		validModes := map[string]bool{"push": true, "pull": true, "bidirectional": true}
		if !validModes[config.SyncMode] {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("syncMode"),
				config.SyncMode,
				"must be one of: push, pull, bidirectional"))
		}
	}

	// Validate sync interval
	if config.SyncInterval != "" {
		if _, err := time.ParseDuration(config.SyncInterval); err != nil {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("syncInterval"),
				config.SyncInterval,
				"must be a valid duration string"))
		}
	}

	// Validate retry backoff if present
	if config.RetryBackoff != nil {
		if errs := st.validateRetryBackoff(fldPath.Child("retryBackoff")); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	return allErrs
}

// validateRetryBackoff validates retry backoff configuration.
func (st *SyncTarget) validateRetryBackoff(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	backoff := st.Spec.SyncerConfig.RetryBackoff

	// Validate initial interval
	if backoff.InitialInterval != "" {
		if _, err := time.ParseDuration(backoff.InitialInterval); err != nil {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("initialInterval"),
				backoff.InitialInterval,
				"must be a valid duration string"))
		}
	}

	// Validate max interval
	if backoff.MaxInterval != "" {
		if _, err := time.ParseDuration(backoff.MaxInterval); err != nil {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("maxInterval"),
				backoff.MaxInterval,
				"must be a valid duration string"))
		}
	}

	// Validate multiplier
	if backoff.Multiplier != 0 && backoff.Multiplier < 1.0 {
		allErrs = append(allErrs, field.Invalid(
			fldPath.Child("multiplier"),
			backoff.Multiplier,
			"must be >= 1.0"))
	}

	return allErrs
}

// validateResourceQuotas validates resource quota specifications.
func (st *SyncTarget) validateResourceQuotas(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	quotas := st.Spec.ResourceQuotas

	// Validate CPU quota if present
	if quotas.CPU != nil {
		if quotas.CPU.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("cpu"),
				quotas.CPU,
				"must be non-negative"))
		}
	}

	// Validate memory quota if present
	if quotas.Memory != nil {
		if quotas.Memory.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("memory"),
				quotas.Memory,
				"must be non-negative"))
		}
	}

	// Validate storage quota if present
	if quotas.Storage != nil {
		if quotas.Storage.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("storage"),
				quotas.Storage,
				"must be non-negative"))
		}
	}

	// Validate pods quota if present
	if quotas.Pods != nil {
		if quotas.Pods.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("pods"),
				quotas.Pods,
				"must be non-negative"))
		}
	}

	// Validate custom quotas
	for key, value := range quotas.Custom {
		if value.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("custom").Key(key),
				value,
				"must be non-negative"))
		}
	}

	return allErrs
}

// validateWorkloadSelector validates workload selector configuration.
func (st *SyncTarget) validateWorkloadSelector(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	selector := st.Spec.Selector

	// Validate that at least one selection criteria is specified
	if len(selector.MatchLabels) == 0 &&
		len(selector.MatchExpressions) == 0 &&
		selector.NamespaceSelector == nil &&
		len(selector.Locations) == 0 {
		allErrs = append(allErrs, field.Invalid(
			fldPath,
			selector,
			"at least one selection criteria must be specified"))
	}

	// Validate locations
	for i, location := range selector.Locations {
		if location == "" {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("locations").Index(i),
				location,
				"location name cannot be empty"))
		}
	}

	return allErrs
}

// validateImmutableFields ensures that immutable fields haven't changed.
func (st *SyncTarget) validateImmutableFields(old *SyncTarget) field.ErrorList {
	allErrs := field.ErrorList{}

	// ClusterRef is immutable after creation
	if st.Spec.ClusterRef.Name != old.Spec.ClusterRef.Name {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "clusterRef", "name"),
			"cluster reference name is immutable"))
	}

	if st.Spec.ClusterRef.Workspace != old.Spec.ClusterRef.Workspace {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "clusterRef", "workspace"),
			"cluster reference workspace is immutable"))
	}

	return allErrs
}