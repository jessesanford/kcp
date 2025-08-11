// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package controller implements the TMC external controller foundation.
// This controller is designed to consume KCP TMC APIs via APIBinding and manage
// physical Kubernetes clusters for workload placement and execution.
package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

// ClusterRegistrationController manages physical cluster registration and health.
// This controller is responsible for:
// - Connecting to physical Kubernetes clusters
// - Performing health checks on registered clusters
// - Managing cluster capabilities and status
// - Preparing for TMC API consumption via APIBinding (Phase 1 integration)
type ClusterRegistrationController struct {
	// Core components
	queue workqueue.RateLimitingInterface
	
	// KCP client for future TMC API consumption
	kcpClusterClient kcpclientset.ClusterInterface
	
	// Physical cluster clients for health checking
	clusterClients map[string]kubernetes.Interface
	
	// Configuration
	workspace    logicalcluster.Name
	resyncPeriod time.Duration
	workerCount  int
	
	// Health checking state
	clusterHealth map[string]*ClusterHealthStatus
}

// ClusterHealthStatus tracks the health of a physical cluster
type ClusterHealthStatus struct {
	// Name of the cluster
	Name string
	
	// LastCheck time of last health check
	LastCheck time.Time
	
	// Healthy indicates if the cluster is healthy
	Healthy bool
	
	// Error message if unhealthy
	Error string
	
	// NodeCount from the latest health check
	NodeCount int
	
	// Version of the Kubernetes cluster
	Version string
}

// NewClusterRegistrationController creates a new cluster registration controller.
// This controller provides the foundation for TMC cluster management and will
// integrate with KCP TMC APIs once they are available via APIBinding.
func NewClusterRegistrationController(
	kcpClusterClient kcpclientset.ClusterInterface,
	clusterConfigs map[string]*rest.Config,
	workspace logicalcluster.Name,
	resyncPeriod time.Duration,
	workerCount int,
) (*ClusterRegistrationController, error) {
	
	if len(clusterConfigs) == 0 {
		return nil, fmt.Errorf("at least one cluster configuration is required")
	}
	
	// Build cluster clients
	clusterClients := make(map[string]kubernetes.Interface)
	for name, config := range clusterConfigs {
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for cluster %s: %w", name, err)
		}
		clusterClients[name] = client
	}
	
	controller := &ClusterRegistrationController{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cluster-registration"),
		kcpClusterClient: kcpClusterClient,
		clusterClients:   clusterClients,
		workspace:        workspace,
		resyncPeriod:     resyncPeriod,
		workerCount:      workerCount,
		clusterHealth:    make(map[string]*ClusterHealthStatus),
	}
	
	return controller, nil
}

// Start begins the cluster registration controller operations.
func (c *ClusterRegistrationController) Start(ctx context.Context) error {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.InfoS("Starting ClusterRegistrationController", "workspace", c.workspace)

	// Start workers
	for i := 0; i < c.workerCount; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	// Start health checker
	go wait.UntilWithContext(ctx, c.runHealthChecker, c.resyncPeriod)

	<-ctx.Done()
	return nil
}

// runWorker is the main worker goroutine that processes items from the queue.
func (c *ClusterRegistrationController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes the next work item from the queue.
func (c *ClusterRegistrationController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	defer c.queue.Done(obj)

	key, ok := obj.(string)
	if !ok {
		c.queue.Forget(obj)
		utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %T", obj))
		return true
	}

	if err := c.processClusterHealth(ctx, key); err != nil {
		c.queue.AddRateLimited(key)
		utilruntime.HandleError(fmt.Errorf("error processing cluster health %s: %w", key, err))
		return true
	}

	c.queue.Forget(obj)
	return true
}

// processClusterHealth processes health check for a specific cluster.
func (c *ClusterRegistrationController) processClusterHealth(ctx context.Context, clusterName string) error {
	client, exists := c.clusterClients[clusterName]
	if !exists {
		return fmt.Errorf("no client configured for cluster %s", clusterName)
	}

	// Perform health check
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		c.updateClusterHealth(clusterName, false, err.Error(), 0, "")
		return fmt.Errorf("failed to list nodes for cluster %s: %w", clusterName, err)
	}

	// Get server version
	serverVersion, err := client.Discovery().ServerVersion()
	version := ""
	if err == nil {
		version = serverVersion.String()
	}

	c.updateClusterHealth(clusterName, true, "", len(nodes.Items), version)
	return nil
}

// updateClusterHealth updates the health status for a cluster.
func (c *ClusterRegistrationController) updateClusterHealth(name string, healthy bool, errorMsg string, nodeCount int, version string) {
	c.clusterHealth[name] = &ClusterHealthStatus{
		Name:      name,
		LastCheck: time.Now(),
		Healthy:   healthy,
		Error:     errorMsg,
		NodeCount: nodeCount,
		Version:   version,
	}
	
	if healthy {
		klog.V(2).InfoS("Cluster health check passed", "cluster", name, "nodes", nodeCount, "version", version)
	} else {
		klog.ErrorS(fmt.Errorf(errorMsg), "Cluster health check failed", "cluster", name)
	}
}

// runHealthChecker performs periodic health checks on all clusters.
func (c *ClusterRegistrationController) runHealthChecker(ctx context.Context) {
	for clusterName := range c.clusterClients {
		c.queue.Add(clusterName)
	}
}

// GetClusterHealth returns the current health status for all clusters.
func (c *ClusterRegistrationController) GetClusterHealth() map[string]*ClusterHealthStatus {
	result := make(map[string]*ClusterHealthStatus)
	for name, health := range c.clusterHealth {
		result[name] = health
	}
	return result
}