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
	"k8s.io/apimachinery/pkg/api/resource"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// SessionState represents the persistent state of a placement session.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type SessionState struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec SessionStateSpec `json:"spec,omitempty"`
	// +optional
	Status SessionStateStatus `json:"status,omitempty"`
}

// SessionStateList contains a list of SessionState
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SessionStateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SessionState `json:"items"`
}

// SessionStateSpec defines the desired state of SessionState
type SessionStateSpec struct {
	// SessionRef references the placement session this state belongs to
	SessionRef SessionReference `json:"sessionRef"`

	// StateData contains the serialized session state data
	StateData SessionStateData `json:"stateData"`

	// SyncPolicy defines how state should be synchronized across clusters
	// +optional
	SyncPolicy *StateSyncPolicy `json:"syncPolicy,omitempty"`

	// RetentionPolicy defines how long to retain session state
	// +optional
	RetentionPolicy *StateRetentionPolicy `json:"retentionPolicy,omitempty"`
}

// SessionStateStatus defines the observed state of SessionState
type SessionStateStatus struct {
	// Conditions represent the current conditions of the SessionState
	// +optional
	Conditions []conditionsv1alpha1.Condition `json:"conditions,omitempty"`

	// SyncStatus contains the status of state synchronization
	// +optional
	SyncStatus *StateSyncStatus `json:"syncStatus,omitempty"`

	// LastHeartbeat is the timestamp of the last session heartbeat
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// StateVersion is the version of the session state
	// +optional
	StateVersion int64 `json:"stateVersion,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// SessionStateData contains the serialized session state
type SessionStateData struct {
	// CurrentPhase is the current phase of the session
	CurrentPhase SessionPhase `json:"currentPhase"`

	// PhaseTransitions contains the history of phase transitions
	// +optional
	PhaseTransitions []PhaseTransition `json:"phaseTransitions,omitempty"`

	// PlacementContext contains the context for placement decisions
	// +optional
	PlacementContext *PlacementContext `json:"placementContext,omitempty"`

	// ResourceAllocations contains current resource allocations
	// +optional
	ResourceAllocations []ResourceAllocation `json:"resourceAllocations,omitempty"`

	// ConflictHistory contains the history of conflicts and resolutions
	// +optional
	ConflictHistory []ConflictRecord `json:"conflictHistory,omitempty"`

	// SessionEvents contains significant session events
	// +optional
	SessionEvents []SessionEvent `json:"sessionEvents,omitempty"`

	// Checkpoints contains state checkpoints for recovery
	// +optional
	Checkpoints []StateCheckpoint `json:"checkpoints,omitempty"`
}

// PhaseTransition represents a transition between session phases
type PhaseTransition struct {
	// FromPhase is the phase being transitioned from
	FromPhase SessionPhase `json:"fromPhase"`

	// ToPhase is the phase being transitioned to
	ToPhase SessionPhase `json:"toPhase"`

	// Timestamp is when the transition occurred
	Timestamp metav1.Time `json:"timestamp"`

	// Reason is the reason for the phase transition
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message provides additional details about the transition
	// +optional
	Message string `json:"message,omitempty"`

	// Initiator identifies what initiated the transition
	// +optional
	Initiator string `json:"initiator,omitempty"`
}

// PlacementContext contains context information for placement decisions
type PlacementContext struct {
	// AvailableClusters contains information about available clusters
	// +optional
	AvailableClusters []ClusterInfo `json:"availableClusters,omitempty"`

	// WorkloadRequirements contains requirements for workloads in the session
	// +optional
	WorkloadRequirements []WorkloadRequirement `json:"workloadRequirements,omitempty"`

	// PlacementHistory contains the history of placement decisions
	// +optional
	PlacementHistory []PlacementHistoryEntry `json:"placementHistory,omitempty"`

	// ConstraintEvaluations contains evaluations of placement constraints
	// +optional
	ConstraintEvaluations []ConstraintEvaluation `json:"constraintEvaluations,omitempty"`
}

// ClusterInfo contains information about a cluster in the placement context
type ClusterInfo struct {
	// Name is the name of the cluster
	Name string `json:"name"`

	// Location contains location information for the cluster
	// +optional
	Location *ClusterLocation `json:"location,omitempty"`

	// Capabilities contains the capabilities of the cluster
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`

	// ResourceCapacity contains the resource capacity of the cluster
	// +optional
	ResourceCapacity map[string]resource.Quantity `json:"resourceCapacity,omitempty"`

	// Labels contains the labels associated with the cluster
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Status is the current status of the cluster
	// +optional
	Status ClusterStatus `json:"status,omitempty"`
}

// ClusterLocation contains location information for a cluster
type ClusterLocation struct {
	// Region is the region where the cluster is located
	// +optional
	Region string `json:"region,omitempty"`

	// Zone is the availability zone where the cluster is located
	// +optional
	Zone string `json:"zone,omitempty"`

	// Provider is the cloud provider hosting the cluster
	// +optional
	Provider string `json:"provider,omitempty"`
}

// ClusterStatus defines the status of a cluster
// +kubebuilder:validation:Enum=Ready;NotReady;Unknown;Maintenance
type ClusterStatus string

const (
	// ClusterStatusReady indicates the cluster is ready for workloads
	ClusterStatusReady ClusterStatus = "Ready"
	// ClusterStatusNotReady indicates the cluster is not ready for workloads
	ClusterStatusNotReady ClusterStatus = "NotReady"
	// ClusterStatusUnknown indicates the cluster status is unknown
	ClusterStatusUnknown ClusterStatus = "Unknown"
	// ClusterStatusMaintenance indicates the cluster is under maintenance
	ClusterStatusMaintenance ClusterStatus = "Maintenance"
)

// WorkloadRequirement contains requirements for a workload
type WorkloadRequirement struct {
	// WorkloadRef references the workload
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// ResourceRequirements defines the resource requirements
	// +optional
	ResourceRequirements map[string]resource.Quantity `json:"resourceRequirements,omitempty"`

	// PlacementConstraints defines placement constraints for the workload
	// +optional
	PlacementConstraints []PlacementConstraint `json:"placementConstraints,omitempty"`

	// Priority defines the priority of this workload
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=500
	// +optional
	Priority int32 `json:"priority,omitempty"`
}

// PlacementHistoryEntry represents an entry in the placement history
type PlacementHistoryEntry struct {
	// Timestamp is when the placement was made
	Timestamp metav1.Time `json:"timestamp"`

	// WorkloadRef references the workload that was placed
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// ClusterName is the name of the cluster where the workload was placed
	ClusterName string `json:"clusterName"`

	// PlacementReason is the reason for the placement decision
	// +optional
	PlacementReason string `json:"placementReason,omitempty"`

	// PlacementScore is the score assigned to this placement
	// +optional
	PlacementScore *float64 `json:"placementScore,omitempty"`

	// Status is the current status of this placement
	// +optional
	Status PlacementStatus `json:"status,omitempty"`
}

// PlacementStatus defines the status of a placement
// +kubebuilder:validation:Enum=Pending;Active;Completed;Failed;Cancelled
type PlacementStatus string

const (
	// PlacementStatusPending indicates the placement is pending
	PlacementStatusPending PlacementStatus = "Pending"
	// PlacementStatusActive indicates the placement is active
	PlacementStatusActive PlacementStatus = "Active"
	// PlacementStatusCompleted indicates the placement has completed
	PlacementStatusCompleted PlacementStatus = "Completed"
	// PlacementStatusFailed indicates the placement has failed
	PlacementStatusFailed PlacementStatus = "Failed"
	// PlacementStatusCancelled indicates the placement was cancelled
	PlacementStatusCancelled PlacementStatus = "Cancelled"
)

// ResourceAllocation represents a resource allocation within a session
type ResourceAllocation struct {
	// ClusterName is the name of the cluster
	ClusterName string `json:"clusterName"`

	// AllocatedResources contains the allocated resources
	AllocatedResources map[string]resource.Quantity `json:"allocatedResources"`

	// WorkloadAllocations contains allocations for specific workloads
	// +optional
	WorkloadAllocations []WorkloadAllocation `json:"workloadAllocations,omitempty"`

	// AllocationTime is when the resources were allocated
	AllocationTime metav1.Time `json:"allocationTime"`

	// Status is the current status of the resource allocation
	// +optional
	Status AllocationStatus `json:"status,omitempty"`
}

// WorkloadAllocation represents resource allocation for a specific workload
type WorkloadAllocation struct {
	// WorkloadRef references the workload
	WorkloadRef WorkloadReference `json:"workloadRef"`

	// AllocatedResources contains the resources allocated to the workload
	AllocatedResources map[string]resource.Quantity `json:"allocatedResources"`
}

// AllocationStatus defines the status of a resource allocation
// +kubebuilder:validation:Enum=Allocated;Pending;Failed;Released
type AllocationStatus string

const (
	// AllocationStatusAllocated indicates resources are allocated
	AllocationStatusAllocated AllocationStatus = "Allocated"
	// AllocationStatusPending indicates allocation is pending
	AllocationStatusPending AllocationStatus = "Pending"
	// AllocationStatusFailed indicates allocation has failed
	AllocationStatusFailed AllocationStatus = "Failed"
	// AllocationStatusReleased indicates resources have been released
	AllocationStatusReleased AllocationStatus = "Released"
)

// ConflictRecord represents a record of a placement conflict and its resolution
type ConflictRecord struct {
	// ConflictID is the unique identifier for the conflict
	ConflictID string `json:"conflictID"`

	// Timestamp is when the conflict occurred
	Timestamp metav1.Time `json:"timestamp"`

	// ConflictType describes the type of conflict
	ConflictType ConflictType `json:"conflictType"`

	// ConflictingPlacements contains the placements that were in conflict
	ConflictingPlacements []PlacementReference `json:"conflictingPlacements"`

	// Resolution describes how the conflict was resolved
	// +optional
	Resolution *ConflictResolution `json:"resolution,omitempty"`

	// ResolutionTime is when the conflict was resolved
	// +optional
	ResolutionTime *metav1.Time `json:"resolutionTime,omitempty"`
}

// ConflictResolution describes how a conflict was resolved
type ConflictResolution struct {
	// Strategy describes the resolution strategy used
	Strategy ConflictResolutionType `json:"strategy"`

	// WinningPlacement identifies the placement that was chosen
	// +optional
	WinningPlacement *PlacementReference `json:"winningPlacement,omitempty"`

	// RejectedPlacements contains the placements that were rejected
	// +optional
	RejectedPlacements []PlacementReference `json:"rejectedPlacements,omitempty"`

	// Reason describes why this resolution was chosen
	// +optional
	Reason string `json:"reason,omitempty"`
}

// SessionEvent represents a significant event in a session
type SessionEvent struct {
	// EventID is the unique identifier for the event
	EventID string `json:"eventID"`

	// Timestamp is when the event occurred
	Timestamp metav1.Time `json:"timestamp"`

	// EventType describes the type of event
	EventType SessionEventType `json:"eventType"`

	// Message contains a human-readable description of the event
	Message string `json:"message"`

	// Source identifies the source of the event
	// +optional
	Source string `json:"source,omitempty"`

	// Reason provides a structured reason for the event
	// +optional
	Reason string `json:"reason,omitempty"`

	// RelatedObjects contains references to related objects
	// +optional
	RelatedObjects []ObjectReference `json:"relatedObjects,omitempty"`
}

// SessionEventType defines the types of session events
// +kubebuilder:validation:Enum=Created;Started;Suspended;Resumed;Completed;Failed;HeartbeatMissed;ConflictDetected;ConflictResolved
type SessionEventType string

const (
	// SessionEventTypeCreated indicates the session was created
	SessionEventTypeCreated SessionEventType = "Created"
	// SessionEventTypeStarted indicates the session was started
	SessionEventTypeStarted SessionEventType = "Started"
	// SessionEventTypeSuspended indicates the session was suspended
	SessionEventTypeSuspended SessionEventType = "Suspended"
	// SessionEventTypeResumed indicates the session was resumed
	SessionEventTypeResumed SessionEventType = "Resumed"
	// SessionEventTypeCompleted indicates the session was completed
	SessionEventTypeCompleted SessionEventType = "Completed"
	// SessionEventTypeFailed indicates the session failed
	SessionEventTypeFailed SessionEventType = "Failed"
	// SessionEventTypeHeartbeatMissed indicates a heartbeat was missed
	SessionEventTypeHeartbeatMissed SessionEventType = "HeartbeatMissed"
	// SessionEventTypeConflictDetected indicates a conflict was detected
	SessionEventTypeConflictDetected SessionEventType = "ConflictDetected"
	// SessionEventTypeConflictResolved indicates a conflict was resolved
	SessionEventTypeConflictResolved SessionEventType = "ConflictResolved"
)

// StateCheckpoint represents a checkpoint of session state for recovery
type StateCheckpoint struct {
	// CheckpointID is the unique identifier for the checkpoint
	CheckpointID string `json:"checkpointID"`

	// Timestamp is when the checkpoint was created
	Timestamp metav1.Time `json:"timestamp"`

	// Phase is the session phase when the checkpoint was created
	Phase SessionPhase `json:"phase"`

	// Data contains the serialized checkpoint data
	Data []byte `json:"data"`

	// Version is the version of the checkpoint format
	// +optional
	Version string `json:"version,omitempty"`

	// Checksum is the checksum of the checkpoint data
	// +optional
	Checksum string `json:"checksum,omitempty"`
}

// StateSyncPolicy defines how session state should be synchronized
type StateSyncPolicy struct {
	// SyncMode defines the synchronization mode
	// +kubebuilder:validation:Enum=Immediate;Batch;Periodic
	// +kubebuilder:default="Immediate"
	// +optional
	SyncMode StateSyncMode `json:"syncMode,omitempty"`

	// SyncInterval defines the interval for periodic synchronization
	// +kubebuilder:default="1m"
	// +optional
	SyncInterval metav1.Duration `json:"syncInterval,omitempty"`

	// ConflictResolution defines how to handle sync conflicts
	// +kubebuilder:validation:Enum=LastWrite;Timestamp;Manual
	// +kubebuilder:default="Timestamp"
	// +optional
	ConflictResolution StateSyncConflictResolution `json:"conflictResolution,omitempty"`

	// Replicas defines how many replicas of the state to maintain
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	// +optional
	Replicas int32 `json:"replicas,omitempty"`
}

// StateSyncMode defines synchronization modes
// +kubebuilder:validation:Enum=Immediate;Batch;Periodic
type StateSyncMode string

const (
	// StateSyncModeImmediate synchronizes state immediately
	StateSyncModeImmediate StateSyncMode = "Immediate"
	// StateSyncModeBatch synchronizes state in batches
	StateSyncModeBatch StateSyncMode = "Batch"
	// StateSyncModePeriodic synchronizes state periodically
	StateSyncModePeriodic StateSyncMode = "Periodic"
)

// StateSyncConflictResolution defines how to handle sync conflicts
// +kubebuilder:validation:Enum=LastWrite;Timestamp;Manual
type StateSyncConflictResolution string

const (
	// StateSyncConflictResolutionLastWrite uses the last write wins strategy
	StateSyncConflictResolutionLastWrite StateSyncConflictResolution = "LastWrite"
	// StateSyncConflictResolutionTimestamp uses timestamp-based resolution
	StateSyncConflictResolutionTimestamp StateSyncConflictResolution = "Timestamp"
	// StateSyncConflictResolutionManual requires manual conflict resolution
	StateSyncConflictResolutionManual StateSyncConflictResolution = "Manual"
)

// StateRetentionPolicy defines how long to retain session state
type StateRetentionPolicy struct {
	// RetentionPeriod defines how long to retain completed session state
	// +kubebuilder:default="24h"
	// +optional
	RetentionPeriod metav1.Duration `json:"retentionPeriod,omitempty"`

	// FailedSessionRetention defines how long to retain failed session state
	// +kubebuilder:default="7d"
	// +optional
	FailedSessionRetention metav1.Duration `json:"failedSessionRetention,omitempty"`

	// CheckpointRetention defines how long to retain checkpoints
	// +kubebuilder:default="30d"
	// +optional
	CheckpointRetention metav1.Duration `json:"checkpointRetention,omitempty"`

	// MaxCheckpoints defines the maximum number of checkpoints to retain
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxCheckpoints int32 `json:"maxCheckpoints,omitempty"`
}

// StateSyncStatus contains the status of state synchronization
type StateSyncStatus struct {
	// LastSyncTime is when the state was last synchronized
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// SyncVersion is the version of the last synchronized state
	// +optional
	SyncVersion int64 `json:"syncVersion,omitempty"`

	// ReplicaStates contains the status of state replicas
	// +optional
	ReplicaStates []ReplicaState `json:"replicaStates,omitempty"`

	// SyncConflicts contains information about sync conflicts
	// +optional
	SyncConflicts []SyncConflict `json:"syncConflicts,omitempty"`
}

// ReplicaState contains the status of a state replica
type ReplicaState struct {
	// ClusterName is the name of the cluster hosting this replica
	ClusterName string `json:"clusterName"`

	// Version is the version of the state in this replica
	Version int64 `json:"version"`

	// LastUpdateTime is when this replica was last updated
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`

	// Status is the current status of this replica
	Status ReplicaStatus `json:"status"`
}

// ReplicaStatus defines the status of a state replica
// +kubebuilder:validation:Enum=InSync;OutOfSync;Failed;Recovering
type ReplicaStatus string

const (
	// ReplicaStatusInSync indicates the replica is in sync
	ReplicaStatusInSync ReplicaStatus = "InSync"
	// ReplicaStatusOutOfSync indicates the replica is out of sync
	ReplicaStatusOutOfSync ReplicaStatus = "OutOfSync"
	// ReplicaStatusFailed indicates the replica has failed
	ReplicaStatusFailed ReplicaStatus = "Failed"
	// ReplicaStatusRecovering indicates the replica is recovering
	ReplicaStatusRecovering ReplicaStatus = "Recovering"
)

// SyncConflict represents a synchronization conflict
type SyncConflict struct {
	// ConflictID is the unique identifier for the conflict
	ConflictID string `json:"conflictID"`

	// DetectedTime is when the conflict was detected
	DetectedTime metav1.Time `json:"detectedTime"`

	// ConflictingVersions contains the conflicting state versions
	ConflictingVersions []ConflictingVersion `json:"conflictingVersions"`

	// Resolution describes how the conflict was resolved
	// +optional
	Resolution *SyncConflictResolution `json:"resolution,omitempty"`
}

// ConflictingVersion represents a conflicting state version
type ConflictingVersion struct {
	// ClusterName is the name of the cluster with this version
	ClusterName string `json:"clusterName"`

	// Version is the version number
	Version int64 `json:"version"`

	// Timestamp is when this version was created
	Timestamp metav1.Time `json:"timestamp"`
}

// SyncConflictResolution describes how a sync conflict was resolved
type SyncConflictResolution struct {
	// Strategy is the resolution strategy used
	Strategy StateSyncConflictResolution `json:"strategy"`

	// WinningVersion is the version that was chosen
	WinningVersion int64 `json:"winningVersion"`

	// WinningCluster is the cluster that had the winning version
	WinningCluster string `json:"winningCluster"`

	// ResolvedTime is when the conflict was resolved
	ResolvedTime metav1.Time `json:"resolvedTime"`
}

// ConstraintEvaluation represents the evaluation of a placement constraint
type ConstraintEvaluation struct {
	// ConstraintName is the name of the constraint
	ConstraintName string `json:"constraintName"`

	// ConstraintType is the type of the constraint
	ConstraintType PlacementConstraintType `json:"constraintType"`

	// EvaluationTime is when the constraint was evaluated
	EvaluationTime metav1.Time `json:"evaluationTime"`

	// Result is the result of the constraint evaluation
	Result ConstraintEvaluationResult `json:"result"`

	// Score is the score assigned to this constraint evaluation (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	Score int32 `json:"score,omitempty"`

	// Details provides additional details about the evaluation
	// +optional
	Details string `json:"details,omitempty"`
}

// ConstraintEvaluationResult defines the result of constraint evaluation
// +kubebuilder:validation:Enum=Satisfied;Violated;PartiallyMet;Unknown
type ConstraintEvaluationResult string

const (
	// ConstraintEvaluationResultSatisfied indicates the constraint was satisfied
	ConstraintEvaluationResultSatisfied ConstraintEvaluationResult = "Satisfied"
	// ConstraintEvaluationResultViolated indicates the constraint was violated
	ConstraintEvaluationResultViolated ConstraintEvaluationResult = "Violated"
	// ConstraintEvaluationResultPartiallyMet indicates the constraint was partially met
	ConstraintEvaluationResultPartiallyMet ConstraintEvaluationResult = "PartiallyMet"
	// ConstraintEvaluationResultUnknown indicates the constraint evaluation result is unknown
	ConstraintEvaluationResultUnknown ConstraintEvaluationResult = "Unknown"
)