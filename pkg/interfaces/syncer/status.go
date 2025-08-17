package syncer

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/logicalcluster"
)

// StatusReporter reports sync status
type StatusReporter interface {
	// ReportResourceStatus reports status for a resource
	ReportResourceStatus(
		ctx context.Context,
		workspace logicalcluster.Name,
		obj *unstructured.Unstructured,
		status SyncStatus,
	) error

	// ReportSyncTargetStatus updates SyncTarget status
	ReportSyncTargetStatus(
		ctx context.Context,
		target *workloadv1alpha1.SyncTarget,
		status TargetStatus,
	) error

	// GetAggregatedStatus returns overall status
	GetAggregatedStatus(ctx context.Context) (*AggregatedStatus, error)
}

// SyncStatus represents resource sync status
type SyncStatus struct {
	// Phase of synchronization
	Phase SyncPhase

	// Generation that was synced
	ObservedGeneration int64

	// Conditions
	Conditions conditionsv1alpha1.Conditions

	// Message
	Message string

	// LastSyncTime
	LastSyncTime *metav1.Time
}

// SyncPhase represents the sync phase of a resource
type SyncPhase string

const (
	SyncPhasePending  SyncPhase = "Pending"
	SyncPhaseSyncing  SyncPhase = "Syncing"
	SyncPhaseSynced   SyncPhase = "Synced"
	SyncPhaseFailed   SyncPhase = "Failed"
	SyncPhaseDeleting SyncPhase = "Deleting"
)

// TargetStatus represents SyncTarget status
type TargetStatus struct {
	// Allocatable resources
	Allocatable workloadv1alpha1.ResourceList

	// Capacity resources
	Capacity workloadv1alpha1.ResourceList

	// Conditions
	Conditions conditionsv1alpha1.Conditions

	// VirtualWorkspaces
	VirtualWorkspaces []workloadv1alpha1.VirtualWorkspace
}

// AggregatedStatus combines all status information
type AggregatedStatus struct {
	// TotalResources being synced
	TotalResources int

	// SyncedResources count
	SyncedResources int

	// FailedResources count
	FailedResources int

	// HealthPercentage
	HealthPercentage float64

	// Conditions
	Conditions conditionsv1alpha1.Conditions
}

// StatusAggregator aggregates status from multiple sources
type StatusAggregator interface {
	// AddStatusSource adds a status source
	AddStatusSource(name string, source StatusReporter) error

	// RemoveStatusSource removes a source
	RemoveStatusSource(name string) error

	// Aggregate computes aggregated status
	Aggregate(ctx context.Context) (*AggregatedStatus, error)
}