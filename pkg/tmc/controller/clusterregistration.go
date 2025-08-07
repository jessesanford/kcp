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

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	tmcv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/tmc/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/util/conditions"
)

// ClusterRegistrationController manages physical cluster registration and health monitoring.
// This controller runs outside KCP and watches ClusterRegistration resources via APIBinding,
// then performs health checks against the physical clusters.
type ClusterRegistrationController struct {
	queue workqueue.RateLimitingInterface

	// KCP clients for watching ClusterRegistration resources
	kcpClusterClient         kcpclientset.ClusterInterface
	clusterRegistrationLister tmcv1alpha1informers.ClusterRegistrationClusterLister

	// Physical cluster clients for health checking
	clusterClients map[string]kubernetes.Interface
	clientsMutex   sync.RWMutex

	// Configuration
	workspace                  logicalcluster.Name
	healthCheckInterval        time.Duration
	enableHealthChecking       bool
	
	// Metrics and monitoring
	lastHealthCheckTime        sync.Map // cluster name -> time.Time
	healthCheckErrors          sync.Map // cluster name -> error count
}

// NewClusterRegistrationController creates a new cluster registration controller that manages
// physical cluster connections and performs health monitoring.
func NewClusterRegistrationController(
	kcpClusterClient kcpclientset.ClusterInterface,
	clusterRegistrationInformer tmcv1alpha1informers.ClusterRegistrationClusterInformer,
	clusterConfigs map[string]*rest.Config,
	workspace logicalcluster.Name,
	healthCheckInterval time.Duration,
	enableHealthChecking bool,
) (*ClusterRegistrationController, error) {

	// Build cluster clients from configs
	clusterClients := make(map[string]kubernetes.Interface)
	for name, config := range clusterConfigs {
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for cluster %s: %w", name, err)
		}
		clusterClients[name] = client
	}

	c := &ClusterRegistrationController{
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"cluster-registration"),
		kcpClusterClient:          kcpClusterClient,
		clusterRegistrationLister: clusterRegistrationInformer.Lister(),
		clusterClients:           clusterClients,
		workspace:                workspace,
		healthCheckInterval:      healthCheckInterval,
		enableHealthChecking:     enableHealthChecking,
	}

	// Set up event handlers for ClusterRegistration resources
	clusterRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueue(obj)
			klog.V(4).InfoS("ClusterRegistration added", "object", obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.enqueue(newObj)
			klog.V(4).InfoS("ClusterRegistration updated", "object", newObj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueue(obj)
			klog.V(4).InfoS("ClusterRegistration deleted", "object", obj)
		},
	})

	return c, nil
}

// enqueue adds a ClusterRegistration to the work queue
func (c *ClusterRegistrationController) enqueue(obj interface{}) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %w", obj, err))
		return
	}
	c.queue.Add(key)
}

// Start runs the controller with the specified number of worker threads
func (c *ClusterRegistrationController) Start(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.InfoS("Starting ClusterRegistration controller", 
		"workspace", c.workspace,
		"workers", numThreads,
		"healthCheckEnabled", c.enableHealthChecking,
		"healthCheckInterval", c.healthCheckInterval)
	defer klog.InfoS("Shutting down ClusterRegistration controller")

	// Start periodic health checking if enabled
	if c.enableHealthChecking {
		go c.runHealthChecker(ctx)
	}

	// Start worker threads
	for i := 0; i < numThreads; i++ {
		go c.runWorker(ctx)
	}

	<-ctx.Done()
}

// runWorker is the main worker loop that processes work items from the queue
func (c *ClusterRegistrationController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// processNextWorkItem processes a single work item from the queue
func (c *ClusterRegistrationController) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.reconcile(ctx, key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("syncing ClusterRegistration %q failed: %w", key, err))
	c.queue.AddRateLimited(key)
	return true
}

// reconcile handles a single ClusterRegistration resource
func (c *ClusterRegistrationController) reconcile(ctx context.Context, key string) error {
	clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		return err
	}

	// Only process resources in our workspace
	if clusterName != c.workspace {
		klog.V(4).InfoS("Skipping ClusterRegistration from different workspace", 
			"key", key, "workspace", clusterName, "ourWorkspace", c.workspace)
		return nil
	}

	clusterReg, err := c.clusterRegistrationLister.Cluster(clusterName).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ClusterRegistration was deleted", "key", key)
		// Clean up any cached data for this cluster
		c.cleanupClusterData(name)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get ClusterRegistration %s: %w", key, err)
	}

	return c.syncClusterRegistration(ctx, clusterReg)
}

// syncClusterRegistration processes a ClusterRegistration and updates its status
func (c *ClusterRegistrationController) syncClusterRegistration(
	ctx context.Context,
	clusterReg *tmcv1alpha1.ClusterRegistration,
) error {
	klog.V(3).InfoS("Processing ClusterRegistration", 
		"name", clusterReg.Name,
		"location", clusterReg.Spec.Location,
		"endpoint", clusterReg.Spec.ClusterEndpoint.ServerURL)

	// Check if we have a client for this cluster
	c.clientsMutex.RLock()
	clusterClient, exists := c.clusterClients[clusterReg.Name]
	c.clientsMutex.RUnlock()

	if !exists {
		return c.updateClusterRegistrationStatus(ctx, clusterReg, false, 
			"ClusterNotConfigured", 
			"No kubeconfig provided for this cluster")
	}

	// Perform immediate health check
	healthy, err := c.testClusterHealth(ctx, clusterClient, clusterReg.Name)
	if err != nil {
		return c.updateClusterRegistrationStatus(ctx, clusterReg, false,
			"ClusterUnhealthy", 
			fmt.Sprintf("Health check failed: %v", err))
	}

	if !healthy {
		return c.updateClusterRegistrationStatus(ctx, clusterReg, false,
			"ClusterUnhealthy", "Cluster failed health checks")
	}

	// Update status to ready
	return c.updateClusterRegistrationStatus(ctx, clusterReg, true,
		"ClusterReady", "Cluster is healthy and ready for workload placement")
}

// testClusterHealth performs a health check against a physical cluster
func (c *ClusterRegistrationController) testClusterHealth(
	ctx context.Context, 
	client kubernetes.Interface,
	clusterName string,
) (bool, error) {
	// Record the health check attempt
	c.lastHealthCheckTime.Store(clusterName, time.Now())

	// Simple health check - try to list nodes with a reasonable timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := client.CoreV1().Nodes().List(timeoutCtx, metav1.ListOptions{Limit: 1})
	if err != nil {
		// Increment error counter
		if count, exists := c.healthCheckErrors.Load(clusterName); exists {
			c.healthCheckErrors.Store(clusterName, count.(int)+1)
		} else {
			c.healthCheckErrors.Store(clusterName, 1)
		}
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Reset error counter on success
	c.healthCheckErrors.Store(clusterName, 0)
	return true, nil
}

// updateClusterRegistrationStatus updates the ClusterRegistration status in KCP
func (c *ClusterRegistrationController) updateClusterRegistrationStatus(
	ctx context.Context,
	clusterReg *tmcv1alpha1.ClusterRegistration,
	ready bool,
	reason string,
	message string,
) error {
	// Work on a copy to avoid modifying the cached object
	clusterReg = clusterReg.DeepCopy()

	// Update heartbeat timestamp
	now := metav1.NewTime(time.Now())
	clusterReg.Status.LastHeartbeat = &now

	// Update Ready condition
	if ready {
		conditions.MarkTrue(clusterReg, tmcv1alpha1.ClusterRegistrationReady)
	} else {
		conditions.MarkFalse(clusterReg, 
			tmcv1alpha1.ClusterRegistrationReady,
			reason, 
			conditionsv1alpha1.ConditionSeverityError,
			message)
	}

	// Update the status via KCP client
	_, err := c.kcpClusterClient.Cluster(c.workspace.Path()).
		TmcV1alpha1().
		ClusterRegistrations().
		UpdateStatus(ctx, clusterReg, metav1.UpdateOptions{})

	if err != nil {
		return fmt.Errorf("failed to update ClusterRegistration status for %s: %w", 
			clusterReg.Name, err)
	}

	klog.V(3).InfoS("Updated ClusterRegistration status", 
		"name", clusterReg.Name, 
		"ready", ready,
		"reason", reason)

	return nil
}

// runHealthChecker runs periodic health checks for all registered clusters
func (c *ClusterRegistrationController) runHealthChecker(ctx context.Context) {
	ticker := time.NewTicker(c.healthCheckInterval)
	defer ticker.Stop()

	klog.InfoS("Starting cluster health checker", "interval", c.healthCheckInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.performPeriodicHealthChecks(ctx)
		}
	}
}

// performPeriodicHealthChecks checks the health of all registered clusters
func (c *ClusterRegistrationController) performPeriodicHealthChecks(ctx context.Context) {
	// Get all ClusterRegistrations in our workspace
	clusterRegs, err := c.clusterRegistrationLister.Cluster(c.workspace).List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "Failed to list ClusterRegistrations for health check")
		return
	}

	klog.V(4).InfoS("Performing periodic health checks", "clusters", len(clusterRegs))

	for _, clusterReg := range clusterRegs {
		// Skip if we don't have a client for this cluster
		c.clientsMutex.RLock()
		client, exists := c.clusterClients[clusterReg.Name]
		c.clientsMutex.RUnlock()

		if !exists {
			continue
		}

		// Perform health check in goroutine to avoid blocking
		go func(reg *tmcv1alpha1.ClusterRegistration, cl kubernetes.Interface) {
			healthy, err := c.testClusterHealth(ctx, cl, reg.Name)
			
			// Update status based on health check result
			var reason, message string
			if err != nil {
				reason = "HealthCheckFailed"
				message = fmt.Sprintf("Periodic health check failed: %v", err)
			} else if !healthy {
				reason = "ClusterUnhealthy"
				message = "Periodic health check indicates cluster is unhealthy"
			} else {
				reason = "HealthCheckPassed"
				message = "Periodic health check passed"
			}

			if err := c.updateClusterRegistrationStatus(ctx, reg, healthy, reason, message); err != nil {
				klog.ErrorS(err, "Failed to update cluster status during health check", 
					"cluster", reg.Name)
			}
		}(clusterReg, client)
	}
}

// cleanupClusterData cleans up any cached data when a cluster is deleted
func (c *ClusterRegistrationController) cleanupClusterData(clusterName string) {
	c.lastHealthCheckTime.Delete(clusterName)
	c.healthCheckErrors.Delete(clusterName)
	klog.V(3).InfoS("Cleaned up cluster data", "cluster", clusterName)
}