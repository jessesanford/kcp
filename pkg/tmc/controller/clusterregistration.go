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
	"net/http"
	"time"

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"
)

const (
	// ClusterRegistrationControllerName is the name of the cluster registration controller
	ClusterRegistrationControllerName = "cluster-registration"
)

// ClusterRegistrationController manages cluster registrations and health monitoring.
// This is a demonstration controller that shows how to use the TMCController foundation
// for managing cluster resources in a workspace-aware manner.
type ClusterRegistrationController struct {
	*TMCController
}

// NewClusterRegistrationController creates a new cluster registration controller
// using the TMC controller foundation. This demonstrates the pattern for building
// specific TMC controllers on top of the foundation.
//
// Parameters:
//   - informer: SharedIndexInformer for the resources to watch
//   - healthCheckInterval: How often to perform health checks
//
// Returns:
//   - *ClusterRegistrationController: Configured controller ready to start
//   - error: Configuration or setup error
func NewClusterRegistrationController(
	informer cache.SharedIndexInformer,
	healthCheckInterval time.Duration,
) (*ClusterRegistrationController, error) {
	
	controller := &ClusterRegistrationController{}
	
	// Create the underlying TMC controller foundation
	tmcController, err := NewTMCController(TMCControllerOptions{
		Name:                ClusterRegistrationControllerName,
		Informer:            informer,
		SyncHandler:         controller.syncClusterRegistration,
		HealthChecker:       controller.performGlobalHealthCheck,
		HealthCheckInterval: healthCheckInterval,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create TMC controller foundation: %w", err)
	}

	controller.TMCController = tmcController
	return controller, nil
}

// syncClusterRegistration handles the reconciliation of a cluster registration resource.
// This demonstrates the reconciliation pattern for TMC resources.
func (c *ClusterRegistrationController) syncClusterRegistration(ctx context.Context, key string) error {
	logger := klog.FromContext(ctx).WithValues("key", key, "controller", ClusterRegistrationControllerName)

	// Parse the key to extract cluster name and object name
	clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid key %q: %w", key, err)
	}

	logger = logger.WithValues("clusterName", clusterName, "resourceName", name)
	logger.V(2).Info("Reconciling cluster registration")

	// Get the object from the informer's cache
	obj, exists, err := c.informer.GetIndexer().GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get object from indexer: %w", err)
	}

	if !exists {
		logger.V(2).Info("Cluster registration no longer exists")
		return nil
	}

	// This is where specific reconciliation logic would go
	// For now, we just log that we're handling the resource
	logger.Info("Successfully reconciled cluster registration", "object", obj)

	return nil
}

// performGlobalHealthCheck performs a global health check for the controller.
// This demonstrates how to implement health checking in TMC controllers.
func (c *ClusterRegistrationController) performGlobalHealthCheck(ctx context.Context) (bool, error) {
	logger := klog.FromContext(ctx).WithName("health-check")
	
	// Perform a simple health check by verifying controller components
	queueLength := c.GetQueueLength()
	if queueLength > 1000 {
		logger.Info("Queue length is high, may indicate processing issues", "queueLength", queueLength)
		return false, nil
	}

	// Check if we can reach a basic endpoint (placeholder for actual health checks)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:8080/healthz")
	if err != nil {
		// Don't fail health check for unreachable endpoint in foundation
		logger.V(4).Info("Health endpoint not reachable (expected in foundation)", "error", err)
		return true, nil
	}
	defer resp.Body.Close()

	healthy := resp.StatusCode == http.StatusOK
	logger.V(4).Info("Health check completed", "healthy", healthy, "statusCode", resp.StatusCode)
	
	return healthy, nil
}

// GetClusterRegistrations returns cluster registrations from the informer cache.
// This demonstrates how to access cached resources in TMC controllers.
func (c *ClusterRegistrationController) GetClusterRegistrations() []interface{} {
	return c.informer.GetIndexer().List()
}

// GetClusterRegistrationByKey retrieves a specific cluster registration by key.
// This demonstrates key-based resource access patterns.
func (c *ClusterRegistrationController) GetClusterRegistrationByKey(key string) (interface{}, bool, error) {
	return c.informer.GetIndexer().GetByKey(key)
}

// ListClusterRegistrationsByWorkspace lists cluster registrations for a specific workspace.
// This demonstrates workspace-aware resource filtering.
func (c *ClusterRegistrationController) ListClusterRegistrationsByWorkspace(workspace logicalcluster.Name) ([]interface{}, error) {
	allObjects := c.informer.GetIndexer().List()
	var workspaceObjects []interface{}

	for _, obj := range allObjects {
		// Extract the workspace from the object's key
		key, err := kcpcache.MetaClusterNamespaceKeyFunc(obj)
		if err != nil {
			continue
		}

		clusterName, _, _, err := kcpcache.SplitMetaClusterNamespaceKey(key)
		if err != nil {
			continue
		}

		if clusterName.String() == workspace.String() {
			workspaceObjects = append(workspaceObjects, obj)
		}
	}

	return workspaceObjects, nil
}
