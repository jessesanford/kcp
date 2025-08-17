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

	"github.com/kcp-dev/logicalcluster/v3"
)

// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp

// SyncTarget represents a target cluster for workload scheduling in TMC.
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SyncTarget.
	Spec SyncTargetSpec `json:"spec,omitempty"`

	// Status defines the observed state of the SyncTarget.
	Status SyncTargetStatus `json:"status,omitempty"`
}

// SyncTargetSpec defines the desired configuration for a SyncTarget.
type SyncTargetSpec struct {
	// APIServerURL is the URL of the target cluster's API server.
	APIServerURL string `json:"apiServerURL,omitempty"`

	// Unschedulable marks the SyncTarget as unschedulable for new workloads.
	Unschedulable bool `json:"unschedulable,omitempty"`
}

// SyncTargetStatus defines the observed state of the SyncTarget.
type SyncTargetStatus struct {
	// LastHeartbeat is the time when the last heartbeat was received.
	LastHeartbeat metav1.Time `json:"lastHeartbeat,omitempty"`

	// LastReconcileTime is the time when this resource was last reconciled.
	LastReconcileTime metav1.Time `json:"lastReconcileTime,omitempty"`

	// Capacity represents the total resource capacity of the target cluster.
	Capacity corev1.ResourceList `json:"capacity,omitempty"`

	// Allocatable represents the allocatable resource capacity.
	Allocatable corev1.ResourceList `json:"allocatable,omitempty"`

	// Available represents the currently available resource capacity.
	Available corev1.ResourceList `json:"available,omitempty"`

	// VirtualWorkspaces represents the associated virtual workspaces.
	VirtualWorkspaces []VirtualWorkspaceReference `json:"virtualWorkspaces,omitempty"`

	// Conditions represent the current service state of the SyncTarget.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// VirtualWorkspaceReference represents a reference to an associated VirtualWorkspace.
type VirtualWorkspaceReference struct {
	// Name is the name of the virtual workspace.
	Name string `json:"name"`

	// URL is the URL of the virtual workspace.
	URL string `json:"url,omitempty"`
}

// SyncTargetList contains a list of SyncTarget resources.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SyncTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncTarget `json:"items"`
}

// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp

// VirtualWorkspace represents a virtual workspace for workload placement.
type VirtualWorkspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the VirtualWorkspace.
	Spec VirtualWorkspaceSpec `json:"spec,omitempty"`

	// Status defines the observed state of the VirtualWorkspace.
	Status VirtualWorkspaceStatus `json:"status,omitempty"`
}

// VirtualWorkspaceSpec defines the desired configuration for a VirtualWorkspace.
type VirtualWorkspaceSpec struct {
	// URL is the virtual workspace URL endpoint.
	URL string `json:"url,omitempty"`
}

// VirtualWorkspaceStatus defines the observed state of the VirtualWorkspace.
type VirtualWorkspaceStatus struct {
	// URL is the resolved virtual workspace URL.
	URL string `json:"url,omitempty"`

	// Conditions represent the current state of the VirtualWorkspace.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// VirtualWorkspaceList contains a list of VirtualWorkspace resources.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VirtualWorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualWorkspace `json:"items"`
}

// ClusterName returns the logical cluster name for the SyncTarget.
func (s *SyncTarget) ClusterName() logicalcluster.Name {
	return logicalcluster.From(s)
}