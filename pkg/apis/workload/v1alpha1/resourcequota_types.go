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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=quota
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="CPU Request",type="string",JSONPath=".status.used['requests.cpu']",description="Used CPU requests"
// +kubebuilder:printcolumn:name="CPU Limit",type="string",JSONPath=".spec.hard['requests.cpu']",description="CPU request limit"
// +kubebuilder:printcolumn:name="Memory Request",type="string",JSONPath=".status.used['requests.memory']",description="Used memory requests"
// +kubebuilder:printcolumn:name="Memory Limit",type="string",JSONPath=".spec.hard['requests.memory']",description="Memory request limit"

// ResourceQuota sets resource usage limits for compute resources in a logical cluster workspace.
// It tracks CPU, memory, storage, and object count quotas across namespaces within a workspace.
type ResourceQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired quota limits for compute resources.
	// +optional
	Spec ResourceQuotaSpec `json:"spec,omitempty"`

	// Status represents the current observed state of quota usage.
	// +optional
	Status ResourceQuotaStatus `json:"status,omitempty"`
}

// ResourceQuotaSpec defines the resource limits and scope for quota enforcement.
type ResourceQuotaSpec struct {
	// Hard is the set of desired resource limits.
	// Supported resources: requests.cpu, requests.memory, limits.cpu, limits.memory,
	// requests.storage, persistentvolumeclaims, pods, services, secrets, configmaps
	// +optional
	Hard corev1.ResourceList `json:"hard,omitempty"`

	// ScopeSelector restricts quota to specific resources based on runtime attributes.
	// This feature is deferred to a future implementation.
	// TODO: Implement scope selectors in follow-up PR for advanced quota filtering
	// +optional
	ScopeSelector *ScopeSelector `json:"scopeSelector,omitempty"`

	// Scopes restricts quota to specific resource lifecycles.
	// Supported scopes: Terminating, NotTerminating, BestEffort, NotBestEffort, PriorityClass
	// This feature is deferred to a future implementation.
	// TODO: Implement quota scopes in follow-up PR for pod lifecycle filtering
	// +optional
	Scopes []ResourceQuotaScope `json:"scopes,omitempty"`
}

// ResourceQuotaStatus represents the current observed state of resource quota usage.
type ResourceQuotaStatus struct {
	// Hard is the set of enforced resource limits.
	// +optional
	Hard corev1.ResourceList `json:"hard,omitempty"`

	// Used is the current observed total usage of the resource in the namespace.
	// +optional
	Used corev1.ResourceList `json:"used,omitempty"`

	// LastUpdated represents the time when the quota usage was last calculated.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Violations contains descriptions of any quota violations.
	// This field is set when resources exceed their hard limits.
	// +optional
	Violations []string `json:"violations,omitempty"`
}

// ScopeSelector represents a collection of scope requirements for quota resources.
// This is a placeholder for future advanced scope selection functionality.
type ScopeSelector struct {
	// MatchExpressions is a list of scope selector requirements by scope of the resources.
	// TODO: Implement in follow-up PR for advanced resource filtering
	// +optional
	MatchExpressions []ScopeSelectorRequirement `json:"matchExpressions,omitempty"`
}

// ScopeSelectorRequirement represents a requirement for scope selection.
type ScopeSelectorRequirement struct {
	// ScopeName is the name of the scope that the requirement applies to.
	// +required
	ScopeName ResourceQuotaScope `json:"scopeName"`

	// Operator represents the relationship between the scope name and values.
	// +required
	Operator ScopeSelectorOperator `json:"operator"`

	// Values is an array of string values for the requirement.
	// +optional
	Values []string `json:"values,omitempty"`
}

// ResourceQuotaScope defines the lifecycle stage of pods that quota should apply to.
type ResourceQuotaScope string

const (
	// ResourceQuotaScopeTerminating matches pods that have a non-zero deletion timestamp.
	ResourceQuotaScopeTerminating ResourceQuotaScope = "Terminating"
	// ResourceQuotaScopeNotTerminating matches pods without a deletion timestamp.
	ResourceQuotaScopeNotTerminating ResourceQuotaScope = "NotTerminating"
	// ResourceQuotaScopeBestEffort matches pods with best effort quality of service.
	ResourceQuotaScopeBestEffort ResourceQuotaScope = "BestEffort"
	// ResourceQuotaScopeNotBestEffort matches pods without best effort quality of service.
	ResourceQuotaScopeNotBestEffort ResourceQuotaScope = "NotBestEffort"
	// ResourceQuotaScopePriorityClass matches pods that reference a priority class.
	ResourceQuotaScopePriorityClass ResourceQuotaScope = "PriorityClass"
)

// ScopeSelectorOperator represents the relationship between scope names and values.
type ScopeSelectorOperator string

const (
	// ScopeSelectorOpIn means the scope name exists in the set of values.
	ScopeSelectorOpIn ScopeSelectorOperator = "In"
	// ScopeSelectorOpNotIn means the scope name does not exist in the set of values.
	ScopeSelectorOpNotIn ScopeSelectorOperator = "NotIn"
	// ScopeSelectorOpExists means the scope name exists.
	ScopeSelectorOpExists ScopeSelectorOperator = "Exists"
	// ScopeSelectorOpDoesNotExist means the scope name does not exist.
	ScopeSelectorOpDoesNotExist ScopeSelectorOperator = "DoesNotExist"
)

// +kubebuilder:object:root=true

// ResourceQuotaList contains a list of ResourceQuota objects.
type ResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceQuota `json:"items"`
}