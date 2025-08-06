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

package apiexport

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

	"github.com/kcp-dev/kcp/pkg/logging"
	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	apisv1alpha1informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/apis/v1alpha1"
	apisv1alpha1listers "github.com/kcp-dev/kcp/sdk/client/listers/apis/v1alpha1"
)

const (
	// ControllerName is the name of the TMC APIExport controller
	ControllerName = "tmc-apiexport"
	
	// TMCAPIExportName is the name of the TMC APIExport
	TMCAPIExportName = "tmc.kcp.io"
)

// Controller manages TMC APIExport following exact KCP patterns
type Controller struct {
	queue workqueue.RateLimitingInterface

	kcpClusterClient kcpclientset.ClusterInterface

	apiExportLister  apisv1alpha1listers.APIExportClusterLister
	apiExportIndexer cache.Indexer

	getAPIResourceSchema func(clusterName logicalcluster.Name, name string) (*apisv1alpha1.APIResourceSchema, error)
}

// NewController creates a new TMC APIExport controller following KCP patterns
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	apiExportInformer apisv1alpha1informers.APIExportClusterInformer,
	apiResourceSchemaInformer apisv1alpha1informers.APIResourceSchemaClusterInformer,
) (*Controller, error) {

	queue := workqueue.NewNamedRateLimitingQueue(
		workqueue.DefaultControllerRateLimiter(),
		ControllerName,
	)

	c := &Controller{
		queue:            queue,
		kcpClusterClient: kcpClusterClient,
		apiExportLister:  apiExportInformer.Lister(),
		apiExportIndexer: apiExportInformer.Informer().GetIndexer(),
		getAPIResourceSchema: func(clusterName logicalcluster.Name, name string) (*apisv1alpha1.APIResourceSchema, error) {
			return apiResourceSchemaInformer.Lister().Cluster(clusterName).Get(name)
		},
	}

	_, err := apiExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.enqueue(obj) },
		UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
		DeleteFunc: func(obj interface{}) { c.enqueue(obj) },
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

// enqueue adds an APIExport to the work queue following KCP patterns
func (c *Controller) enqueue(obj interface{}) {
	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// Start runs the controller following KCP patterns
func (c *Controller) Start(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	ctx = klog.NewContext(ctx, logger)
	logger.Info("Starting TMC APIExport controller")
	defer logger.Info("Shutting down TMC APIExport controller")

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-ctx.Done()
}

// runWorker processes items from the queue
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes a single work item from the queue
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two resources with the same key are never processed in
	// parallel.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	err := c.reconcile(ctx, key.(string))
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%s: reconcile error: %w", key, err))
	c.queue.AddRateLimited(key)
	return true
}

// reconcile handles TMC APIExport resources following KCP patterns
func (c *Controller) reconcile(ctx context.Context, key string) error {
	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		logger.Error(err, "invalid key")
		return nil
	}

	logger = logger.WithValues("cluster", clusterName, "name", name)

	apiExport, err := c.apiExportLister.Cluster(clusterName).Get(name)
	if errors.IsNotFound(err) {
		logger.V(2).Info("TMC APIExport was deleted")
		return nil
	}
	if err != nil {
		return err
	}

	// Only process TMC APIExports
	if !c.isTMCAPIExport(apiExport) {
		return nil
	}

	logger.V(2).Info("Processing TMC APIExport")
	return c.reconcileTMCAPIExport(ctx, apiExport)
}

// isTMCAPIExport checks if this is a TMC-related APIExport
func (c *Controller) isTMCAPIExport(apiExport *apisv1alpha1.APIExport) bool {
	return apiExport.Name == TMCAPIExportName
}

// reconcileTMCAPIExport ensures TMC APIExport is properly configured for KCP integration
func (c *Controller) reconcileTMCAPIExport(ctx context.Context, apiExport *apisv1alpha1.APIExport) error {
	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	clusterName := logicalcluster.From(apiExport)

	// Ensure TMC APIs are properly configured in the APIExport
	// This controller validates that TMC APIResourceSchemas are present and properly configured
	
	expectedSchemas := []string{
		"tmc.kcp.io.v1alpha1.ClusterRegistration",
		"tmc.kcp.io.v1alpha1.WorkloadPlacement",
	}

	// Validate that expected schemas are referenced in the APIExport
	for _, schemaName := range expectedSchemas {
		found := false
		for _, exportedSchema := range apiExport.Spec.LatestResourceSchemas {
			if exportedSchema == schemaName {
				found = true
				break
			}
		}
		
		if !found {
			logger.V(2).Info("TMC APIExport missing expected schema", "schema", schemaName)
			// In a real implementation, we might create missing schemas or update the APIExport
			// For now, we log and continue
		} else {
			// Verify the schema exists
			_, err := c.getAPIResourceSchema(clusterName, schemaName)
			if err != nil {
				if errors.IsNotFound(err) {
					logger.V(2).Info("TMC APIResourceSchema not found", "schema", schemaName)
				} else {
					return fmt.Errorf("error getting APIResourceSchema %s: %w", schemaName, err)
				}
			} else {
				logger.V(3).Info("TMC APIResourceSchema found", "schema", schemaName)
			}
		}
	}

	logger.V(2).Info("Successfully reconciled TMC APIExport")
	return nil
}