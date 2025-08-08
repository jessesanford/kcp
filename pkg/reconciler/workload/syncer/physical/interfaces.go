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

package physical

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	
	tmcv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1"
)

// WorkloadSyncer defines the interface for synchronizing workloads to physical clusters
type WorkloadSyncer interface {
	// SyncWorkload synchronizes a workload resource to a physical cluster
	// based on placement decisions
	SyncWorkload(ctx context.Context,
		cluster *tmcv1alpha1.ClusterRegistration,
		workload runtime.Object,
	) error
	
	// GetStatus retrieves the current status of a workload from the physical cluster
	GetStatus(ctx context.Context,
		cluster *tmcv1alpha1.ClusterRegistration,
		workload runtime.Object,
	) (*WorkloadStatus, error)
	
	// DeleteWorkload removes a workload from the physical cluster
	DeleteWorkload(ctx context.Context,
		cluster *tmcv1alpha1.ClusterRegistration,
		workload runtime.Object,
	) error
	
	// HealthCheck verifies the syncer can communicate with the physical cluster
	HealthCheck(ctx context.Context,
		cluster *tmcv1alpha1.ClusterRegistration,
	) error
}

// WorkloadStatus represents the status of a workload in a physical cluster
type WorkloadStatus struct {
	// Ready indicates if the workload is ready in the cluster
	Ready bool
	
	// Phase represents the current lifecycle phase
	Phase WorkloadPhase
	
	// Conditions contains detailed status conditions
	Conditions []WorkloadCondition
	
	// Resources contains status of individual resources that make up the workload
	Resources []ResourceStatus
	
	// LastUpdated is when this status was last refreshed
	LastUpdated time.Time
	
	// ClusterName identifies which cluster this status is from
	ClusterName string
}

// WorkloadPhase represents the lifecycle phase of a workload
type WorkloadPhase string

const (
	WorkloadPhasePending    WorkloadPhase = "Pending"
	WorkloadPhaseDeploying  WorkloadPhase = "Deploying"
	WorkloadPhaseReady      WorkloadPhase = "Ready"
	WorkloadPhaseDegraded   WorkloadPhase = "Degraded"
	WorkloadPhaseFailed     WorkloadPhase = "Failed"
	WorkloadPhaseTerminating WorkloadPhase = "Terminating"
)

// WorkloadCondition represents a condition of a workload
type WorkloadCondition struct {
	Type               string
	Status             ConditionStatus
	LastTransitionTime time.Time
	Reason             string
	Message            string
}

// ConditionStatus represents the status of a condition
type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

// ResourceStatus represents the status of an individual resource
type ResourceStatus struct {
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
	Ready     bool
	Phase     string
	Message   string
}

// PlacementSyncer orchestrates syncing workloads according to placement decisions
type PlacementSyncer interface {
	// ProcessPlacement handles a WorkloadPlacement by syncing workloads to selected clusters
	ProcessPlacement(ctx context.Context,
		placement *tmcv1alpha1.WorkloadPlacement,
	) error
	
	// GetPlacementStatus aggregates status from all clusters for a placement
	GetPlacementStatus(ctx context.Context,
		placement *tmcv1alpha1.WorkloadPlacement,
	) (*PlacementStatus, error)
	
	// CleanupPlacement removes workloads from clusters when placement is deleted
	CleanupPlacement(ctx context.Context,
		placement *tmcv1alpha1.WorkloadPlacement,
	) error
}

// PlacementStatus represents aggregated status across multiple clusters
type PlacementStatus struct {
	// TotalClusters is the number of clusters in the placement
	TotalClusters int
	
	// ReadyClusters is the number of clusters where workload is ready
	ReadyClusters int
	
	// FailedClusters is the number of clusters where workload failed
	FailedClusters int
	
	// ClusterStatuses contains per-cluster status details
	ClusterStatuses map[string]*WorkloadStatus
	
	// OverallPhase represents the aggregated status
	OverallPhase WorkloadPhase
	
	// LastUpdated is when this status was last computed
	LastUpdated time.Time
}

// SyncerFactory creates WorkloadSyncers for specific clusters
type SyncerFactory interface {
	// CreateSyncer creates a WorkloadSyncer for the specified cluster
	CreateSyncer(ctx context.Context,
		cluster *tmcv1alpha1.ClusterRegistration,
	) (WorkloadSyncer, error)
	
	// GetSyncer retrieves an existing syncer for a cluster
	GetSyncer(clusterName string) (WorkloadSyncer, bool)
	
	// RemoveSyncer removes and cleans up a syncer for a cluster
	RemoveSyncer(clusterName string) error
}

// SyncEvent represents an event during synchronization
type SyncEvent struct {
	Type        SyncEventType
	Cluster     string
	Workload    WorkloadRef
	Timestamp   time.Time
	Message     string
	Error       error
}

// SyncEventType represents the type of sync event
type SyncEventType string

const (
	SyncEventStarted   SyncEventType = "SyncStarted"
	SyncEventCompleted SyncEventType = "SyncCompleted"
	SyncEventFailed    SyncEventType = "SyncFailed"
	SyncEventSkipped   SyncEventType = "SyncSkipped"
)

// WorkloadRef identifies a workload resource
type WorkloadRef struct {
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
}

// SyncEventHandler processes sync events
type SyncEventHandler interface {
	// HandleEvent processes a sync event
	HandleEvent(ctx context.Context, event *SyncEvent) error
}