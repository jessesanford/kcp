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
	"k8s.io/apimachinery/pkg/runtime"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// ClusterRegistration represents a physical cluster registered with the TMC for workload management.
// It provides cluster metadata, capabilities, and connection information required for
// transparent multi-cluster operations with full workspace isolation.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp,shortName=cr
// +kubebuilder:printcolumn:name="Location",type=string,JSONPath=`.spec.location`,description="Cluster location"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`,description="Ready condition status"
// +kubebuilder:printcolumn:name="Capacity",type=string,JSONPath=`.status.capacity.cpu`,description="CPU capacity in millicores"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the ClusterRegistration
	Spec ClusterRegistrationSpec `json:"spec,omitempty"`

	// Status represents the current state of the ClusterRegistration
	// +optional
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of the ClusterRegistration
type ClusterRegistrationSpec struct {
	// Location specifies the logical location of this cluster for placement decisions
	Location string `json:"location"`

	// ClusterEndpoint defines the connection information for the cluster
	ClusterEndpoint ClusterEndpoint `json:"clusterEndpoint"`

	// Capabilities define the cluster's capabilities for workload placement
	// +optional
	Capabilities []ClusterCapability `json:"capabilities,omitempty"`

	// ResourceQuotas define resource limits for this cluster
	// +optional
	ResourceQuotas []ResourceQuota `json:"resourceQuotas,omitempty"`

	// Taints specify cluster taints that affect workload placement
	// +optional
	Taints []ClusterTaint `json:"taints,omitempty"`

	// MaintenanceWindows define scheduled maintenance periods
	// +optional
	MaintenanceWindows []MaintenanceWindow `json:"maintenanceWindows,omitempty"`

	// NodeSelector specifies node selection criteria for this cluster
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// ClusterEndpoint defines cluster connection information
type ClusterEndpoint struct {
	// URL is the cluster's API server endpoint
	URL string `json:"url"`

	// CABundle contains the cluster's CA certificate bundle
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// Insecure indicates whether to skip TLS verification (not recommended for production)
	// +optional
	Insecure bool `json:"insecure,omitempty"`

	// Credentials specify authentication credentials for the cluster
	// +optional
	Credentials *ClusterCredentials `json:"credentials,omitempty"`
}

// ClusterCredentials define cluster authentication credentials
type ClusterCredentials struct {
	// SecretRef references a secret containing cluster credentials
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`

	// ServiceAccountRef references a service account for cluster access
	// +optional
	ServiceAccountRef *ServiceAccountReference `json:"serviceAccountRef,omitempty"`

	// TokenRef references a token for cluster authentication
	// +optional
	TokenRef *TokenReference `json:"tokenRef,omitempty"`
}

// SecretReference references a secret
type SecretReference struct {
	// Name of the secret
	Name string `json:"name"`

	// Namespace of the secret
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Key within the secret containing the credential
	// +optional
	Key string `json:"key,omitempty"`
}

// ServiceAccountReference references a service account
type ServiceAccountReference struct {
	// Name of the service account
	Name string `json:"name"`

	// Namespace of the service account
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// TokenReference references a token
type TokenReference struct {
	// SecretRef references a secret containing the token
	SecretRef SecretReference `json:"secretRef"`
}

// ClusterCapability defines a cluster capability
type ClusterCapability struct {
	// Type of capability (e.g., "storage", "compute", "networking", "gpu")
	Type string `json:"type"`

	// Available indicates if the capability is currently available
	Available bool `json:"available"`

	// Attributes provide additional capability metadata
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`

	// ResourceTypes lists the resource types supported by this capability
	// +optional
	ResourceTypes []string `json:"resourceTypes,omitempty"`
}

// ResourceQuota defines resource quotas for the cluster
type ResourceQuota struct {
	// ResourceType specifies the resource type (cpu, memory, storage, pods)
	ResourceType string `json:"resourceType"`

	// Hard limit for the resource
	Hard string `json:"hard"`

	// Used amount of the resource
	// +optional
	Used string `json:"used,omitempty"`
}

// ClusterTaint represents a cluster taint that affects workload scheduling
type ClusterTaint struct {
	// Key is the taint key
	Key string `json:"key"`

	// Value is the taint value
	// +optional
	Value string `json:"value,omitempty"`

	// Effect indicates the taint effect on workload scheduling
	// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
	Effect TaintEffect `json:"effect"`

	// TimeAdded represents the time at which the taint was added
	// +optional
	TimeAdded *metav1.Time `json:"timeAdded,omitempty"`
}

// TaintEffect defines taint effects
// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
type TaintEffect string

const (
	// TaintEffectNoSchedule means workloads will not be scheduled to the cluster
	TaintEffectNoSchedule TaintEffect = "NoSchedule"

	// TaintEffectPreferNoSchedule means workloads will prefer not to be scheduled to the cluster
	TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"

	// TaintEffectNoExecute means workloads will not be scheduled and existing ones will be evicted
	TaintEffectNoExecute TaintEffect = "NoExecute"
)

// MaintenanceWindow defines a scheduled maintenance period
type MaintenanceWindow struct {
	// Name of the maintenance window
	Name string `json:"name"`

	// Start time of the maintenance window
	Start metav1.Time `json:"start"`

	// Duration of the maintenance window
	Duration metav1.Duration `json:"duration"`

	// Recurring indicates if this is a recurring maintenance window
	// +optional
	Recurring bool `json:"recurring,omitempty"`

	// RecurrenceRule defines the recurrence pattern (RFC 5545 RRULE format)
	// +optional
	RecurrenceRule string `json:"recurrenceRule,omitempty"`
}

// ClusterRegistrationStatus represents the observed state of the ClusterRegistration
type ClusterRegistrationStatus struct {
	// Conditions represent the latest available observations of the cluster's current state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase represents the current phase of cluster registration
	// +optional
	Phase ClusterRegistrationPhase `json:"phase,omitempty"`

	// Capacity represents the total resources available on the cluster
	// +optional
	Capacity ClusterCapacityStatus `json:"capacity,omitempty"`

	// Allocated represents the currently allocated resources on the cluster
	// +optional
	Allocated ClusterCapacityStatus `json:"allocated,omitempty"`

	// LastHeartbeatTime is the last time the cluster sent a heartbeat
	// +optional
	LastHeartbeatTime *metav1.Time `json:"lastHeartbeatTime,omitempty"`

	// Version contains cluster version information
	// +optional
	Version *ClusterVersion `json:"version,omitempty"`

	// Health provides cluster health information
	// +optional
	Health *ClusterHealth `json:"health,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ClusterRegistrationPhase defines the phase of cluster registration
// +kubebuilder:validation:Enum=Pending;Registering;Ready;Failed;Unreachable
type ClusterRegistrationPhase string

const (
	// ClusterRegistrationPhasePending means the cluster registration is pending
	ClusterRegistrationPhasePending ClusterRegistrationPhase = "Pending"

	// ClusterRegistrationPhaseRegistering means the cluster is being registered
	ClusterRegistrationPhaseRegistering ClusterRegistrationPhase = "Registering"

	// ClusterRegistrationPhaseReady means the cluster is registered and ready
	ClusterRegistrationPhaseReady ClusterRegistrationPhase = "Ready"

	// ClusterRegistrationPhaseFailed means the cluster registration failed
	ClusterRegistrationPhaseFailed ClusterRegistrationPhase = "Failed"

	// ClusterRegistrationPhaseUnreachable means the cluster is unreachable
	ClusterRegistrationPhaseUnreachable ClusterRegistrationPhase = "Unreachable"
)

// ClusterCapacityStatus represents cluster resource capacity information
type ClusterCapacityStatus struct {
	// CPU capacity in millicores
	// +optional
	CPU *int64 `json:"cpu,omitempty"`

	// Memory capacity in bytes
	// +optional
	Memory *int64 `json:"memory,omitempty"`

	// Storage capacity in bytes
	// +optional
	Storage *int64 `json:"storage,omitempty"`

	// Pods represents maximum pod capacity
	// +optional
	Pods *int32 `json:"pods,omitempty"`

	// GPU capacity
	// +optional
	GPU *int32 `json:"gpu,omitempty"`
}

// ClusterVersion contains cluster version information
type ClusterVersion struct {
	// Kubernetes version
	// +optional
	Kubernetes string `json:"kubernetes,omitempty"`

	// Platform version (e.g., OpenShift, EKS, GKE)
	// +optional
	Platform string `json:"platform,omitempty"`

	// TMC agent version running on the cluster
	// +optional
	TMCAgent string `json:"tmcAgent,omitempty"`
}

// ClusterHealth provides cluster health information
type ClusterHealth struct {
	// Ready indicates if the cluster is ready to accept workloads
	Ready bool `json:"ready"`

	// NodeCount is the number of ready nodes in the cluster
	// +optional
	NodeCount *int32 `json:"nodeCount,omitempty"`

	// ComponentStatuses lists the status of cluster components
	// +optional
	ComponentStatuses []ComponentStatus `json:"componentStatuses,omitempty"`
}

// ComponentStatus represents the status of a cluster component
type ComponentStatus struct {
	// Name of the component
	Name string `json:"name"`

	// Status indicates if the component is healthy
	Status string `json:"status"`

	// Message provides additional information about the component status
	// +optional
	Message string `json:"message,omitempty"`
}

// WorkloadPlacement represents a policy for placing workloads across registered clusters.
// It defines sophisticated placement strategies, constraints, and policies for
// transparent multi-cluster workload distribution with workspace awareness.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=kcp,shortName=wp
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.placementPolicy.strategy`,description="Placement strategy"
// +kubebuilder:printcolumn:name="Workspace",type=string,JSONPath=`.spec.workspaceSelector.name`,description="Target workspace"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`,description="Ready condition status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type WorkloadPlacement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired workload placement policy
	Spec WorkloadPlacementSpec `json:"spec,omitempty"`

	// Status represents the current state of the workload placement
	// +optional
	Status WorkloadPlacementStatus `json:"status,omitempty"`
}

// WorkloadPlacementSpec defines the desired workload placement policy
type WorkloadPlacementSpec struct {
	// WorkspaceSelector specifies which workspace this placement applies to
	WorkspaceSelector WorkspaceSelector `json:"workspaceSelector"`

	// ResourceSelector defines which resources this placement applies to
	ResourceSelector ResourceSelector `json:"resourceSelector"`

	// PlacementPolicy defines the placement strategy and constraints
	PlacementPolicy PlacementPolicy `json:"placementPolicy"`

	// SchedulingConstraints define additional scheduling constraints
	// +optional
	SchedulingConstraints *SchedulingConstraints `json:"schedulingConstraints,omitempty"`

	// RolloutStrategy defines how updates should be rolled out
	// +optional
	RolloutStrategy *RolloutStrategy `json:"rolloutStrategy,omitempty"`

	// Priority specifies the priority of this placement policy
	// Higher priority policies take precedence over lower priority ones
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=100
	Priority int32 `json:"priority,omitempty"`
}

// WorkspaceSelector specifies which workspace the placement applies to
type WorkspaceSelector struct {
	// Name of the specific workspace
	// +optional
	Name string `json:"name,omitempty"`

	// LabelSelector selects workspaces based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Path specifies the workspace path pattern
	// +optional
	Path string `json:"path,omitempty"`
}

// ResourceSelector defines which resources the placement applies to
type ResourceSelector struct {
	// APIVersion specifies the API version of resources
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind specifies the kind of resources
	// +optional
	Kind string `json:"kind,omitempty"`

	// LabelSelector selects resources based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// NameSelector specifies resource name patterns
	// +optional
	NameSelector *NameSelector `json:"nameSelector,omitempty"`

	// NamespaceSelector selects resources from specific namespaces
	// +optional
	NamespaceSelector *NamespaceSelector `json:"namespaceSelector,omitempty"`
}

// NameSelector specifies resource name patterns
type NameSelector struct {
	// MatchNames lists specific resource names
	// +optional
	MatchNames []string `json:"matchNames,omitempty"`

	// MatchPatterns lists regex patterns for resource names
	// +optional
	MatchPatterns []string `json:"matchPatterns,omitempty"`
}

// NamespaceSelector selects namespaces for resource selection
type NamespaceSelector struct {
	// MatchNames lists specific namespace names
	// +optional
	MatchNames []string `json:"matchNames,omitempty"`

	// LabelSelector selects namespaces based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// PlacementPolicy defines the placement strategy and constraints
type PlacementPolicy struct {
	// Strategy defines the placement strategy
	// +kubebuilder:validation:Enum=Spread;Binpack;Affinity;Custom
	// +kubebuilder:default=Spread
	Strategy PlacementStrategy `json:"strategy,omitempty"`

	// ClusterSelector specifies target cluster selection criteria
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// Replicas specifies the desired number of replicas across clusters
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// ReplicaScheduling defines how replicas should be distributed
	// +optional
	ReplicaScheduling *ReplicaScheduling `json:"replicaScheduling,omitempty"`

	// Tolerations specify tolerations for cluster taints
	// +optional
	Tolerations []PlacementToleration `json:"tolerations,omitempty"`

	// Affinity specifies cluster affinity rules
	// +optional
	Affinity *PlacementAffinity `json:"affinity,omitempty"`
}

// PlacementStrategy defines placement strategies
// +kubebuilder:validation:Enum=Spread;Binpack;Affinity;Custom
type PlacementStrategy string

const (
	// SpreadPlacementStrategy distributes workloads evenly across clusters
	SpreadPlacementStrategy PlacementStrategy = "Spread"

	// BinpackPlacementStrategy places workloads to maximize resource utilization
	BinpackPlacementStrategy PlacementStrategy = "Binpack"

	// AffinityPlacementStrategy places workloads based on affinity rules
	AffinityPlacementStrategy PlacementStrategy = "Affinity"

	// CustomPlacementStrategy uses custom placement logic
	CustomPlacementStrategy PlacementStrategy = "Custom"
)

// ClusterSelector specifies target cluster selection criteria
type ClusterSelector struct {
	// LabelSelector selects clusters based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// LocationSelector selects clusters based on location
	// +optional
	LocationSelector *LocationSelector `json:"locationSelector,omitempty"`

	// ClusterNames lists specific cluster names
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`

	// CapabilityRequirements specify required cluster capabilities
	// +optional
	CapabilityRequirements []CapabilityRequirement `json:"capabilityRequirements,omitempty"`
}

// LocationSelector selects clusters based on location
type LocationSelector struct {
	// MatchExpressions lists location match expressions
	// +optional
	MatchExpressions []LocationMatchExpression `json:"matchExpressions,omitempty"`

	// Regions lists specific regions
	// +optional
	Regions []string `json:"regions,omitempty"`

	// Zones lists specific availability zones
	// +optional
	Zones []string `json:"zones,omitempty"`
}

// LocationMatchExpression represents a location match expression
type LocationMatchExpression struct {
	// Key is the location key
	Key string `json:"key"`

	// Operator represents the key's relationship to the values
	// +kubebuilder:validation:Enum=In;NotIn;Exists;DoesNotExist
	Operator LocationOperator `json:"operator"`

	// Values is an array of string values
	// +optional
	Values []string `json:"values,omitempty"`
}

// LocationOperator defines location operators
// +kubebuilder:validation:Enum=In;NotIn;Exists;DoesNotExist
type LocationOperator string

const (
	// LocationOperatorIn means the key's value is in the set of values
	LocationOperatorIn LocationOperator = "In"

	// LocationOperatorNotIn means the key's value is not in the set of values
	LocationOperatorNotIn LocationOperator = "NotIn"

	// LocationOperatorExists means the key exists
	LocationOperatorExists LocationOperator = "Exists"

	// LocationOperatorDoesNotExist means the key does not exist
	LocationOperatorDoesNotExist LocationOperator = "DoesNotExist"
)

// CapabilityRequirement specifies a required cluster capability
type CapabilityRequirement struct {
	// Type of capability required
	Type string `json:"type"`

	// Required indicates if this capability is mandatory
	// +optional
	Required bool `json:"required,omitempty"`

	// Attributes specify required capability attributes
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`
}

// ReplicaScheduling defines how replicas should be distributed
type ReplicaScheduling struct {
	// Type defines the replica scheduling type
	// +kubebuilder:validation:Enum=Duplicated;Divided
	// +kubebuilder:default=Duplicated
	Type ReplicaSchedulingType `json:"type,omitempty"`

	// Weight specifies cluster weights for replica distribution
	// +optional
	Weight map[string]int32 `json:"weight,omitempty"`

	// MinReplicas specifies minimum replicas per cluster
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas specifies maximum replicas per cluster
	// +optional
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
}

// ReplicaSchedulingType defines replica scheduling types
// +kubebuilder:validation:Enum=Duplicated;Divided
type ReplicaSchedulingType string

const (
	// DuplicatedReplicaScheduling means replicas are duplicated across all clusters
	DuplicatedReplicaScheduling ReplicaSchedulingType = "Duplicated"

	// DividedReplicaScheduling means replicas are divided across clusters
	DividedReplicaScheduling ReplicaSchedulingType = "Divided"
)

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

	// TolerationSeconds specifies how long workloads can tolerate the taint
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

// PlacementAffinity defines cluster affinity rules
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
	RequiredDuringSchedulingIgnoredDuringExecution []ClusterAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`

	// PreferredDuringSchedulingIgnoredDuringExecution specifies soft constraints
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedClusterAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// ClusterAntiAffinity defines cluster anti-affinity
type ClusterAntiAffinity struct {
	// RequiredDuringSchedulingIgnoredDuringExecution specifies hard anti-affinity
	// +optional
	RequiredDuringSchedulingIgnoredDuringExecution []ClusterAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`

	// PreferredDuringSchedulingIgnoredDuringExecution specifies soft anti-affinity
	// +optional
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedClusterAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// ClusterAffinityTerm defines a cluster affinity term
type ClusterAffinityTerm struct {
	// LabelSelector selects clusters based on labels
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// LocationSelector selects clusters based on location
	// +optional
	LocationSelector *LocationSelector `json:"locationSelector,omitempty"`

	// ClusterNames specifies specific cluster names
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`
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

// SchedulingConstraints define additional scheduling constraints
type SchedulingConstraints struct {
	// ResourceRequirements specify resource requirements
	// +optional
	ResourceRequirements *ResourceRequirements `json:"resourceRequirements,omitempty"`

	// TopologySpreadConstraints define workload distribution across topology domains
	// +optional
	TopologySpreadConstraints []TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// ConflictPolicy defines behavior when placement conflicts occur
	// +kubebuilder:validation:Enum=Fail;Override;Merge
	// +kubebuilder:default=Fail
	ConflictPolicy ConflictPolicy `json:"conflictPolicy,omitempty"`
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

	// GPU requirements
	// +optional
	GPU *int32 `json:"gpu,omitempty"`
}

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

// ConflictPolicy defines behavior when placement conflicts occur
// +kubebuilder:validation:Enum=Fail;Override;Merge
type ConflictPolicy string

const (
	// ConflictPolicyFail means fail when conflicts occur
	ConflictPolicyFail ConflictPolicy = "Fail"

	// ConflictPolicyOverride means override existing placements
	ConflictPolicyOverride ConflictPolicy = "Override"

	// ConflictPolicyMerge means merge with existing placements
	ConflictPolicyMerge ConflictPolicy = "Merge"
)

// RolloutStrategy defines how updates should be rolled out
type RolloutStrategy struct {
	// Type defines the rollout strategy type
	// +kubebuilder:validation:Enum=RollingUpdate;Recreate;BlueGreen;Canary
	// +kubebuilder:default=RollingUpdate
	Type RolloutStrategyType `json:"type,omitempty"`

	// RollingUpdate defines rolling update parameters
	// +optional
	RollingUpdate *RollingUpdateStrategy `json:"rollingUpdate,omitempty"`

	// BlueGreen defines blue-green deployment parameters
	// +optional
	BlueGreen *BlueGreenStrategy `json:"blueGreen,omitempty"`

	// Canary defines canary deployment parameters
	// +optional
	Canary *CanaryStrategy `json:"canary,omitempty"`
}

// RolloutStrategyType defines rollout strategy types
// +kubebuilder:validation:Enum=RollingUpdate;Recreate;BlueGreen;Canary
type RolloutStrategyType string

const (
	// RollingUpdateRolloutStrategy means rolling update rollout
	RollingUpdateRolloutStrategy RolloutStrategyType = "RollingUpdate"

	// RecreateRolloutStrategy means recreate all instances
	RecreateRolloutStrategy RolloutStrategyType = "Recreate"

	// BlueGreenRolloutStrategy means blue-green rollout
	BlueGreenRolloutStrategy RolloutStrategyType = "BlueGreen"

	// CanaryRolloutStrategy means canary rollout
	CanaryRolloutStrategy RolloutStrategyType = "Canary"
)

// RollingUpdateStrategy defines rolling update parameters
type RollingUpdateStrategy struct {
	// MaxUnavailable is the maximum number of pods that can be unavailable during the update
	// +optional
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`

	// MaxSurge is the maximum number of pods that can be created above the desired replica count
	// +optional
	MaxSurge *int32 `json:"maxSurge,omitempty"`

	// Partition indicates the partition number for rolling update
	// +optional
	Partition *int32 `json:"partition,omitempty"`
}

// BlueGreenStrategy defines blue-green deployment parameters
type BlueGreenStrategy struct {
	// PrePromotionAnalysis defines analysis to run before promotion
	// +optional
	PrePromotionAnalysis *AnalysisTemplate `json:"prePromotionAnalysis,omitempty"`

	// PostPromotionAnalysis defines analysis to run after promotion
	// +optional
	PostPromotionAnalysis *AnalysisTemplate `json:"postPromotionAnalysis,omitempty"`

	// ActiveService specifies the active service name
	// +optional
	ActiveService string `json:"activeService,omitempty"`

	// PreviewService specifies the preview service name
	// +optional
	PreviewService string `json:"previewService,omitempty"`
}

// CanaryStrategy defines canary deployment parameters
type CanaryStrategy struct {
	// Steps define the canary deployment steps
	// +optional
	Steps []CanaryStep `json:"steps,omitempty"`

	// Analysis defines analysis templates for canary validation
	// +optional
	Analysis *AnalysisTemplate `json:"analysis,omitempty"`

	// TrafficSplitting defines traffic splitting configuration
	// +optional
	TrafficSplitting *TrafficSplitting `json:"trafficSplitting,omitempty"`
}

// CanaryStep defines a step in canary deployment
type CanaryStep struct {
	// Weight specifies the percentage of traffic to route to canary
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	Weight *int32 `json:"weight,omitempty"`

	// Duration specifies how long to wait before next step
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// Pause indicates whether to pause at this step
	// +optional
	Pause bool `json:"pause,omitempty"`
}

// AnalysisTemplate defines analysis configuration
type AnalysisTemplate struct {
	// Name of the analysis template
	Name string `json:"name"`

	// Args specify analysis arguments
	// +optional
	Args []AnalysisArg `json:"args,omitempty"`
}

// AnalysisArg defines an analysis argument
type AnalysisArg struct {
	// Name of the argument
	Name string `json:"name"`

	// Value of the argument
	Value string `json:"value"`
}

// TrafficSplitting defines traffic splitting configuration
type TrafficSplitting struct {
	// SMI indicates whether to use SMI for traffic splitting
	// +optional
	SMI *SMITrafficSplitting `json:"smi,omitempty"`

	// Istio indicates whether to use Istio for traffic splitting
	// +optional
	Istio *IstioTrafficSplitting `json:"istio,omitempty"`

	// Nginx indicates whether to use NGINX for traffic splitting
	// +optional
	Nginx *NginxTrafficSplitting `json:"nginx,omitempty"`
}

// SMITrafficSplitting defines SMI-based traffic splitting
type SMITrafficSplitting struct {
	// Enabled indicates whether SMI traffic splitting is enabled
	Enabled bool `json:"enabled"`

	// RootService specifies the root service name
	// +optional
	RootService string `json:"rootService,omitempty"`
}

// IstioTrafficSplitting defines Istio-based traffic splitting
type IstioTrafficSplitting struct {
	// Enabled indicates whether Istio traffic splitting is enabled
	Enabled bool `json:"enabled"`

	// VirtualService specifies the virtual service configuration
	// +optional
	VirtualService *runtime.RawExtension `json:"virtualService,omitempty"`

	// DestinationRule specifies the destination rule configuration
	// +optional
	DestinationRule *runtime.RawExtension `json:"destinationRule,omitempty"`
}

// NginxTrafficSplitting defines NGINX-based traffic splitting
type NginxTrafficSplitting struct {
	// Enabled indicates whether NGINX traffic splitting is enabled
	Enabled bool `json:"enabled"`

	// IngressClass specifies the NGINX ingress class
	// +optional
	IngressClass string `json:"ingressClass,omitempty"`

	// AnnotationPrefix specifies the annotation prefix for NGINX
	// +optional
	AnnotationPrefix string `json:"annotationPrefix,omitempty"`
}

// WorkloadPlacementStatus represents the observed state of the WorkloadPlacement
type WorkloadPlacementStatus struct {
	// Conditions represent the latest available observations of the placement's current state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase represents the current phase of workload placement
	// +optional
	Phase WorkloadPlacementPhase `json:"phase,omitempty"`

	// PlacementDecisions lists the placement decisions made
	// +optional
	PlacementDecisions []PlacementDecision `json:"placementDecisions,omitempty"`

	// SelectedClusters lists the clusters selected for placement
	// +optional
	SelectedClusters []SelectedCluster `json:"selectedClusters,omitempty"`

	// ReplicaStatus provides replica status across clusters
	// +optional
	ReplicaStatus *ReplicaStatus `json:"replicaStatus,omitempty"`

	// LastScheduleTime is the last time placement was scheduled
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// WorkloadPlacementPhase defines the phase of workload placement
// +kubebuilder:validation:Enum=Pending;Scheduling;Scheduled;Failed;Updating
type WorkloadPlacementPhase string

const (
	// WorkloadPlacementPhasePending means the placement is pending
	WorkloadPlacementPhasePending WorkloadPlacementPhase = "Pending"

	// WorkloadPlacementPhaseScheduling means the placement is being scheduled
	WorkloadPlacementPhaseScheduling WorkloadPlacementPhase = "Scheduling"

	// WorkloadPlacementPhaseScheduled means the placement has been scheduled
	WorkloadPlacementPhaseScheduled WorkloadPlacementPhase = "Scheduled"

	// WorkloadPlacementPhaseFailed means the placement failed
	WorkloadPlacementPhaseFailed WorkloadPlacementPhase = "Failed"

	// WorkloadPlacementPhaseUpdating means the placement is being updated
	WorkloadPlacementPhaseUpdating WorkloadPlacementPhase = "Updating"
)

// PlacementDecision represents a placement decision
type PlacementDecision struct {
	// Cluster is the name of the selected cluster
	Cluster string `json:"cluster"`

	// Score is the score assigned to this cluster
	// +optional
	Score int32 `json:"score,omitempty"`

	// Reason explains why this cluster was selected
	// +optional
	Reason string `json:"reason,omitempty"`

	// Weight is the weight assigned to this cluster for replica distribution
	// +optional
	Weight int32 `json:"weight,omitempty"`
}

// SelectedCluster represents a cluster selected for placement
type SelectedCluster struct {
	// Name is the cluster name
	Name string `json:"name"`

	// Replicas is the number of replicas scheduled to this cluster
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Status indicates the status of placement on this cluster
	// +optional
	Status ClusterPlacementStatus `json:"status,omitempty"`
}

// ClusterPlacementStatus represents the status of placement on a cluster
// +kubebuilder:validation:Enum=Pending;Scheduled;Running;Failed;Unknown
type ClusterPlacementStatus string

const (
	// ClusterPlacementStatusPending means placement is pending
	ClusterPlacementStatusPending ClusterPlacementStatus = "Pending"

	// ClusterPlacementStatusScheduled means placement is scheduled
	ClusterPlacementStatusScheduled ClusterPlacementStatus = "Scheduled"

	// ClusterPlacementStatusRunning means placement is running
	ClusterPlacementStatusRunning ClusterPlacementStatus = "Running"

	// ClusterPlacementStatusFailed means placement failed
	ClusterPlacementStatusFailed ClusterPlacementStatus = "Failed"

	// ClusterPlacementStatusUnknown means placement status is unknown
	ClusterPlacementStatusUnknown ClusterPlacementStatus = "Unknown"
)

// ReplicaStatus provides replica status across clusters
type ReplicaStatus struct {
	// Total is the total number of replicas
	// +optional
	Total int32 `json:"total,omitempty"`

	// Ready is the number of ready replicas
	// +optional
	Ready int32 `json:"ready,omitempty"`

	// Available is the number of available replicas
	// +optional
	Available int32 `json:"available,omitempty"`

	// Unavailable is the number of unavailable replicas
	// +optional
	Unavailable int32 `json:"unavailable,omitempty"`

	// UpdatedReplicas is the number of updated replicas
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
}

// Condition types for TMC resources

// ClusterRegistration condition types
const (
	// ClusterRegistrationReady means the cluster is registered and ready
	ClusterRegistrationReady conditionsv1alpha1.ConditionType = "Ready"

	// ClusterRegistrationConnected means the cluster is connected
	ClusterRegistrationConnected conditionsv1alpha1.ConditionType = "Connected"

	// ClusterRegistrationHealthy means the cluster is healthy
	ClusterRegistrationHealthy conditionsv1alpha1.ConditionType = "Healthy"

	// ClusterRegistrationCapacityReported means cluster capacity is reported
	ClusterRegistrationCapacityReported conditionsv1alpha1.ConditionType = "CapacityReported"
)

// WorkloadPlacement condition types
const (
	// WorkloadPlacementReady means the placement policy is ready
	WorkloadPlacementReady conditionsv1alpha1.ConditionType = "Ready"

	// WorkloadPlacementScheduled means clusters have been selected
	WorkloadPlacementScheduled conditionsv1alpha1.ConditionType = "Scheduled"

	// WorkloadPlacementDeployed means workloads have been deployed
	WorkloadPlacementDeployed conditionsv1alpha1.ConditionType = "Deployed"

	// WorkloadPlacementProgressing means placement is progressing
	WorkloadPlacementProgressing conditionsv1alpha1.ConditionType = "Progressing"
)

// Condition implementations for TMC resources
func (cr *ClusterRegistration) GetConditions() conditionsv1alpha1.Conditions {
	return cr.Status.Conditions
}

func (cr *ClusterRegistration) SetConditions(conditions conditionsv1alpha1.Conditions) {
	cr.Status.Conditions = conditions
}

func (wp *WorkloadPlacement) GetConditions() conditionsv1alpha1.Conditions {
	return wp.Status.Conditions
}

func (wp *WorkloadPlacement) SetConditions(conditions conditionsv1alpha1.Conditions) {
	wp.Status.Conditions = conditions
}

// List types for TMC resources

// ClusterRegistrationList contains a list of ClusterRegistration
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of ClusterRegistration objects
	Items []ClusterRegistration `json:"items"`
}

// WorkloadPlacementList contains a list of WorkloadPlacement
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadPlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of WorkloadPlacement objects
	Items []WorkloadPlacement `json:"items"`
}