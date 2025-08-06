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

// WorkloadSelector defines how to select workloads for TMC operations.
// This enables TMC to identify which workloads should be managed by a policy.
type WorkloadSelector struct {
	// LabelSelector selects workloads by labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// WorkloadTypes specifies specific workload types to select
	// +optional
	WorkloadTypes []WorkloadType `json:"workloadTypes,omitempty"`

	// NamespaceSelector selects workloads from specific namespaces
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

// WorkloadType represents a specific type of workload
type WorkloadType struct {
	// APIVersion of the workload resource
	APIVersion string `json:"apiVersion"`

	// Kind of the workload resource
	Kind string `json:"kind"`
}

// ClusterSelector defines how to select clusters for TMC operations.
// This enables TMC to identify which clusters should be used for workload placement.
type ClusterSelector struct {
	// LabelSelector selects clusters by labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// LocationSelector selects clusters by location/region
	// +optional
	LocationSelector []string `json:"locationSelector,omitempty"`

	// ClusterNames specifies explicit cluster names to select
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`
}

// WorkloadReference identifies a workload being managed by TMC
type WorkloadReference struct {
	// APIVersion of the workload
	APIVersion string `json:"apiVersion"`

	// Kind of the workload
	Kind string `json:"kind"`

	// Name of the workload
	Name string `json:"name"`

	// Namespace of the workload
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// ClusterName identifies the cluster containing the workload
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
}