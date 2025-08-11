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

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

// BasicController provides basic controller lifecycle and workqueue processing
// This serves as the foundation controller that demonstrates the basic patterns
// used by all TMC controllers.
type BasicController struct {
	// Core components
	queue workqueue.RateLimitingInterface
	
	// Clients
	kcpClient      kcpclientset.ClusterInterface
	clusterClients map[string]kubernetes.Interface
	
	// Configuration
	workspace    logicalcluster.Name
	resyncPeriod time.Duration
	workerCount  int
}

// NewBasicController creates a new basic controller for demonstrating
// TMC controller patterns.
func NewBasicController(
	kcpClient kcpclientset.ClusterInterface,
	clusterClients map[string]kubernetes.Interface,
	workspace logicalcluster.Name,
	resyncPeriod time.Duration,
	workerCount int,
) (*BasicController, error) {
	
	if len(clusterClients) == 0 {
		return nil, fmt.Errorf("at least one cluster client is required")
	}
	
	c := &BasicController{
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"tmc-basic-controller"),
		kcpClient:      kcpClient,
		clusterClients: clusterClients,
		workspace:      workspace,
		resyncPeriod:   resyncPeriod,
		workerCount:    workerCount,
	}
	
	klog.InfoS("Created basic TMC controller",
		"workspace", workspace,
		"clusters", len(clusterClients),
		"resyncPeriod", resyncPeriod)
	
	return c, nil
}

// Name returns the controller name for identification
func (c *BasicController) Name() string {
	return "basic-tmc-controller"
}

// Start runs the basic controller until the context is cancelled
func (c *BasicController) Start(ctx context.Context) error {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	
	klog.InfoS("Starting basic TMC controller", 
		"workspace", c.workspace,
		"workers", c.workerCount)
	defer klog.InfoS("Shutting down basic TMC controller")
	
	// Start periodic work queueing
	go c.startPeriodicWork(ctx)
	
	// Start workers
	for i := 0; i < c.workerCount; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}
	
	// Queue initial work for all clusters
	for clusterName := range c.clusterClients {
		c.queue.Add(clusterName)
	}
	
	<-ctx.Done()
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *BasicController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *BasicController) processNextWorkItem(ctx context.Context) bool {
	obj, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(obj)
	
	clusterName := obj.(string)
	if err := c.syncCluster(ctx, clusterName); err != nil {
		utilruntime.HandleError(fmt.Errorf("error syncing cluster %s: %w", clusterName, err))
		c.queue.AddRateLimited(obj)
		return true
	}
	
	c.queue.Forget(obj)
	return true
}

// syncCluster performs basic cluster synchronization tasks
func (c *BasicController) syncCluster(ctx context.Context, clusterName string) error {
	klog.V(4).InfoS("Syncing cluster", "cluster", clusterName)
	
	client, exists := c.clusterClients[clusterName]
	if !exists {
		return fmt.Errorf("cluster client not found: %s", clusterName)
	}
	
	// Perform basic cluster verification
	if err := c.verifyClusterConnection(ctx, clusterName, client); err != nil {
		klog.V(2).InfoS("Cluster verification failed", "cluster", clusterName, "error", err)
		return err
	}
	
	klog.V(2).InfoS("Cluster sync completed", "cluster", clusterName)
	return nil
}

// verifyClusterConnection performs basic connectivity verification
func (c *BasicController) verifyClusterConnection(ctx context.Context, clusterName string, client kubernetes.Interface) error {
	_, err := client.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to verify cluster connectivity: %w", err)
	}
	
	klog.V(4).InfoS("Cluster connectivity verified", "cluster", clusterName)
	return nil
}

// startPeriodicWork starts periodic work scheduling for all clusters
func (c *BasicController) startPeriodicWork(ctx context.Context) {
	ticker := time.NewTicker(c.resyncPeriod)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Queue work for all clusters
			for clusterName := range c.clusterClients {
				c.queue.Add(clusterName)
			}
		}
	}
}