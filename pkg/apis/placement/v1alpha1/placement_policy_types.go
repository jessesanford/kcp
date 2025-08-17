package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,categories=kcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Strategy",type="string",JSONPath=`.spec.strategy`
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Placed",type="integer",JSONPath=`.status.placedReplicas`
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// PlacementPolicy defines how workloads should be placed across SyncTargets.
// It provides declarative scheduling policies for workload distribution based on
// various constraints, preferences, and strategies.
type PlacementPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired placement behavior for workloads.
	Spec PlacementPolicySpec `json:"spec"`

	// Status reflects the current state of the placement policy.
	// +optional
	Status PlacementPolicyStatus `json:"status,omitempty"`
}

// PlacementPolicySpec defines the desired placement behavior for workloads.
// It specifies which workloads to target, placement strategies, resource requirements,
// and various constraints for intelligent workload distribution.
type PlacementPolicySpec struct {
	// TargetWorkload identifies which workloads this policy applies to.
	// It can match by API version, kind, name, or label selector.
	TargetWorkload WorkloadSelector `json:"targetWorkload"`

	// Strategy defines the placement approach to use for workload distribution.
	// +kubebuilder:validation:Enum=Singleton;HighAvailability;Spread;Binpack
	Strategy PlacementStrategy `json:"strategy"`

	// Replicas specifies the desired number of replicas across all locations.
	// If not specified, defaults are applied based on the strategy.
	// +optional
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// LocationSelectors define criteria for selecting eligible SyncTargets.
	// Multiple selectors are OR'd together.
	// +optional
	LocationSelectors []LocationSelector `json:"locationSelectors,omitempty"`

	// ResourceRequirements specify the resource needs for each replica.
	// Used by the scheduler to ensure sufficient capacity exists.
	// +optional
	ResourceRequirements ResourceRequirements `json:"resourceRequirements,omitempty"`

	// Tolerations allow placement on SyncTargets with matching taints.
	// Similar to Kubernetes pod tolerations but for location-level taints.
	// +optional
	Tolerations []Toleration `json:"tolerations,omitempty"`

	// SpreadConstraints control how replicas are distributed across topology domains.
	// They ensure replicas are spread according to specified rules.
	// +optional
	SpreadConstraints []SpreadConstraint `json:"spreadConstraints,omitempty"`

	// AffinityRules specify preferences for co-location or separation of workloads.
	// They influence but don't mandate placement decisions.
	// +optional
	AffinityRules *AffinityRules `json:"affinityRules,omitempty"`
}

// WorkloadSelector identifies target workloads for the placement policy.
// It supports matching by API details and labels.
type WorkloadSelector struct {
	// APIVersion of the target workload resources.
	APIVersion string `json:"apiVersion"`

	// Kind of the target workload resources.
	Kind string `json:"kind"`

	// Name specifies a specific workload by name. If empty, matches all workloads
	// of the specified kind that match other criteria.
	// +optional
	Name string `json:"name,omitempty"`

	// LabelSelector for matching workloads by labels.
	// If specified, only workloads matching the selector are targeted.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// PlacementStrategy defines the approach for distributing workload replicas.
type PlacementStrategy string

const (
	// PlacementStrategySingleton places all replicas on exactly one location.
	// Used for workloads that cannot be distributed or have strict locality requirements.
	PlacementStrategySingleton PlacementStrategy = "Singleton"

	// PlacementStrategyHighAvailability ensures redundancy across failure domains.
	// Distributes replicas to maximize availability and fault tolerance.
	PlacementStrategyHighAvailability PlacementStrategy = "HighAvailability"

	// PlacementStrategySpread distributes replicas evenly across available locations.
	// Balances load while considering location capacity and constraints.
	PlacementStrategySpread PlacementStrategy = "Spread"

	// PlacementStrategyBinpack consolidates replicas to minimize the number of locations used.
	// Optimizes for resource efficiency and cost reduction.
	PlacementStrategyBinpack PlacementStrategy = "Binpack"
)

// LocationSelector defines criteria for selecting eligible SyncTargets.
// It supports direct name selection, label-based selection, and cell-based selection.
type LocationSelector struct {
	// Name directly specifies a SyncTarget by name.
	// Takes precedence over other selection methods if specified.
	// +optional
	Name string `json:"name,omitempty"`

	// LabelSelector for matching SyncTargets by labels.
	// Only SyncTargets matching all specified labels are eligible.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// CellSelector enables selection based on KCP cell membership.
	// Provides cell-level granularity for location selection.
	// +optional
	CellSelector *CellSelector `json:"cellSelector,omitempty"`
}

// CellSelector selects SyncTargets based on KCP cell membership.
// Cells provide logical grouping of SyncTargets for management purposes.
type CellSelector struct {
	// MatchLabels specifies labels that cells must have for their SyncTargets to be eligible.
	// All specified labels must match exactly.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// RequiredDuringScheduling specifies cell names that must be available during scheduling.
	// Placement will fail if none of these cells are available.
	// +optional
	RequiredDuringScheduling []string `json:"requiredDuringScheduling,omitempty"`
}

// ResourceRequirements specify the compute resources needed for workload replicas.
// Used by the scheduler to ensure target locations have sufficient capacity.
type ResourceRequirements struct {
	// Requests specify the minimum resources required for each replica.
	// The scheduler ensures target locations can satisfy these requirements.
	// +optional
	Requests ResourceList `json:"requests,omitempty"`

	// Limits specify the maximum resources each replica is allowed to consume.
	// Used for capacity planning and resource quotas.
	// +optional
	Limits ResourceList `json:"limits,omitempty"`
}

// ResourceList is a map of resource names to quantities.
// It mirrors Kubernetes ResourceList but may include custom resource types.
type ResourceList map[string]resource.Quantity

// Toleration allows placement on SyncTargets with matching taints.
// Similar to Kubernetes tolerations but applied at the SyncTarget level.
type Toleration struct {
	// Key is the taint key that the toleration matches.
	Key string `json:"key"`

	// Value is the taint value the toleration matches.
	// Only required if Operator is "Equal".
	// +optional
	Value string `json:"value,omitempty"`

	// Operator represents the key-value relationship.
	// Valid operators are "Equal" and "Exists".
	// +kubebuilder:validation:Enum=Equal;Exists
	// +optional
	Operator TolerationOperator `json:"operator,omitempty"`

	// Effect indicates which taint effect to tolerate.
	// Empty means tolerate all effects.
	// +optional
	Effect string `json:"effect,omitempty"`
}

// TolerationOperator is the operator for matching taints and tolerations.
type TolerationOperator string

const (
	// TolerationOpEqual means the key and value must match exactly.
	TolerationOpEqual TolerationOperator = "Equal"

	// TolerationOpExists means only the key must exist (value ignored).
	TolerationOpExists TolerationOperator = "Exists"
)

// SpreadConstraint defines how replicas should be distributed across topology domains.
// It ensures workloads are spread according to failure domain or other topology considerations.
type SpreadConstraint struct {
	// TopologyKey is the key of SyncTarget labels that defines the topology domain.
	// Replicas will be spread across different values of this key.
	TopologyKey string `json:"topologyKey"`

	// MaxSkew describes the maximum allowed difference between the number of replicas
	// in any two topology domains. Must be greater than zero.
	// +kubebuilder:validation:Minimum=1
	MaxSkew int32 `json:"maxSkew"`

	// WhenUnsatisfiable indicates how to deal with a replica if it doesn't satisfy
	// the spread constraint.
	// +kubebuilder:validation:Enum=DoNotSchedule;ScheduleAnyway
	WhenUnsatisfiable UnsatisfiableConstraintAction `json:"whenUnsatisfiable"`
}

// UnsatisfiableConstraintAction defines actions to take when constraints cannot be satisfied.
type UnsatisfiableConstraintAction string

const (
	// DoNotSchedule instructs the scheduler not to schedule replicas that would violate constraints.
	DoNotSchedule UnsatisfiableConstraintAction = "DoNotSchedule"

	// ScheduleAnyway allows scheduling even if constraints are violated, but with lower priority.
	ScheduleAnyway UnsatisfiableConstraintAction = "ScheduleAnyway"
)

// AffinityRules define preferences for co-location or separation of workloads.
// They influence placement decisions but don't create hard constraints.
type AffinityRules struct {
	// WorkloadAffinity specifies preferences for co-locating with other workloads.
	// Higher weights indicate stronger preferences.
	// +optional
	WorkloadAffinity []WorkloadAffinityTerm `json:"workloadAffinity,omitempty"`

	// WorkloadAntiAffinity specifies preferences for avoiding co-location with other workloads.
	// Higher weights indicate stronger avoidance preferences.
	// +optional
	WorkloadAntiAffinity []WorkloadAffinityTerm `json:"workloadAntiAffinity,omitempty"`
}

// WorkloadAffinityTerm defines affinity or anti-affinity with other workloads.
// It combines workload selection criteria with topology requirements and preference weights.
type WorkloadAffinityTerm struct {
	// LabelSelector identifies other workloads to have affinity with.
	LabelSelector *metav1.LabelSelector `json:"labelSelector"`

	// TopologyKey specifies the scope for the affinity rule.
	// Workloads are considered co-located if they share the same value for this key.
	TopologyKey string `json:"topologyKey"`

	// Weight associated with matching the corresponding affinityTerm.
	// Valid values are 1-100. Higher weights have stronger influence on placement.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Weight *int32 `json:"weight,omitempty"`
}

// PlacementPolicyStatus defines the observed state of a PlacementPolicy.
// It tracks the current phase, placed replicas, and placement decisions.
type PlacementPolicyStatus struct {
	// Phase indicates the current lifecycle phase of the placement policy.
	// +kubebuilder:validation:Enum=Pending;Scheduling;Scheduled;Failed
	// +optional
	Phase PlacementPhase `json:"phase,omitempty"`

	// PlacedReplicas is the number of replicas successfully placed across all locations.
	PlacedReplicas int32 `json:"placedReplicas"`

	// Placements contains the specific placement decisions made by the scheduler.
	// Each entry represents replicas placed on a specific SyncTarget.
	// +optional
	Placements []PlacementDecision `json:"placements,omitempty"`

	// LastScheduleTime records when placement was last attempted by the scheduler.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// Conditions represent the latest available observations of the placement policy's state.
	// Standard condition types include "Scheduled" and "ConstraintsSatisfied".
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
}

// PlacementPhase represents the current lifecycle phase of a placement policy.
type PlacementPhase string

const (
	// PlacementPhasePending indicates the policy has been created but not yet processed.
	PlacementPhasePending PlacementPhase = "Pending"

	// PlacementPhaseScheduling indicates the scheduler is actively working on placement decisions.
	PlacementPhaseScheduling PlacementPhase = "Scheduling"

	// PlacementPhaseScheduled indicates successful placement of all replicas.
	PlacementPhaseScheduled PlacementPhase = "Scheduled"

	// PlacementPhaseFailed indicates the scheduler could not satisfy the placement requirements.
	PlacementPhaseFailed PlacementPhase = "Failed"
)

// PlacementDecision represents a specific placement made by the scheduler.
// It records which SyncTarget received replicas and why.
type PlacementDecision struct {
	// LocationName is the name of the SyncTarget where replicas are placed.
	LocationName string `json:"locationName"`

	// Replicas is the number of replicas placed on this SyncTarget.
	Replicas int32 `json:"replicas"`

	// Reason provides a human-readable explanation for why this placement was made.
	// Used for debugging and policy tuning.
	// +optional
	Reason string `json:"reason,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlacementPolicyList contains a list of PlacementPolicy objects.
// It follows the standard Kubernetes list conventions for API responses.
type PlacementPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of PlacementPolicy objects.
	Items []PlacementPolicy `json:"items"`
}
