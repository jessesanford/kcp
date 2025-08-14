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
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/util/workqueue"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	"github.com/kcp-dev/kcp/pkg/features"
)

// Metrics for TMC cluster health monitoring
var (
	clusterHealthCheckDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "tmc_cluster_health_check_duration_seconds",
			Help: "Duration of cluster health checks",
		},
		[]string{"cluster", "workspace"},
	)
	
	clusterHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tmc_cluster_health_status",
			Help: "Health status of clusters (1=healthy, 0=unhealthy)",
		},
		[]string{"cluster", "workspace"},
	)
)

// init registers the metrics
func init() {
	prometheus.MustRegister(clusterHealthCheckDuration)
	prometheus.MustRegister(clusterHealthStatus)
}

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
	
	// Thread safety
	mu sync.RWMutex
	
	// Retry configuration
	maxRetries     int
	backoffManager wait.Backoff
}

// ClusterHealthStatus tracks the health of a physical cluster
type ClusterHealthStatus struct {
	// Name of the cluster
	Name string
	
	// Workspace this cluster belongs to
	Workspace logicalcluster.Name
	
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
	clusterHealth := make(map[string]*ClusterHealthStatus)
	
	for name, config := range clusterConfigs {
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for cluster %s: %w", name, err)
		}
		clusterClients[name] = client
		clusterHealth[name] = &ClusterHealthStatus{
			Name:      name,
			Workspace: workspace,
			Healthy: false, // Start as unhealthy until first check
		}
		
		klog.V(2).InfoS("Configured cluster client", "cluster", name)
	}
	
	c := &ClusterRegistrationController{
		queue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"cluster-registration"),
		kcpClusterClient: kcpClusterClient,
		clusterClients:   clusterClients,
		workspace:        workspace,
		resyncPeriod:     resyncPeriod,
		workerCount:      workerCount,
		clusterHealth:    clusterHealth,
		maxRetries: 5,
		backoffManager: wait.Backoff{
			Duration: 1 * time.Second,
			Factor:   2.0,
			Jitter:   0.1,
			Steps:    5,
			Cap:      30 * time.Second,
		},
	}
	
	klog.InfoS("Created cluster registration controller",
		"workspace", workspace,
		"clusters", len(clusterConfigs),
		"resyncPeriod", resyncPeriod)
	
	return c, nil
}

// Start runs the cluster registration controller.
// This method starts health checking of all configured clusters and prepares
// for future integration with KCP TMC APIs via APIBinding.
func (c *ClusterRegistrationController) Start(ctx context.Context) error {
	// Check feature flags
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMC) {
		klog.InfoS("TMC feature disabled, skipping ClusterRegistration controller")
		return nil
	}
	
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCClusterRegistration) {
		klog.InfoS("TMCClusterRegistration feature disabled, skipping controller")
		return nil
	}
	
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	
	klog.InfoS("Starting ClusterRegistration controller", 
		"workspace", c.workspace,
		"clusters", len(c.clusterClients))
	defer klog.InfoS("Shutting down ClusterRegistration controller")
	
	// Start periodic health checks
	go c.startHealthChecking(ctx)
	
	// Start workers
	for i := 0; i < c.workerCount; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}
	
	// Initial health check for all clusters
	c.mu.RLock()
	for clusterName := range c.clusterClients {
		c.queue.Add(clusterName)
	}
	
	c.mu.RUnlock()
	
	<-ctx.Done()
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *ClusterRegistrationController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *ClusterRegistrationController) processNextWorkItem(ctx context.Context) bool {
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

// syncCluster performs health checking for a single cluster
func (c *ClusterRegistrationController) syncCluster(ctx context.Context, clusterName string) error {
	klog.V(4).InfoS("Syncing cluster", "cluster", clusterName, "workspace", c.workspace)
	
	c.mu.RLock()
	client, exists := c.clusterClients[clusterName]
	c.mu.RUnlock()
	if !exists {
		return fmt.Errorf("cluster client not found: %s", clusterName)
	}
	
	// Perform health check
	healthy, err := c.performHealthCheck(ctx, clusterName, client)
	
	// Update health status with lock
	c.mu.Lock()
	c.clusterHealth[clusterName] = &ClusterHealthStatus{
		Name:      clusterName,
		Workspace: c.workspace,
		LastCheck: time.Now(),
		Healthy:   healthy,
		Error:     func() string { if err != nil { return err.Error() }; return "" }(),
	}
	c.mu.Unlock()
	
	if healthy {
		klog.V(2).InfoS("Cluster health check passed", "cluster", clusterName, "workspace", c.workspace)
	} else {
		klog.V(2).InfoS("Cluster health check failed", "cluster", clusterName, "workspace", c.workspace, "error", err)
	}
	
	return nil
}

// performHealthCheck tests cluster connectivity and basic functionality
func (c *ClusterRegistrationController) performHealthCheck(ctx context.Context, clusterName string, client kubernetes.Interface) (bool, error) {
	start := time.Now()
	defer func() {
		clusterHealthCheckDuration.WithLabelValues(clusterName, string(c.workspace)).Observe(time.Since(start).Seconds())
	}()
	
	// Check context before expensive operations
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	
	// Test 1: List nodes to verify API server connectivity
	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		clusterHealthStatus.WithLabelValues(clusterName, string(c.workspace)).Set(0)
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}
	
	// Test 2: Get cluster version
	version, err := client.Discovery().ServerVersion()
	if err != nil {
		clusterHealthStatus.WithLabelValues(clusterName, string(c.workspace)).Set(0)
		return false, fmt.Errorf("failed to get server version: %w", err)
	}
	
	// Update health status with additional info
	c.mu.Lock()
	if health, exists := c.clusterHealth[clusterName]; exists {
		health.NodeCount = len(nodeList.Items)
		health.Version = version.String()
	}
	c.mu.Unlock()
	
	klog.V(4).InfoS("Cluster health check details",
		"cluster", clusterName,
		"nodes", len(nodeList.Items),
		"version", version.String())
	
	// Update metric
	clusterHealthStatus.WithLabelValues(clusterName, string(c.workspace)).Set(1)
	
	return true, nil
}

// startHealthChecking starts periodic health checking for all clusters
func (c *ClusterRegistrationController) startHealthChecking(ctx context.Context) {
	ticker := time.NewTicker(c.resyncPeriod)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Queue health checks for all clusters
			c.mu.RLock()
			for clusterName := range c.clusterClients {
				c.queue.Add(clusterName)
			}
			c.mu.RUnlock()
		}
	}
}

// GetClusterHealth returns the current health status of a cluster
func (c *ClusterRegistrationController) GetClusterHealth(clusterName string) (*ClusterHealthStatus, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	health, exists := c.clusterHealth[clusterName]
	if !exists {
		return nil, false
	}
	
	// Return a copy to avoid race conditions
	return &ClusterHealthStatus{
		Name:      health.Name,
		Workspace: health.Workspace,
		LastCheck: health.LastCheck,
		Healthy:   health.Healthy,
		Error:     health.Error,
		NodeCount: health.NodeCount,
		Version:   health.Version,
	}, true
}

// GetAllClusterHealth returns health status for all clusters
func (c *ClusterRegistrationController) GetAllClusterHealth() map[string]*ClusterHealthStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string]*ClusterHealthStatus)
	
	for name, health := range c.clusterHealth {
		result[name] = &ClusterHealthStatus{
			Name:      health.Name,
			Workspace: health.Workspace,
			LastCheck: health.LastCheck,
			Healthy:   health.Healthy,
			Error:     health.Error,
			NodeCount: health.NodeCount,
			Version:   health.Version,
		}
	}
	
	return result
}

// IsHealthy returns true if all clusters are healthy
func (c *ClusterRegistrationController) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	for _, health := range c.clusterHealth {
		if !health.Healthy {
			return false
		}
	}
	return true
}
// Helper function to determine retryable errors
func isRetryableError(err error) bool {
	// Network errors are typically retryable
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	
	// Check for specific error types
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}
	
	// Connection refused, etc.
	if strings.Contains(err.Error(), "connection refused") ||
	   strings.Contains(err.Error(), "i/o timeout") {
		return true
	}
	
	return false
}
