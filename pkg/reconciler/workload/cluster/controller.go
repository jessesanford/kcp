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

package cluster

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/logging"
	corev1alpha1 "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	corev1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/core/v1alpha1"
	corev1alpha1listers "github.com/kcp-dev/kcp/sdk/client/listers/core/v1alpha1"
)

const (
	// ControllerName is the name of the cluster controller.
	ControllerName = "kcp-workload-cluster"
)

// ClusterRegistration represents a cluster registration in the TMC system.
// For Phase 6 Wave 2, we'll work with a simplified structure that can be
// extended as the full TMC APIs are developed.
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of ClusterRegistration.
type ClusterRegistrationSpec struct {
	// Location specifies the location of the cluster.
	Location string `json:"location,omitempty"`
	
	// Labels contains metadata labels for the cluster.
	Labels map[string]string `json:"labels,omitempty"`
	
	// Capabilities describes the capabilities of the cluster.
	Capabilities map[string]string `json:"capabilities,omitempty"`
}

// ClusterRegistrationStatus defines the observed state of ClusterRegistration.
type ClusterRegistrationStatus struct {
	// Conditions represents the latest available observations of the cluster registration's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	
	// Phase represents the current phase of cluster registration.
	Phase ClusterRegistrationPhase `json:"phase,omitempty"`
	
	// SyncTargetRef references the associated SyncTarget.
	SyncTargetRef *ClusterReference `json:"syncTargetRef,omitempty"`
}

// ClusterRegistrationPhase defines the phase of cluster registration.
type ClusterRegistrationPhase string

const (
	// ClusterRegistrationPhasePending indicates the cluster registration is being processed.
	ClusterRegistrationPhasePending ClusterRegistrationPhase = "Pending"
	
	// ClusterRegistrationPhaseRegistered indicates the cluster is registered.
	ClusterRegistrationPhaseRegistered ClusterRegistrationPhase = "Registered"
	
	// ClusterRegistrationPhaseReady indicates the cluster is ready for workloads.
	ClusterRegistrationPhaseReady ClusterRegistrationPhase = "Ready"
	
	// ClusterRegistrationPhaseFailed indicates the cluster registration failed.
	ClusterRegistrationPhaseFailed ClusterRegistrationPhase = "Failed"
)

// ClusterReference represents a reference to a cluster resource.
type ClusterReference struct {
	// Name is the name of the referenced resource.
	Name string `json:"name"`
	
	// Namespace is the namespace of the referenced resource.
	Namespace string `json:"namespace,omitempty"`
	
	// Cluster is the logical cluster of the referenced resource.
	Cluster string `json:"cluster,omitempty"`
}

// NewController creates a new cluster registration controller that manages
// ClusterRegistration resources and coordinates with SyncTarget controllers.
//
// The controller follows KCP patterns for workspace isolation and integrates
// with the APIExport system for TMC functionality.
//
// Parameters:
//   - kcpClusterClient: Cluster-aware KCP client for API operations
//   - logicalClusterInformer: Informer for LogicalCluster resources
//   - workspace: Logical cluster name for workspace isolation
//
// Returns:
//   - *controller: Configured controller ready to start
//   - error: Configuration or setup error
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	logicalClusterInformer corev1alpha1informers.LogicalClusterClusterInformer,
	workspace logicalcluster.Name,
) (*controller, error) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[string](),
		workqueue.TypedRateLimitingQueueConfig[string]{
			Name: ControllerName,
		},
	)

	// Initialize cluster manager with default implementations
	clusterManager := NewClusterManager(
		NewDefaultClientBuilder(),
		NewDefaultCertificateValidator(),
		NewDefaultRBACManager(),
		NewDefaultSyncTargetManager(),
		NewDefaultPlacementNotifier(),
	)

	c := &controller{
		queue:              queue,
		kcpClusterClient:   kcpClusterClient,
		workspace:          workspace,
		logicalClusterLister: logicalClusterInformer.Lister(),
		clusterManager:     clusterManager,
		
		// Cluster registration operations
		listClusters: func(clusterName logicalcluster.Name) ([]*ClusterRegistration, error) {
			// In a full implementation, this would use a ClusterRegistration lister
			// For now, we'll return empty list as a placeholder
			return []*ClusterRegistration{}, nil
		},
		
		getCluster: func(clusterName logicalcluster.Name, name string) (*ClusterRegistration, error) {
			// Placeholder for getting a specific ClusterRegistration
			return nil, apierrors.NewNotFound(corev1alpha1.Resource("clusterregistrations"), name)
		},
	}

	// Set up informer event handlers
	_, _ = logicalClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.enqueueLogicalCluster(obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { c.enqueueLogicalCluster(newObj) },
		DeleteFunc: func(obj interface{}) { c.enqueueLogicalCluster(obj) },
	})

	return c, nil
}

// controller manages ClusterRegistration resources and coordinates with
// SyncTarget controllers for TMC workload placement.
type controller struct {
	queue              workqueue.TypedRateLimitingInterface[string]
	kcpClusterClient   kcpclientset.ClusterInterface
	workspace          logicalcluster.Name
	logicalClusterLister corev1alpha1listers.LogicalClusterClusterLister
	
	// Cluster management
	clusterManager *ClusterManager

	// Cluster registration operations
	listClusters           func(clusterName logicalcluster.Name) ([]*ClusterRegistration, error)
	getCluster             func(clusterName logicalcluster.Name, name string) (*ClusterRegistration, error)
}

// enqueueLogicalCluster enqueues a LogicalCluster for processing.
func (c *controller) enqueueLogicalCluster(obj interface{}) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	
	logger := logging.WithQueueKey(logging.WithReconciler(klog.Background(), ControllerName), key)
	logger.V(2).Info("queueing LogicalCluster")
	c.queue.Add(key)
}

// Start starts the controller and blocks until the context is cancelled.
func (c *controller) Start(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	ctx = klog.NewContext(ctx, logger)
	logger.Info("Starting controller")
	defer logger.Info("Shutting down controller")

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

// startWorker runs a single worker thread for processing queue items.
func (c *controller) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes the next item in the work queue.
func (c *controller) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	logger := logging.WithQueueKey(klog.FromContext(ctx), key)
	ctx = klog.NewContext(ctx, logger)
	
	err := c.process(ctx, key)
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("failed to process key %q: %w", key, err))
	c.queue.AddRateLimited(key)
	return true
}

// process handles the reconciliation of a single cluster registration.
func (c *controller) process(ctx context.Context, key string) error {
	logger := klog.FromContext(ctx)
	
	clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		logger.Error(err, "invalid key")
		return nil
	}

	obj, err := c.logicalClusterLister.Cluster(clusterName).Get(name)
	if apierrors.IsNotFound(err) {
		logger.V(2).Info("LogicalCluster not found")
		return nil
	}
	if err != nil {
		return err
	}

	old := obj
	obj = obj.DeepCopy()

	logger = logging.WithObject(logger, obj)
	ctx = klog.NewContext(ctx, logger)

	var errs []error
	if err := c.reconcile(ctx, obj); err != nil {
		errs = append(errs, err)
	}

	// Update status if needed
	oldResource := &Resource{ObjectMeta: old.ObjectMeta, Status: &old.Status}
	newResource := &Resource{ObjectMeta: obj.ObjectMeta, Status: &obj.Status}
	updatedObj, updateErr := c.commit(ctx, oldResource, newResource, obj)
	if updateErr != nil {
		errs = append(errs, updateErr)
	}
	if updatedObj != nil {
		obj = updatedObj
	}

	return utilerrors.NewAggregate(errs)
}

// Resource represents a resource that can be committed.
type Resource struct {
	metav1.ObjectMeta
	Status *corev1alpha1.LogicalClusterStatus `json:"status,omitempty"`
}

// commit commits the resource using the committer pattern.
func (c *controller) commit(ctx context.Context, oldResource, newResource *Resource, obj *corev1alpha1.LogicalCluster) (*corev1alpha1.LogicalCluster, error) {
	clusterName := logicalcluster.From(obj)
	
	specOrObjectMetaChanged := !equality.Semantic.DeepEqual(oldResource.ObjectMeta, newResource.ObjectMeta)
	statusChanged := !equality.Semantic.DeepEqual(oldResource.Status, newResource.Status)

	if specOrObjectMetaChanged && statusChanged {
		logger := klog.FromContext(ctx)
		logger.V(2).Info("committing spec and status change")
		return c.kcpClusterClient.Cluster(clusterName.Path()).CoreV1alpha1().LogicalClusters().Update(ctx, obj, metav1.UpdateOptions{})
	} else if statusChanged {
		logger := klog.FromContext(ctx)
		logger.V(2).Info("committing status change")
		return c.kcpClusterClient.Cluster(clusterName.Path()).CoreV1alpha1().LogicalClusters().UpdateStatus(ctx, obj, metav1.UpdateOptions{})
	}

	return obj, nil
}

// reconcile performs the main reconciliation logic for cluster registration.
// This integrates with the ClusterManager for full cluster lifecycle management.
func (c *controller) reconcile(ctx context.Context, logicalCluster *corev1alpha1.LogicalCluster) error {
	logger := klog.FromContext(ctx)
	clusterName := logicalcluster.From(logicalCluster)
	
	logger.V(2).Info("reconciling logical cluster for workload placement", "cluster", clusterName)

	// Get all cluster registrations for this logical cluster
	clusters, err := c.listClusters(clusterName)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	// Process each cluster registration
	var errs []error
	for _, cluster := range clusters {
		if err := c.reconcileClusterRegistration(ctx, cluster); err != nil {
			logger.Error(err, "failed to reconcile cluster registration", "cluster", cluster.Name)
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

// reconcileClusterRegistration reconciles a single cluster registration using the ClusterManager.
func (c *controller) reconcileClusterRegistration(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("reconciling cluster registration", "cluster", cluster.Name)

	// Use ClusterManager for full reconciliation
	if err := c.clusterManager.ReconcileCluster(ctx, cluster); err != nil {
		return fmt.Errorf("cluster manager reconciliation failed: %w", err)
	}

	logger.V(2).Info("cluster registration reconciled successfully", "cluster", cluster.Name, "phase", cluster.Status.Phase)
	return nil
}