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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TMCConfig represents the configuration for the TMC (Transparent Multi-Cluster) functionality.
// It defines feature flags and global settings that control TMC behavior across the system.
type TMCConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TMCConfigSpec   `json:"spec,omitempty"`
	Status TMCConfigStatus `json:"status,omitempty"`
}

// TMCConfigSpec defines the desired state of TMCConfig.
type TMCConfigSpec struct {
	// FeatureFlags controls which TMC features are enabled
	// +optional
	FeatureFlags map[string]bool `json:"featureFlags,omitempty"`
}

// TMCConfigStatus defines the observed state of TMCConfig.
type TMCConfigStatus struct {
	// Conditions represent the current observed conditions of the TMC configuration
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true

// TMCConfigList contains a list of TMCConfig.
type TMCConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TMCConfig `json:"items"`
}

// TMCStatus represents the base status type for TMC resources.
// It provides a common status structure that can be embedded by other TMC types.
type TMCStatus struct {
	// Conditions represent the current observed conditions
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase represents the current phase of the resource lifecycle
	// +optional
	Phase string `json:"phase,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed TMC resource spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ResourceIdentifier uniquely identifies a Kubernetes resource within the TMC system.
// It provides a standardized way to reference resources across different clusters and workspaces.
type ResourceIdentifier struct {
	// Group is the API group name (e.g., "apps", "extensions")
	Group string `json:"group"`

	// Version is the API version (e.g., "v1", "v1beta1", "v1alpha1")
	Version string `json:"version"`

	// Resource is the resource name (plural form, e.g., "deployments", "services")
	Resource string `json:"resource"`

	// Kind is the resource kind (e.g., "Deployment", "Service")
	// +optional
	Kind string `json:"kind,omitempty"`

	// Namespace is the resource namespace (empty for cluster-scoped resources)
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name is the resource name
	// +optional
	Name string `json:"name,omitempty"`
}

// ClusterIdentifier uniquely identifies a cluster within the TMC system.
// It provides a standardized way to reference physical or logical clusters.
type ClusterIdentifier struct {
	// Name is the cluster name
	Name string `json:"name"`

	// Region represents the geographical region of the cluster
	// +optional
	Region string `json:"region,omitempty"`

	// Zone represents the availability zone within the region
	// +optional
	Zone string `json:"zone,omitempty"`

	// Provider identifies the cloud provider (e.g., "aws", "gcp", "azure")
	// +optional
	Provider string `json:"provider,omitempty"`

	// Environment represents the environment type (e.g., "prod", "staging", "dev")
	// +optional
	Environment string `json:"environment,omitempty"`

	// Labels provides additional metadata for cluster classification
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}
