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

package tmcexport

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"

	"github.com/kcp-dev/kcp/pkg/logging"
	apisv1alpha2 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha2"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	apisv1alpha2informers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/apis/v1alpha2"
	apisv1alpha2listers "github.com/kcp-dev/kcp/sdk/client/listers/apis/v1alpha2"
)

const (
	ControllerName = "kcp-tmc-apiexport"

	// TMCAPIExportName is the name of the TMC APIExport resource
	TMCAPIExportName = "tmc.kcp.io"
)

// NewController returns a new controller for TMC APIExport management.
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	apiExportInformer apisv1alpha2informers.APIExportClusterInformer,
) (*Controller, error) {
	c := &Controller{
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: ControllerName,
			},
		),

		kcpClusterClient: kcpClusterClient,
		apiExportLister:  apiExportInformer.Lister(),
	}


	logger := logging.WithReconciler(klog.Background(), ControllerName)

	_, _ = apiExportInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueAPIExport(obj.(*apisv1alpha2.APIExport), logger)
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueueAPIExport(obj.(*apisv1alpha2.APIExport), logger)
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueAPIExport(obj.(*apisv1alpha2.APIExport), logger)
		},
	})


	return c, nil
}

// Controller manages TMC APIExport resources for making TMC APIs available in workspaces.
type Controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	kcpClusterClient kcpclientset.ClusterInterface
	apiExportLister  apisv1alpha2listers.APIExportClusterLister
}

func (c *Controller) enqueueAPIExport(apiExport *apisv1alpha2.APIExport, logger logr.Logger) {
	// Only handle TMC APIExports
	if apiExport.Name != TMCAPIExportName {
		return
	}

	key, err := kcpcache.DeletionHandlingMetaClusterNamespaceKeyFunc(apiExport)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	logging.WithQueueKey(logger, key).V(4).Info("queueing TMC APIExport")
	c.queue.Add(key)
}


func (c *Controller) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// Start starts the controller, which stops when ctx.Done() is closed.
func (c *Controller) Start(ctx context.Context) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	ctx = klog.NewContext(ctx, logger)
	logger.Info("starting controller")
	defer logger.Info("shutting down controller")

	go wait.Until(func() { c.startWorker(ctx) }, time.Second, ctx.Done())

	<-ctx.Done()
}

func (c *Controller) ShutDown() {
	c.queue.ShutDown()
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	k, quit := c.queue.Get()
	if quit {
		return false
	}
	key := k

	logger := logging.WithQueueKey(klog.FromContext(ctx), key)
	ctx = klog.NewContext(ctx, logger)
	logger.V(4).Info("processing key")

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.process(ctx, key); err != nil {
		utilruntime.HandleError(fmt.Errorf("%s: failed to sync %q, err: %w", ControllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	}

	c.queue.Forget(key)
	return true
}

func (c *Controller) process(ctx context.Context, key string) error {
	clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	// Only handle TMC APIExports
	if name != TMCAPIExportName {
		return nil
	}

	logger := klog.FromContext(ctx).WithValues("clusterName", clusterName, "apiExportName", name)
	ctx = klog.NewContext(ctx, logger)

	apiExport, err := c.apiExportLister.Cluster(clusterName).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "error getting APIExport")
		return nil // nothing we can do here
	}

	if apiExport != nil {
		logger = logging.WithObject(logger, apiExport)
		ctx = klog.NewContext(ctx, logger)
	}

	return c.reconcile(ctx, apiExport, clusterName)
}