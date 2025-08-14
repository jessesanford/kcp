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

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateUpstreamSyncConfig validates an UpstreamSyncConfig
func ValidateUpstreamSyncConfig(config *UpstreamSyncConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	specPath := field.NewPath("spec")

	// Validate SyncTargets
	if len(config.Spec.SyncTargets) == 0 {
		allErrs = append(allErrs, field.Required(specPath.Child("syncTargets"),
			"at least one sync target must be specified"))
	}

	// Validate ResourceSelectors
	if len(config.Spec.ResourceSelectors) == 0 {
		allErrs = append(allErrs, field.Required(specPath.Child("resourceSelectors"),
			"at least one resource selector must be specified"))
	}

	// Validate SyncInterval
	if config.Spec.SyncInterval.Duration < 10*time.Second {
		allErrs = append(allErrs, field.Invalid(specPath.Child("syncInterval"),
			config.Spec.SyncInterval, "sync interval must be at least 10s"))
	}

	// Validate ConflictStrategy
	validStrategies := map[ConflictStrategy]bool{
		ConflictStrategyUseNewest: true,
		ConflictStrategyUseOldest: true,
		ConflictStrategyManual:    true,
		ConflictStrategyPriority:  true,
	}

	if !validStrategies[config.Spec.ConflictStrategy] {
		allErrs = append(allErrs, field.Invalid(specPath.Child("conflictStrategy"),
			config.Spec.ConflictStrategy,
			fmt.Sprintf("must be one of: UseNewest, UseOldest, Manual, Priority")))
	}

	return allErrs
}