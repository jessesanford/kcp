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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

// SetDefaults_UpstreamSyncConfig sets defaults for UpstreamSyncConfig
func SetDefaults_UpstreamSyncConfig(obj *UpstreamSyncConfig) {
	if obj.Spec.SyncInterval.Duration == 0 {
		obj.Spec.SyncInterval = metav1.Duration{Duration: 30 * time.Second}
	}

	if obj.Spec.ConflictStrategy == "" {
		obj.Spec.ConflictStrategy = ConflictStrategyUseNewest
	}

	// Ensure status conditions are initialized
	if obj.Status.Conditions == nil {
		obj.Status.Conditions = []metav1.Condition{}
	}
}