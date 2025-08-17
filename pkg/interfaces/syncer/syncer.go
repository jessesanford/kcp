package syncer

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/logicalcluster"
)

// Syncer is the main interface for workload synchronization
type Syncer interface {
	// Start begins the syncer operation
	Start(ctx context.Context) error

	// Stop gracefully stops the syncer
	Stop() error

	// GetSyncTarget returns the target this syncer manages
	GetSyncTarget() *workloadv1alpha1.SyncTarget

	// GetCapabilities returns syncer capabilities
	GetCapabilities() Capabilities

	// RegisterResource registers a resource for syncing
	RegisterResource(gvr schema.GroupVersionResource) error

	// UnregisterResource stops syncing a resource
	UnregisterResource(gvr schema.GroupVersionResource) error

	// GetStatus returns current syncer status
	GetStatus() Status
}

// Capabilities describes what a syncer can do
type Capabilities struct {
	// SupportedResources that can be synced
	SupportedResources []schema.GroupVersionResource

	// Features enabled in this syncer
	Features []Feature

	// MaxConcurrentSyncs allowed
	MaxConcurrentSyncs int

	// SupportsTransformation if syncer can transform resources
	SupportsTransformation bool

	// SupportsStatusAggregation if syncer aggregates status
	SupportsStatusAggregation bool

	// SupportsBidirectionalSync for two-way sync
	SupportsBidirectionalSync bool
}

// Feature represents a syncer feature
type Feature struct {
	// Name of the feature
	Name string

	// Version of the feature
	Version string

	// Enabled status
	Enabled bool

	// Configuration for the feature
	Configuration map[string]interface{}
}

// Status represents syncer status
type Status struct {
	// Phase of the syncer
	Phase SyncerPhase

	// Message about current state
	Message string

	// SyncedResources count
	SyncedResources int

	// PendingResources count
	PendingResources int

	// FailedResources count
	FailedResources int

	// LastSyncTime
	LastSyncTime *metav1.Time

	// Conditions
	Conditions []metav1.Condition
}

// SyncerPhase represents the syncer lifecycle phase
type SyncerPhase string

const (
	SyncerPhasePending      SyncerPhase = "Pending"
	SyncerPhaseInitializing SyncerPhase = "Initializing"
	SyncerPhaseReady        SyncerPhase = "Ready"
	SyncerPhaseSyncing      SyncerPhase = "Syncing"
	SyncerPhaseTerminating  SyncerPhase = "Terminating"
	SyncerPhaseError        SyncerPhase = "Error"
)

// ResourceSyncer handles synchronization of a specific resource type
type ResourceSyncer interface {
	// Sync synchronizes a resource to the target
	Sync(ctx context.Context, workspace logicalcluster.Name, obj *unstructured.Unstructured) error

	// Delete removes a resource from the target
	Delete(ctx context.Context, workspace logicalcluster.Name, obj *unstructured.Unstructured) error

	// GetStatus gets status from the target
	GetStatus(ctx context.Context, workspace logicalcluster.Name, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)

	// List lists resources at the target
	List(ctx context.Context, workspace logicalcluster.Name, gvr schema.GroupVersionResource) ([]*unstructured.Unstructured, error)
}

// SyncerFactory creates syncers
type SyncerFactory interface {
	// NewSyncer creates a new syncer instance
	NewSyncer(
		target *workloadv1alpha1.SyncTarget,
		upstreamClient dynamic.ClusterInterface,
		downstreamClient dynamic.Interface,
	) (Syncer, error)

	// ValidateConfiguration validates syncer config
	ValidateConfiguration(config map[string]interface{}) error
}