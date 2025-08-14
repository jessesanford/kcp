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
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"

// UpstreamSyncConfig defines configuration for syncing resources from physical clusters to KCP
type UpstreamSyncConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpstreamSyncSpec   `json:"spec,omitempty"`
	Status UpstreamSyncStatus `json:"status,omitempty"`
}

// UpstreamSyncSpec defines the desired state of upstream synchronization
type UpstreamSyncSpec struct {
	// SyncTargets specifies which physical clusters to sync from
	SyncTargets []SyncTargetReference `json:"syncTargets"`

	// ResourceSelectors defines which resources to sync
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// SyncInterval defines how often to sync (default: 30s)
	// +kubebuilder:default="30s"
	SyncInterval metav1.Duration `json:"syncInterval,omitempty"`

	// ConflictStrategy defines how to handle conflicts between clusters
	// +kubebuilder:default=UseNewest
	// +kubebuilder:validation:Enum=UseNewest;UseOldest;Manual;Priority
	ConflictStrategy ConflictStrategy `json:"conflictStrategy,omitempty"`
}

// SyncTargetReference identifies a SyncTarget to monitor
type SyncTargetReference struct {
	// Name of the SyncTarget
	Name string `json:"name"`

	// Workspace containing the SyncTarget (optional, defaults to current)
	Workspace string `json:"workspace,omitempty"`
}

// ResourceSelector identifies resources to sync
type ResourceSelector struct {
	// APIGroup to sync (e.g., "apps")
	APIGroup string `json:"apiGroup"`

	// Resource type (e.g., "deployments")
	Resource string `json:"resource"`

	// Namespace to sync from (optional, empty means all)
	Namespace string `json:"namespace,omitempty"`

	// LabelSelector for filtering resources
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// ConflictStrategy defines how conflicts are resolved
type ConflictStrategy string

const (
	ConflictStrategyUseNewest ConflictStrategy = "UseNewest"
	ConflictStrategyUseOldest ConflictStrategy = "UseOldest"
	ConflictStrategyManual    ConflictStrategy = "Manual"
	ConflictStrategyPriority  ConflictStrategy = "Priority"
)

// UpstreamSyncStatus defines the observed state
type UpstreamSyncStatus struct {
	// ObservedGeneration tracks spec generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastSyncTime records when sync last occurred
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// SyncedResources count of resources synced
	SyncedResources int32 `json:"syncedResources,omitempty"`
}

// +kubebuilder:object:root=true

// UpstreamSyncConfigList contains a list of UpstreamSyncConfig
type UpstreamSyncConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UpstreamSyncConfig `json:"items"`
}