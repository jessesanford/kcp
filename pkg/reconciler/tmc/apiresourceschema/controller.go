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

package apiresourceschema

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/features"
	"github.com/kcp-dev/kcp/pkg/logging"
	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	apisv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/apis/v1alpha1"
)

const (
	ControllerName = "kcp-tmc-apiresourceschema"
)

// NewController returns a new controller for TMC APIResourceSchemas.
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	apiResourceSchemaInformer apisv1alpha1informers.APIResourceSchemaClusterInformer,
) (*controller, error) {
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[string](),
		workqueue.TypedRateLimitingQueueConfig[string]{
			Name: ControllerName,
		},
	)

	c := &controller{
		queue:            queue,
		kcpClusterClient: kcpClusterClient,

		listAPIResourceSchemas: func() ([]*apisv1alpha1.APIResourceSchema, error) {
			return apiResourceSchemaInformer.Lister().List(labels.Everything())
		},
		getAPIResourceSchema: func(clusterName logicalcluster.Name, name string) (*apisv1alpha1.APIResourceSchema, error) {
			return apiResourceSchemaInformer.Lister().Cluster(clusterName).Get(name)
		},
	}

	_, _ = apiResourceSchemaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueue(obj)
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueue(obj)
		},
	})

	return c, nil
}

// controller manages TMC APIResourceSchemas by ensuring they follow KCP patterns
// and maintain proper lifecycle management for TMC workload management.
type controller struct {
	queue            workqueue.TypedRateLimitingInterface[string]
	kcpClusterClient kcpclientset.ClusterInterface

	listAPIResourceSchemas func() ([]*apisv1alpha1.APIResourceSchema, error)
	getAPIResourceSchema   func(clusterName logicalcluster.Name, name string) (*apisv1alpha1.APIResourceSchema, error)
}

// enqueue adds an APIResourceSchema to the work queue.
func (c *controller) enqueue(obj interface{}) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %w", obj, err))
		return
	}

	logger := logging.WithQueueKey(logging.WithReconciler(klog.Background(), ControllerName), key)
	logger.V(2).Info("queueing APIResourceSchema")
	c.queue.Add(key)
}

// Start runs the controller until the given context is cancelled.
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

// startWorker starts a single worker goroutine.
func (c *controller) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes the next work item in the queue.
func (c *controller) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	logger := logging.WithQueueKey(klog.FromContext(ctx), key)
	ctx = klog.NewContext(ctx, logger)
	logger.V(1).Info("processing key")

	// Invoke the method containing the business logic
	err := c.process(ctx, key)
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(ctx, err, key)
	return true
}

// handleErr handles errors from processing work items.
func (c *controller) handleErr(ctx context.Context, err error, key string) {
	logger := klog.FromContext(ctx)
	if err == nil {
		c.queue.Forget(key)
		return
	}

	logger.Error(err, "syncing APIResourceSchema failed")

	if c.queue.NumRequeues(key) < 5 {
		logger.V(2).Info("retrying APIResourceSchema")
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	logger.V(4).Info("dropping APIResourceSchema out of the queue", "numRequeues", c.queue.NumRequeues(key))
}

// process handles the reconciliation logic for a single APIResourceSchema.
func (c *controller) process(ctx context.Context, key string) error {
	logger := klog.FromContext(ctx)
	clusterName, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid resource key %q: %w", key, err)
	}

	apiResourceSchema, err := c.getAPIResourceSchema(clusterName, name)
	if errors.IsNotFound(err) {
		logger.V(2).Info("APIResourceSchema was deleted")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get APIResourceSchema %s|%s: %w", clusterName, name, err)
	}

	return c.reconcile(ctx, apiResourceSchema)
}

// reconcile handles the core reconciliation logic for TMC APIResourceSchemas.
func (c *controller) reconcile(ctx context.Context, apiResourceSchema *apisv1alpha1.APIResourceSchema) error {
	logger := logging.WithObject(klog.FromContext(ctx), apiResourceSchema)

	// Only process TMC-related APIResourceSchemas
	if !c.isTMCAPIResourceSchema(apiResourceSchema) {
		logger.V(4).Info("skipping non-TMC APIResourceSchema")
		return nil
	}

	logger.V(2).Info("reconciling TMC APIResourceSchema")

	// Validate that the TMC APIResourceSchema follows KCP patterns
	if err := c.validateTMCAPIResourceSchema(ctx, apiResourceSchema); err != nil {
		return fmt.Errorf("TMC APIResourceSchema validation failed: %w", err)
	}

	// Ensure proper lifecycle management
	if err := c.ensureTMCLifecycle(ctx, apiResourceSchema); err != nil {
		return fmt.Errorf("failed to ensure TMC lifecycle: %w", err)
	}

	logger.V(2).Info("successfully reconciled TMC APIResourceSchema")
	return nil
}

// isTMCAPIResourceSchema determines if an APIResourceSchema is related to TMC.
func (c *controller) isTMCAPIResourceSchema(apiResourceSchema *apisv1alpha1.APIResourceSchema) bool {
	// Check if the group is TMC-related
	return apiResourceSchema.Spec.Group == "tmc.kcp.io"
}

// validateTMCAPIResourceSchema validates TMC-specific requirements for APIResourceSchemas.
func (c *controller) validateTMCAPIResourceSchema(ctx context.Context, apiResourceSchema *apisv1alpha1.APIResourceSchema) error {
	logger := klog.FromContext(ctx)

	// Validate TMC feature flag is enabled
	if !features.DefaultFeatureGate.Enabled(features.TMC) {
		return fmt.Errorf("TMC feature is not enabled")
	}

	// Validate group
	if apiResourceSchema.Spec.Group != "tmc.kcp.io" {
		return fmt.Errorf("invalid group for TMC APIResourceSchema: %s", apiResourceSchema.Spec.Group)
	}

	// Validate scope based on resource type
	expectedScope := c.getExpectedScopeForTMCResource(apiResourceSchema.Spec.Names.Kind)
	if apiResourceSchema.Spec.Scope != expectedScope {
		return fmt.Errorf("invalid scope %s for TMC resource %s, expected %s",
			apiResourceSchema.Spec.Scope, apiResourceSchema.Spec.Names.Kind, expectedScope)
	}

	// Validate that required fields are present
	if len(apiResourceSchema.Spec.Versions) == 0 {
		return fmt.Errorf("TMC APIResourceSchema must have at least one version")
	}

	logger.V(4).Info("TMC APIResourceSchema validation passed")
	return nil
}

// getExpectedScopeForTMCResource returns the expected scope for a TMC resource kind.
func (c *controller) getExpectedScopeForTMCResource(kind string) string {
	switch kind {
	case "ClusterRegistration":
		// ClusterRegistration is cluster-scoped as it represents physical clusters
		return "Cluster"
	case "WorkloadPlacement":
		// WorkloadPlacement is namespace-scoped as it applies to specific workloads
		return "Namespaced"
	default:
		// Default to namespaced for safety
		return "Namespaced"
	}
}

// ensureTMCLifecycle ensures proper lifecycle management for TMC APIResourceSchemas.
func (c *controller) ensureTMCLifecycle(ctx context.Context, apiResourceSchema *apisv1alpha1.APIResourceSchema) error {
	logger := klog.FromContext(ctx)

	// For TMC APIResourceSchemas, we want to ensure they have proper ownership
	// and are managed by the TMC controller system.
	
	// Check if the APIResourceSchema has proper labels
	if apiResourceSchema.Labels == nil {
		apiResourceSchema.Labels = make(map[string]string)
	}

	// Add TMC management labels
	expectedLabels := map[string]string{
		"app.kubernetes.io/managed-by": "kcp-tmc",
		"tmc.kcp.io/managed":           "true",
	}

	needsUpdate := false
	for key, expectedValue := range expectedLabels {
		if currentValue, exists := apiResourceSchema.Labels[key]; !exists || currentValue != expectedValue {
			apiResourceSchema.Labels[key] = expectedValue
			needsUpdate = true
		}
	}

	// Update the APIResourceSchema if labels were added
	if needsUpdate {
		logger.V(2).Info("updating TMC APIResourceSchema labels")
		clusterName := logicalcluster.From(apiResourceSchema)
		_, err := c.kcpClusterClient.Cluster(clusterName.Path()).ApisV1alpha1().
			APIResourceSchemas().Update(ctx, apiResourceSchema, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update TMC APIResourceSchema labels: %w", err)
		}
	}

	return nil
}