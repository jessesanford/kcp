package placement

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kcp-dev/kcp/pkg/logicalcluster"
)

// PlacementStrategy represents a placement strategy type
type PlacementStrategy string

const (
	PlacementStrategySpread             PlacementStrategy = "Spread"
	PlacementStrategyBinpack            PlacementStrategy = "Binpack"
	PlacementStrategyHighAvailability   PlacementStrategy = "HighAvailability"
	PlacementStrategySingleton          PlacementStrategy = "Singleton"
)

// ResourceList represents a set of named resource quantities
type ResourceList map[string]string

// PlacementPolicy defines placement requirements and preferences for workloads
type PlacementPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Strategy specifies the placement strategy
	Strategy PlacementStrategy `json:"strategy,omitempty"`
	
	// Tolerations allow workloads to be placed on nodes with matching taints
	Tolerations []Toleration `json:"tolerations,omitempty"`
	
	// SpreadConstraints define how replicas should be distributed
	SpreadConstraints []SpreadConstraint `json:"spreadConstraints,omitempty"`
	
	// AffinityRules define workload co-location preferences
	AffinityRules *AffinityRules `json:"affinityRules,omitempty"`
}

// Toleration represents a toleration for taints
type Toleration struct {
	Key      string `json:"key,omitempty"`
	Operator string `json:"operator,omitempty"`
	Value    string `json:"value,omitempty"`
	Effect   string `json:"effect,omitempty"`
}

// SpreadConstraint defines distribution requirements
type SpreadConstraint struct {
	TopologyKey string `json:"topologyKey"`
	MaxSkew     int32  `json:"maxSkew,omitempty"`
}

// AffinityRules define workload co-location rules
type AffinityRules struct {
	NodeAffinity    *NodeAffinity    `json:"nodeAffinity,omitempty"`
	PodAffinity     *PodAffinity     `json:"podAffinity,omitempty"`
	PodAntiAffinity *PodAntiAffinity `json:"podAntiAffinity,omitempty"`
}

// NodeAffinity defines node affinity rules
type NodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// NodeSelector represents node selector requirements
type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `json:"nodeSelectorTerms"`
}

// NodeSelectorTerm represents a selector term
type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `json:"matchExpressions,omitempty"`
	MatchFields      []NodeSelectorRequirement `json:"matchFields,omitempty"`
}

// NodeSelectorRequirement represents a selector requirement
type NodeSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// PreferredSchedulingTerm represents a preferred scheduling term
type PreferredSchedulingTerm struct {
	Weight     int32                `json:"weight"`
	Preference NodeSelectorTerm     `json:"preference"`
}

// PodAffinity defines pod affinity rules
type PodAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAntiAffinity defines pod anti-affinity rules
type PodAntiAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAffinityTerm defines pod affinity term
type PodAffinityTerm struct {
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	Namespaces    []string              `json:"namespaces,omitempty"`
	TopologyKey   string                `json:"topologyKey"`
}

// WeightedPodAffinityTerm defines weighted pod affinity term
type WeightedPodAffinityTerm struct {
	Weight          int32           `json:"weight"`
	PodAffinityTerm PodAffinityTerm `json:"podAffinityTerm"`
}

// SyncTarget represents a target cluster for workload placement
type SyncTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the sync target
	Spec SyncTargetSpec `json:"spec,omitempty"`

	// Status defines the observed state of the sync target
	Status SyncTargetStatus `json:"status,omitempty"`
}

// SyncTargetSpec defines the spec of a sync target
type SyncTargetSpec struct {
	// Cluster identifies the target cluster
	Cluster string `json:"cluster,omitempty"`
	
	// Unschedulable marks the target as unavailable for scheduling
	Unschedulable bool `json:"unschedulable,omitempty"`
	
	// EvictAfter specifies when to evict workloads
	EvictAfter *metav1.Time `json:"evictAfter,omitempty"`
}

// SyncTargetStatus defines the status of a sync target
type SyncTargetStatus struct {
	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	
	// Capacity represents the total resources of the target
	Capacity ResourceList `json:"capacity,omitempty"`
	
	// Allocatable represents the allocatable resources
	Allocatable ResourceList `json:"allocatable,omitempty"`
	
	// Location specifies the target's location
	Location string `json:"location,omitempty"`
}

// Taint represents a node taint
type Taint struct {
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect"`
}

// PlacementEngine determines where workloads should be placed
type PlacementEngine interface {
	// ComputePlacement calculates placement for a workload
	ComputePlacement(
		ctx context.Context,
		workload *unstructured.Unstructured,
		policy *PlacementPolicy,
		targets []*SyncTarget,
	) (*PlacementDecision, error)

	// ValidatePlacement checks if placement is valid
	ValidatePlacement(
		ctx context.Context,
		decision *PlacementDecision,
	) error

	// ReconcilePlacement updates existing placement
	ReconcilePlacement(
		ctx context.Context,
		current *PlacementDecision,
		policy *PlacementPolicy,
	) (*PlacementDecision, error)

	// GetCapabilities returns engine capabilities
	GetCapabilities() EngineCapabilities
}

// PlacementDecision represents where to place workloads
type PlacementDecision struct {
	// Workspace where decision was made
	Workspace logicalcluster.Name

	// Placements per location
	Placements []LocationPlacement

	// TotalReplicas across all locations
	TotalReplicas int32

	// Constraints that were evaluated
	EvaluatedConstraints []ConstraintEvaluation

	// Score of this placement
	Score float64

	// Reason for this decision
	Reason string

	// Timestamp of decision
	Timestamp metav1.Time
}

// LocationPlacement represents placement at a location
type LocationPlacement struct {
	// LocationName (SyncTarget name)
	LocationName string

	// Replicas at this location
	Replicas int32

	// Score for this location
	Score float64

	// Resources allocated
	AllocatedResources ResourceList

	// Constraints satisfied
	SatisfiedConstraints []string

	// Constraints violated
	ViolatedConstraints []string
}

// EngineCapabilities describes what engine can do
type EngineCapabilities struct {
	// SupportedStrategies
	SupportedStrategies []PlacementStrategy

	// MaxLocations engine can handle
	MaxLocations int

	// SupportsRebalancing
	SupportsRebalancing bool

	// SupportsAntiAffinity
	SupportsAntiAffinity bool

	// SupportsTopologySpread
	SupportsTopologySpread bool

	// SupportsPriority
	SupportsPriority bool
}

// ConstraintEvaluation represents constraint check result
type ConstraintEvaluation struct {
	// Type of constraint
	Type string

	// Satisfied or not
	Satisfied bool

	// Score impact
	ScoreImpact float64

	// Message explaining result
	Message string
}

// PlacementController manages placement lifecycle
type PlacementController interface {
	// Start begins the controller
	Start(ctx context.Context) error

	// Stop halts the controller
	Stop() error

	// EnqueuePlacement queues a placement for processing
	EnqueuePlacement(key string)

	// GetEngine returns the placement engine
	GetEngine() PlacementEngine
}