/*
Copyright 2025 The KCP Authors.

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

// Placement represents a policy for placing workloads across SyncTargets.
// It defines how resources should be distributed, scheduled, and managed
// across the available clusters with workspace awareness.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp,shortName=pl
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`,description="Placement strategy"
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source.workspace`,description="Source workspace"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`,description="Ready condition status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Placement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired placement policy
	Spec PlacementSpec `json:"spec,omitempty"`

	// Status represents the current state of the placement
	// +optional
	Status PlacementStatus `json:"status,omitempty"`
}

// PlacementSpec defines the desired placement policy per PRD requirements
type PlacementSpec struct {
	// Source workspace and workload reference
	Source WorkloadReference `json:"source"`

	// LocationSelector specifies target location selection
	LocationSelector LocationSelector `json:"locationSelector"`

	// Constraints define scheduling constraints
	Constraints PlacementConstraints `json:"constraints"`

	// Strategy defines placement strategy (one-to-any, one-to-many)
	Strategy PlacementStrategy `json:"strategy"`
}

// WorkloadReference references a workload in a specific workspace
type WorkloadReference struct {
	// Workspace containing the workload
	Workspace string `json:"workspace"`

	// Namespace containing the workload (if namespaced)
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the workload
	Name string `json:"name"`

	// Kind of the workload
	Kind string `json:"kind"`

	// APIVersion of the workload
	APIVersion string `json:"apiVersion"`
}

// LocationSelector selects target locations for placement
type LocationSelector struct {
	// Path specifies the location path pattern to match
	// +optional
	Path string `json:"path,omitempty"`

	// LabelSelector selects locations based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// PlacementConstraints define scheduling constraints
type PlacementConstraints struct {
	// ResourceRequirements specify resource requirements
	// +optional
	ResourceRequirements *ResourceRequirements `json:"resourceRequirements,omitempty"`

	// Affinity specifies affinity constraints
	// +optional
	Affinity *PlacementAffinity `json:"affinity,omitempty"`

	// Tolerations specify tolerations for cluster taints
	// +optional
	Tolerations []PlacementToleration `json:"tolerations,omitempty"`

	// TopologySpreadConstraints defines distribution across topology domains
	// +optional
	TopologySpreadConstraints []TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// ResourceRequirements specify resource requirements for placement
type ResourceRequirements struct {
	// CPU requirements in millicores
	// +optional
	CPU *int64 `json:"cpu,omitempty"`

	// Memory requirements in bytes
	// +optional
	Memory *int64 `json:"memory,omitempty"`

	// Storage requirements in bytes
	// +optional
	Storage *int64 `json:"storage,omitempty"`
}

// PlacementStrategy defines placement strategies
// +kubebuilder:validation:Enum=OneToAny;OneToMany;Spread
type PlacementStrategy string

const (
	// OneToAnyStrategy places workload in a single best-match cluster
	OneToAnyStrategy PlacementStrategy = "OneToAny"

	// OneToManyStrategy replicates workload to all matching clusters
	OneToManyStrategy PlacementStrategy = "OneToMany"

	// SpreadStrategy distributes workload across clusters with constraints
	SpreadStrategy PlacementStrategy = "Spread"
)

// PlacementAffinity defines affinity constraints
type PlacementAffinity struct {
	// ClusterAffinity specifies cluster affinity
	// +optional
	ClusterAffinity *ClusterAffinity `json:"clusterAffinity,omitempty"`

	// ClusterAntiAffinity specifies cluster anti-affinity
	// +optional
	ClusterAntiAffinity *ClusterAntiAffinity `json:"clusterAntiAffinity,omitempty"`
}

// ClusterAffinity defines cluster affinity
type ClusterAffinity struct {
	// RequiredDuringSchedulingIgnoredDuringExecution specifies hard constraints
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution *ClusterAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`

	// PreferredDuringSchedulingIgnoredDuringExecution specifies soft constraints
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedClusterAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// ClusterAntiAffinity defines cluster anti-affinity
type ClusterAntiAffinity struct {
	// RequiredDuringSchedulingIgnoredDuringExecution specifies hard anti-affinity
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution *ClusterAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`

	// PreferredDuringSchedulingIgnoredDuringExecution specifies soft anti-affinity
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedClusterAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// ClusterAffinityTerm defines a cluster affinity term
type ClusterAffinityTerm struct {
	// LabelSelector selects clusters based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Locations specifies location constraints
	// +optional
	Locations []string `json:"locations,omitempty"`
}

// WeightedClusterAffinityTerm defines a weighted cluster affinity term
type WeightedClusterAffinityTerm struct {
	// Weight associated with the term, range 1-100
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Weight int32 `json:"weight"`

	// ClusterAffinityTerm specifies the cluster affinity term
	ClusterAffinityTerm ClusterAffinityTerm `json:"clusterAffinityTerm"`
}

// PlacementToleration defines toleration for cluster taints
type PlacementToleration struct {
	// Key is the taint key that the toleration applies to
	// +optional
	Key string `json:"key,omitempty"`

	// Operator represents the relationship between the key and value
	// +kubebuilder:validation:Enum=Exists;Equal
	// +kubebuilder:default=Equal
	Operator TolerationOperator `json:"operator,omitempty"`

	// Value is the taint value that the toleration matches
	// +optional
	Value string `json:"value,omitempty"`

	// Effect indicates which taint effect this toleration matches
	// +optional
	Effect TaintEffect `json:"effect,omitempty"`

	// TolerationSeconds specifies how long the pod can be bound to a tainted cluster
	// +optional
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"`
}

// TolerationOperator defines toleration operators
// +kubebuilder:validation:Enum=Exists;Equal
type TolerationOperator string

const (
	// TolerationOpExists means the toleration matches any taint with the matching key
	TolerationOpExists TolerationOperator = "Exists"

	// TolerationOpEqual means the toleration matches taints with matching key and value
	TolerationOpEqual TolerationOperator = "Equal"
)

// TaintEffect defines taint effects
type TaintEffect string

const (
	// TaintEffectNoSchedule means workloads will not be scheduled to the cluster
	TaintEffectNoSchedule TaintEffect = "NoSchedule"

	// TaintEffectPreferNoSchedule means workloads will prefer not to be scheduled to the cluster
	TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"

	// TaintEffectNoExecute means workloads will not be scheduled and existing ones will be evicted
	TaintEffectNoExecute TaintEffect = "NoExecute"
)

// TopologySpreadConstraint defines workload distribution across topology domains
type TopologySpreadConstraint struct {
	// TopologyKey specifies the topology domain key
	TopologyKey string `json:"topologyKey"`

	// WhenUnsatisfiable specifies behavior when constraint cannot be satisfied
	// +kubebuilder:validation:Enum=DoNotSchedule;ScheduleAnyway
	WhenUnsatisfiable UnsatisfiableConstraintAction `json:"whenUnsatisfiable"`

	// MaxSkew defines the maximum difference in workload distribution
	// +kubebuilder:validation:Minimum=1
	MaxSkew int32 `json:"maxSkew"`

	// LabelSelector selects workloads subject to this constraint
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// UnsatisfiableConstraintAction defines actions for unsatisfiable constraints
// +kubebuilder:validation:Enum=DoNotSchedule;ScheduleAnyway
type UnsatisfiableConstraintAction string

const (
	// DoNotSchedule means don't schedule when constraint cannot be satisfied
	DoNotSchedule UnsatisfiableConstraintAction = "DoNotSchedule"

	// ScheduleAnyway means schedule even when constraint cannot be satisfied
	ScheduleAnyway UnsatisfiableConstraintAction = "ScheduleAnyway"
)

// PlacementStatus represents the observed state of the placement per PRD requirements
type PlacementStatus struct {
	// Conditions represent the latest available observations of the placement's current state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// PlacementDecisions lists the placement decisions made
	// +optional
	PlacementDecisions []PlacementDecision `json:"placementDecisions,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed placement spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// PlacementDecision represents a placement decision
type PlacementDecision struct {
	// Cluster is the name of the selected cluster
	Cluster string `json:"cluster"`

	// Reason explains why this cluster was selected
	// +optional
	Reason string `json:"reason,omitempty"`

	// Weight is the score assigned to this cluster
	// +optional
	Weight int32 `json:"weight,omitempty"`
}

// PlacementList contains a list of Placement
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of Placement objects.
	Items []Placement `json:"items"`
}

// Condition types for Placement resources
const (
	// PlacementReady means the placement policy has been successfully applied
	PlacementReady conditionsv1alpha1.ConditionType = "Ready"

	// PlacementScheduled means clusters have been selected according to the policy
	PlacementScheduled conditionsv1alpha1.ConditionType = "Scheduled"
)

// Condition implementation for Placement resource
func (p *Placement) GetConditions() conditionsv1alpha1.Conditions {
	return p.Status.Conditions
}

func (p *Placement) SetConditions(conditions conditionsv1alpha1.Conditions) {
	p.Status.Conditions = conditions
}