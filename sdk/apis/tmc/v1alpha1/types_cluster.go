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

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// ClusterRegistration represents a cluster that has been registered with the TMC system.
// It contains cluster metadata, configuration, and status information needed for
// workload placement and cluster management decisions.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Location",type=string,JSONPath=`.spec.location`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ClusterRegistration struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec ClusterRegistrationSpec `json:"spec,omitempty"`

	// +optional
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of a registered cluster.
type ClusterRegistrationSpec struct {
	// Location specifies the geographic or logical location of the cluster.
	// This is used by location-aware placement policies.
	// +kubebuilder:validation:Required
	Location string `json:"location"`

	// Capabilities describes the capabilities and features available in this cluster.
	// This information is used for workload placement decisions.
	// +optional
	Capabilities ClusterCapabilities `json:"capabilities,omitempty"`

	// Labels are additional labels to be applied to workloads placed on this cluster.
	// These labels can be used for scheduling, monitoring, and policy enforcement.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Taints represent restrictions that prevent certain workloads from being placed
	// on this cluster unless they have matching tolerations.
	// +optional
	Taints []ClusterTaint `json:"taints,omitempty"`
}

// ClusterCapabilities describes the capabilities and resources available in a cluster.
type ClusterCapabilities struct {
	// Compute describes the computational resources available.
	// +optional
	Compute ComputeCapabilities `json:"compute,omitempty"`

	// Storage describes the storage capabilities available.
	// +optional
	Storage StorageCapabilities `json:"storage,omitempty"`

	// Network describes the networking capabilities available.
	// +optional
	Network NetworkCapabilities `json:"network,omitempty"`
}

// ComputeCapabilities describes the computational resources of a cluster.
type ComputeCapabilities struct {
	// Architecture specifies the CPU architecture (e.g., "amd64", "arm64").
	// +optional
	Architecture string `json:"architecture,omitempty"`

	// MaxCPU is the maximum CPU capacity available in the cluster.
	// +optional
	MaxCPU string `json:"maxCPU,omitempty"`

	// MaxMemory is the maximum memory capacity available in the cluster.
	// +optional
	MaxMemory string `json:"maxMemory,omitempty"`
}

// StorageCapabilities describes the storage resources of a cluster.
type StorageCapabilities struct {
	// StorageClasses lists the available storage classes.
	// +optional
	StorageClasses []string `json:"storageClasses,omitempty"`

	// MaxStorage is the maximum storage capacity available.
	// +optional
	MaxStorage string `json:"maxStorage,omitempty"`
}

// NetworkCapabilities describes the networking features of a cluster.
type NetworkCapabilities struct {
	// LoadBalancerSupport indicates if the cluster supports LoadBalancer services.
	// +optional
	LoadBalancerSupport bool `json:"loadBalancerSupport,omitempty"`

	// IngressSupport indicates if the cluster supports Ingress resources.
	// +optional
	IngressSupport bool `json:"ingressSupport,omitempty"`
}

// ClusterTaint represents a restriction on workload placement.
type ClusterTaint struct {
	// Key is the taint key to be applied to the cluster.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Value is the taint value corresponding to the taint key.
	// +optional
	Value string `json:"value,omitempty"`

	// Effect indicates the effect of the taint on workloads that do not tolerate it.
	// Valid effects are NoSchedule, PreferNoSchedule, and NoExecute.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
	Effect TaintEffect `json:"effect"`
}

// TaintEffect is the effect of a cluster taint.
type TaintEffect string

const (
	// TaintEffectNoSchedule means workloads will not be scheduled onto the cluster unless they tolerate the taint.
	TaintEffectNoSchedule TaintEffect = "NoSchedule"
	// TaintEffectPreferNoSchedule means the scheduler will try to avoid placing workloads that don't tolerate the taint.
	TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"
	// TaintEffectNoExecute means workloads will be evicted from the cluster if they don't tolerate the taint.
	TaintEffectNoExecute TaintEffect = "NoExecute"
)

// ClusterRegistrationStatus represents the observed state of a ClusterRegistration.
type ClusterRegistrationStatus struct {
	// Conditions represent the latest available observations of the cluster's state.
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// LastHeartbeatTime is the last time the cluster sent a heartbeat.
	// +optional
	LastHeartbeatTime *metav1.Time `json:"lastHeartbeatTime,omitempty"`

	// WorkloadCount is the current number of workloads running on this cluster.
	// +optional
	WorkloadCount int32 `json:"workloadCount,omitempty"`

	// ResourceUsage provides information about current resource utilization.
	// +optional
	ResourceUsage ClusterResourceUsage `json:"resourceUsage,omitempty"`
}

// ClusterResourceUsage describes the current resource utilization of a cluster.
type ClusterResourceUsage struct {
	// CPU usage as a percentage of total capacity.
	// +optional
	CPU string `json:"cpu,omitempty"`

	// Memory usage as a percentage of total capacity.
	// +optional
	Memory string `json:"memory,omitempty"`

	// Storage usage as a percentage of total capacity.
	// +optional
	Storage string `json:"storage,omitempty"`
}

// ClusterRegistrationList contains a list of ClusterRegistration resources.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterRegistration `json:"items"`
}
