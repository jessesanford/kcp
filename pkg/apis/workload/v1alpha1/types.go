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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Location",type="string",JSONPath=`.spec.cells[0].name`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Syncer",type="string",JSONPath=`.status.syncerIdentity`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// SyncTarget defines a physical cluster target for workload synchronization.
// It represents a physical cluster that can host workloads in the TMC system,
// providing the foundation for multi-cluster workload placement and management.
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SyncTarget
	Spec SyncTargetSpec `json:"spec"`

	// Status defines the observed state of the SyncTarget
	// +optional
	Status SyncTargetStatus `json:"status,omitempty"`
}

// SyncTargetSpec defines the desired state of a SyncTarget
type SyncTargetSpec struct {
	// Cells defines the cells this SyncTarget supports. At least one cell is required.
	// Cells represent failure domains or locations within the target cluster.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Cells []Cell `json:"cells"`

	// SupportedAPIExports defines which APIs this target can sync.
	// This allows the target to advertise which APIs it supports for workload placement.
	// +optional
	SupportedAPIExports []APIExportReference `json:"supportedAPIExports,omitempty"`

	// Unschedulable marks this SyncTarget as unavailable for new workloads.
	// When true, new workloads will not be scheduled to this target, but existing
	// workloads will continue to run.
	// +optional
	Unschedulable bool `json:"unschedulable,omitempty"`

	// EvictAfter defines when to evict workloads after target becomes unhealthy.
	// This provides a grace period for the target to recover before workloads are moved.
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=duration
	EvictAfter *metav1.Duration `json:"evictAfter,omitempty"`
}

// Cell represents a failure domain or location within a SyncTarget.
// Cells provide a way to organize and constrain workload placement within
// a physical cluster based on topology or other characteristics.
type Cell struct {
	// Name is the unique identifier for this cell within the SyncTarget
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Labels provide additional metadata for the cell that can be used
	// for workload placement decisions and constraints
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Taints applied to this cell that affect workload placement.
	// Workloads must tolerate these taints to be scheduled to this cell.
	// +optional
	Taints []Taint `json:"taints,omitempty"`
}

// Taint represents a taint on a cell that affects workload placement
type Taint struct {
	// Key is the taint key to be applied to the cell
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`

	// Value is the taint value corresponding to the taint key
	// +optional
	Value string `json:"value,omitempty"`

	// Effect indicates the taint effect to apply to workloads that do not tolerate the taint.
	// Valid effects are NoSchedule, PreferNoSchedule and NoExecute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
	Effect TaintEffect `json:"effect"`
}

// TaintEffect defines the effect of a taint on workload placement
// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
type TaintEffect string

const (
	// TaintEffectNoSchedule means workloads will not be scheduled to the cell unless they tolerate the taint
	TaintEffectNoSchedule TaintEffect = "NoSchedule"
	// TaintEffectPreferNoSchedule means the scheduler will try to avoid scheduling workloads to the cell
	TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"
	// TaintEffectNoExecute means workloads will be evicted from the cell if they do not tolerate the taint
	TaintEffectNoExecute TaintEffect = "NoExecute"
)

// APIExportReference references an APIExport that this SyncTarget supports
type APIExportReference struct {
	// Workspace is the logical cluster workspace containing the APIExport
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Workspace string `json:"workspace"`

	// Name is the name of the APIExport within the workspace
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// SyncTargetStatus defines the observed state of a SyncTarget
type SyncTargetStatus struct {
	// Allocatable resources on this target available for workload placement
	// +optional
	Allocatable ResourceList `json:"allocatable,omitempty"`

	// Capacity defines the total resources available on this target
	// +optional
	Capacity ResourceList `json:"capacity,omitempty"`

	// SyncerIdentity identifies the syncer component managing this target.
	// This is used to track which syncer instance is responsible for synchronization.
	// +optional
	SyncerIdentity string `json:"syncerIdentity,omitempty"`

	// LastHeartbeatTime is when the syncer last sent a heartbeat for this target.
	// This is used to determine if the target is still healthy and reachable.
	// +optional
	LastHeartbeatTime *metav1.Time `json:"lastHeartbeatTime,omitempty"`

	// VirtualWorkspaces contains the virtual workspace URLs through which this target is exposed.
	// These URLs allow clients to interact with resources on this target.
	// +optional
	VirtualWorkspaces []VirtualWorkspace `json:"virtualWorkspaces,omitempty"`

	// Conditions represent the current status conditions of the SyncTarget.
	// Known condition types include Ready and Heartbeat.
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

// ResourceList is a map of resource name to quantity, representing available resources
type ResourceList map[string]resource.Quantity

// VirtualWorkspace represents a virtual workspace URL for accessing the SyncTarget
type VirtualWorkspace struct {
	// URL is the virtual workspace URL for accessing resources on this SyncTarget
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=uri
	URL string `json:"url"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SyncTargetList contains a list of SyncTargets
type SyncTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncTarget `json:"items"`
}
