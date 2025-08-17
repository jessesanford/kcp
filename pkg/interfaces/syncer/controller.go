package syncer

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/kcp-dev/kcp/pkg/logicalcluster"
)

// Controller manages the sync loop for resources
type Controller interface {
	// Run starts the controller
	Run(ctx context.Context, workers int) error

	// Enqueue adds an item to the work queue
	Enqueue(key string)

	// EnqueueAfter adds an item after a delay
	EnqueueAfter(key string, duration time.Duration)

	// RegisterEventHandler registers handlers for a GVR
	RegisterEventHandler(gvr schema.GroupVersionResource, handler cache.ResourceEventHandler) error
}

// Reconciler reconciles individual resources
type Reconciler interface {
	// Reconcile processes a single resource
	Reconcile(ctx context.Context, key ReconcileKey) (ReconcileResult, error)
}

// ReconcileKey identifies a resource to reconcile
type ReconcileKey struct {
	// Workspace logical cluster
	Workspace logicalcluster.Name

	// GVR of the resource
	GVR schema.GroupVersionResource

	// Namespace of the resource (if namespaced)
	Namespace string

	// Name of the resource
	Name string
}

// ReconcileResult contains reconciliation outcome
type ReconcileResult struct {
	// Requeue if reconciliation should be retried
	Requeue bool

	// RequeueAfter duration
	RequeueAfter time.Duration

	// Error from reconciliation
	Error error
}

// WorkQueue wraps a workqueue for the controller
type WorkQueue interface {
	workqueue.RateLimitingInterface

	// AddAfter adds an item after a delay
	AddAfter(item interface{}, duration time.Duration)

	// Len returns queue length
	Len() int

	// ShuttingDown returns if queue is shutting down
	ShuttingDown() bool
}

// EventHandler processes resource events
type EventHandler interface {
	// OnAdd handles resource creation
	OnAdd(obj interface{})

	// OnUpdate handles resource updates
	OnUpdate(oldObj, newObj interface{})

	// OnDelete handles resource deletion
	OnDelete(obj interface{})
}

// IndexerFactory creates indexers for resources
type IndexerFactory interface {
	// NewIndexer creates an indexer for a GVR
	NewIndexer(gvr schema.GroupVersionResource) (cache.Indexer, error)

	// AddIndexers adds index functions
	AddIndexers(indexer cache.Indexer, indexers cache.Indexers) error
}