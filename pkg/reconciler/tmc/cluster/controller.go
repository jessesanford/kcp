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

// Package cluster implements the TMC cluster controller using the KCP reconciler pattern.
// This controller manages physical Kubernetes cluster registration, health checking,
// and integration with the KCP TMC API system.
package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
)

// Controller manages physical cluster registration and health for TMC.
// This controller follows KCP patterns for workspace-aware reconciliation
// and integrates with the APIBinding system for TMC API consumption.
type Controller struct {
	// Core KCP integration
	kcpClusterClient kcpclientset.ClusterInterface
	committer        committer.Committer

	// Work queue for cluster reconciliation
	queue workqueue.RateLimitingInterface

	// Physical cluster clients for health checking
	clusterClients map[string]kubernetes.Interface
	clientsMutex   sync.RWMutex

	// Configuration
	workspace    logicalcluster.Name
	resyncPeriod time.Duration
	workerCount  int

	// Cluster health tracking
	clusterHealth map[string]*ClusterHealthStatus
	healthMutex   sync.RWMutex
}

// ClusterHealthStatus tracks the health and metadata of a physical cluster.
// This status is used for TMC placement decisions and cluster lifecycle management.
type ClusterHealthStatus struct {
	// Name of the cluster
	Name string `json:"name"`

	// LastCheck time of the last successful health check
	LastCheck time.Time `json:"lastCheck"`

	// Healthy indicates if the cluster passed its last health check
	Healthy bool `json:"healthy"`

	// Error contains the error message if the cluster is unhealthy
	Error string `json:"error,omitempty"`

	// NodeCount from the latest health check
	NodeCount int `json:"nodeCount"`

	// Version of the Kubernetes cluster
	Version string `json:"version"`

	// Capacity contains resource capacity information
	Capacity ClusterCapacity `json:"capacity"`

	// Conditions represent the cluster's current conditions
	Conditions []ClusterCondition `json:"conditions,omitempty"`
}

// ClusterCapacity represents basic resource capacity of a cluster
type ClusterCapacity struct {
	CPU    int64 `json:"cpu"`    // CPU capacity in millicores
	Memory int64 `json:"memory"` // Memory capacity in bytes
}

// ClusterCondition represents a basic condition of the cluster
type ClusterCondition struct {
	Type    string    `json:"type"`
	Status  string    `json:"status"`
	Message string    `json:"message"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
}

// ControllerOptions holds basic configuration options for the cluster controller
type ControllerOptions struct {
	ResyncPeriod time.Duration // How often to perform health checks
	WorkerCount  int           // Number of reconciliation workers
}

// DefaultControllerOptions returns sensible defaults for the controller
func DefaultControllerOptions() *ControllerOptions {
	return &ControllerOptions{
		ResyncPeriod: 30 * time.Second,
		WorkerCount:  2,
	}
}

// NewController creates a new TMC cluster controller.
// This controller follows KCP patterns for workspace-aware reconciliation
// and integrates with the committer pattern for status updates.
//
// Parameters:
//   - kcpClusterClient: Cluster-aware KCP client for TMC API access
//   - clusterConfigs: Map of cluster names to their REST configurations
//   - workspace: Logical cluster name for workspace isolation
//   - opts: Controller configuration options
//
// Returns:
//   - *Controller: Configured controller ready to start
//   - error: Configuration or setup error
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	clusterConfigs map[string]*rest.Config,
	workspace logicalcluster.Name,
	opts *ControllerOptions,
) (*Controller, error) {
	if opts == nil {
		opts = DefaultControllerOptions()
	}

	if len(clusterConfigs) == 0 {
		return nil, fmt.Errorf("at least one cluster configuration is required")
	}

	// Build physical cluster clients
	clusterClients := make(map[string]kubernetes.Interface)
	clusterHealth := make(map[string]*ClusterHealthStatus)

	for name, config := range clusterConfigs {
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for cluster %s: %w", name, err)
		}

		clusterClients[name] = client
		clusterHealth[name] = &ClusterHealthStatus{
			Name:       name,
			Healthy:    false, // Start as unhealthy until first check
			Conditions: []ClusterCondition{},
		}

		klog.V(2).InfoS("Configured cluster client", "cluster", name)
	}

	// Create work queue with rate limiting
	queue := workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(),
		"cluster-controller",
	)

	// Create committer for status updates (will be used for TMC API integration)
	committer := committer.NewCommitter(kcpClusterClient, queue)

	c := &Controller{
		kcpClusterClient: kcpClusterClient,
		committer:        committer,
		queue:            queue,
		clusterClients:   clusterClients,
		workspace:        workspace,
		resyncPeriod:     opts.ResyncPeriod,
		workerCount:      opts.WorkerCount,
		clusterHealth:    clusterHealth,
	}

	klog.InfoS("Created TMC cluster controller",
		"workspace", workspace,
		"clusters", len(clusterConfigs))

	return c, nil
}

// Start runs the cluster controller with the given context.
// This method starts the reconciliation workers and periodic health checking.
// It follows the standard KCP controller startup pattern.
func (c *Controller) Start(ctx context.Context) error {
	defer c.queue.ShutDown()

	klog.InfoS("Starting TMC cluster controller",
		"workspace", c.workspace,
		"clusters", len(c.clusterClients))
	defer klog.InfoS("Shutting down TMC cluster controller")

	// Start periodic health checking
	go c.startPeriodicHealthChecks(ctx)

	// Start reconciliation workers
	for i := 0; i < c.workerCount; i++ {
		go c.runWorker(ctx)
	}

	// Queue initial reconciliation for all clusters
	c.enqueueAllClusters()

	<-ctx.Done()
	return nil
}

// enqueueAllClusters adds all configured clusters to the work queue
func (c *Controller) enqueueAllClusters() {
	c.clientsMutex.RLock()
	defer c.clientsMutex.RUnlock()

	for clusterName := range c.clusterClients {
		c.queue.Add(clusterName)
		klog.V(4).InfoS("Enqueued cluster for reconciliation", "cluster", clusterName)
	}
}

// startPeriodicHealthChecks runs periodic health checks for all clusters
func (c *Controller) startPeriodicHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(c.resyncPeriod)
	defer ticker.Stop()

	klog.V(2).InfoS("Starting periodic health checks", "period", c.resyncPeriod)

	for {
		select {
		case <-ctx.Done():
			klog.V(2).InfoS("Stopping periodic health checks")
			return
		case <-ticker.C:
			c.enqueueAllClusters()
		}
	}
}

// GetClusterHealth returns the current health status of a cluster.
// This method is thread-safe and returns a copy of the health status.
func (c *Controller) GetClusterHealth(clusterName string) (*ClusterHealthStatus, bool) {
	c.healthMutex.RLock()
	defer c.healthMutex.RUnlock()

	health, exists := c.clusterHealth[clusterName]
	if !exists {
		return nil, false
	}

	// Return a deep copy to avoid race conditions
	return c.copyClusterHealth(health), true
}

// GetAllClusterHealth returns health status for all configured clusters.
// This method is thread-safe and returns copies of all health statuses.
func (c *Controller) GetAllClusterHealth() map[string]*ClusterHealthStatus {
	c.healthMutex.RLock()
	defer c.healthMutex.RUnlock()

	result := make(map[string]*ClusterHealthStatus, len(c.clusterHealth))
	for name, health := range c.clusterHealth {
		result[name] = c.copyClusterHealth(health)
	}

	return result
}

// IsHealthy returns true if all clusters are currently healthy
func (c *Controller) IsHealthy() bool {
	c.healthMutex.RLock()
	defer c.healthMutex.RUnlock()

	for _, health := range c.clusterHealth {
		if !health.Healthy {
			return false
		}
	}
	return true
}

// GetHealthyClusterCount returns the number of healthy clusters
func (c *Controller) GetHealthyClusterCount() int {
	c.healthMutex.RLock()
	defer c.healthMutex.RUnlock()

	count := 0
	for _, health := range c.clusterHealth {
		if health.Healthy {
			count++
		}
	}
	return count
}

// copyClusterHealth creates a deep copy of ClusterHealthStatus
func (c *Controller) copyClusterHealth(health *ClusterHealthStatus) *ClusterHealthStatus {
	if health == nil {
		return nil
	}

	conditions := make([]ClusterCondition, len(health.Conditions))
	copy(conditions, health.Conditions)

	return &ClusterHealthStatus{
		Name:      health.Name,
		LastCheck: health.LastCheck,
		Healthy:   health.Healthy,
		Error:     health.Error,
		NodeCount: health.NodeCount,
		Version:   health.Version,
		Capacity: ClusterCapacity{
			CPU:     health.Capacity.CPU,
			Memory:  health.Capacity.Memory,
			Storage: health.Capacity.Storage,
			Pods:    health.Capacity.Pods,
		},
		Conditions: conditions,
	}
}