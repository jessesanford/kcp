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

import "k8s.io/apimachinery/pkg/api/resource"

// SetDefaults_SyncTarget sets default values for SyncTarget fields.
// This function is called by the Kubernetes API server during admission
// to ensure consistent default values across all SyncTarget resources.
//
// +kubebuilder:webhook:path=/mutate-workload-kcp-io-v1alpha1-synctarget,mutating=true,failurePolicy=fail,sideEffects=None,groups=workload.kcp.io,resources=synctargets,verbs=create;update,versions=v1alpha1,name=msynctarget.kb.io,admissionReviewVersions=v1
func SetDefaults_SyncTarget(obj *SyncTarget) {
	// Ensure SyncerConfig is initialized with defaults
	if obj.Spec.SyncerConfig == nil {
		obj.Spec.SyncerConfig = &SyncerConfig{}
	}

	// Set default SyncerConfig values
	SetDefaults_SyncerConfig(obj.Spec.SyncerConfig)
}

// SetDefaults_SyncerConfig sets default values for SyncerConfig fields.
func SetDefaults_SyncerConfig(config *SyncerConfig) {
	// Set default sync mode
	if config.SyncMode == "" {
		config.SyncMode = "push"
	}

	// Set default sync interval  
	if config.SyncInterval == "" {
		config.SyncInterval = "30s"
	}

	// Ensure RetryBackoff is initialized with defaults
	if config.RetryBackoff == nil {
		config.RetryBackoff = &RetryBackoffConfig{}
	}

	// Set default RetryBackoff values
	SetDefaults_RetryBackoffConfig(config.RetryBackoff)
}

// SetDefaults_RetryBackoffConfig sets default values for RetryBackoffConfig fields.
func SetDefaults_RetryBackoffConfig(backoff *RetryBackoffConfig) {
	// Set default initial interval
	if backoff.InitialInterval == "" {
		backoff.InitialInterval = "1s"
	}

	// Set default maximum interval
	if backoff.MaxInterval == "" {
		backoff.MaxInterval = "5m"
	}

	// Set default multiplier
	if backoff.Multiplier == 0 {
		backoff.Multiplier = 2.0
	}
}

// SetDefaults_ResourceQuotas sets default values for ResourceQuotas fields.
// Currently, ResourceQuotas are optional and no defaults are applied.
// This function exists for future extensibility.
func SetDefaults_ResourceQuotas(quotas *ResourceQuotas) {
	// No defaults currently applied for ResourceQuotas
	// They remain nil/empty if not specified, indicating unlimited capacity
}

// GetDefaultResourceQuotas returns a ResourceQuotas struct with sensible defaults
// for common cluster configurations. This can be used by controllers or
// external systems that need to provide default quota values.
func GetDefaultResourceQuotas() *ResourceQuotas {
	return &ResourceQuotas{
		CPU:     resource.NewQuantity(1000, resource.DecimalSI), // 1000 CPU cores
		Memory:  resource.NewQuantity(1000*1024*1024*1024, resource.BinarySI), // 1000 GB
		Storage: resource.NewQuantity(10000*1024*1024*1024, resource.BinarySI), // 10 TB
		Pods:    resource.NewQuantity(1000, resource.DecimalSI), // 1000 pods
		Custom:  make(map[string]resource.Quantity),
	}
}

// ApplyDefaults applies default values to all nested structures within a SyncTarget.
// This is a comprehensive defaulting function that ensures all optional fields
// have appropriate default values.
func (s *SyncTarget) ApplyDefaults() {
	SetDefaults_SyncTarget(s)
}

// ApplyResourceQuotaDefaults applies default resource quotas if none are specified.
// This is useful for controllers that want to ensure some capacity limits exist.
func (s *SyncTarget) ApplyResourceQuotaDefaults() {
	if s.Spec.ResourceQuotas == nil {
		s.Spec.ResourceQuotas = GetDefaultResourceQuotas()
	} else {
		// Apply individual defaults where fields are nil
		if s.Spec.ResourceQuotas.Custom == nil {
			s.Spec.ResourceQuotas.Custom = make(map[string]resource.Quantity)
		}
	}
}