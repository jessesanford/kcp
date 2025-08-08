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

package registration

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"

	"github.com/kcp-dev/kcp/pkg/logging"
	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	tmcclientset "github.com/kcp-dev/kcp/pkg/client/tmc/clientset/versioned/cluster"
	tmcv1alpha1client "github.com/kcp-dev/kcp/pkg/client/tmc/clientset/versioned/typed/tmc/v1alpha1"
	tmcinformers "github.com/kcp-dev/kcp/pkg/client/tmc/informers/externalversions/tmc/v1alpha1"
	tmclisters "github.com/kcp-dev/kcp/pkg/client/tmc/listers/tmc/v1alpha1"
)

const (
	ControllerName = "cluster-registration"
)

// NewController creates a new ClusterRegistration controller following KCP patterns.
// It integrates with the APIExport system and maintains workspace isolation.
//
// Parameters:
//   - tmcClusterClient: Cluster-aware TMC client
//   - clusterRegistrationInformer: Shared informer for ClusterRegistration resources
//
// Returns:
//   - *Controller: Configured controller ready to start
//   - error: Configuration or setup error
func NewController(
	tmcClusterClient tmcclientset.ClusterInterface,
	clusterRegistrationInformer tmcinformers.ClusterRegistrationClusterInformer,
) (*Controller, error) {
	c := &Controller{
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{
				Name: ControllerName,
			},
		),

		tmcClusterClient: tmcClusterClient,

		clusterRegistrationIndexer: clusterRegistrationInformer.Informer().GetIndexer(),
		clusterRegistrationLister:  clusterRegistrationInformer.Lister(),

		commit: committer.NewCommitter[*tmcv1alpha1.ClusterRegistration, tmcv1alpha1client.ClusterRegistrationInterface, *tmcv1alpha1.ClusterRegistrationSpec, *tmcv1alpha1.ClusterRegistrationStatus](tmcClusterClient.TmcV1alpha1().ClusterRegistrations()),
	}

	_, _ = clusterRegistrationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.enqueue(obj) },
		UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
		DeleteFunc: func(obj interface{}) { c.enqueue(obj) },
	})

	return c, nil
}

type clusterRegistrationResource = committer.Resource[*tmcv1alpha1.ClusterRegistrationSpec, *tmcv1alpha1.ClusterRegistrationStatus]

// Controller watches ClusterRegistration resources and manages cluster lifecycle.
// It handles registration reconciliation, connection validation, and status management.
type Controller struct {
	queue workqueue.TypedRateLimitingInterface[string]

	tmcClusterClient tmcclientset.ClusterInterface

	clusterRegistrationIndexer cache.Indexer
	clusterRegistrationLister  tmclisters.ClusterRegistrationClusterLister

	// commit creates a patch and submits it, if needed.
	commit func(ctx context.Context, old, new *clusterRegistrationResource) error
}

func (c *Controller) enqueue(obj interface{}) {
	key, err := kcpcache.MetaClusterNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	logger := logging.WithQueueKey(logging.WithReconciler(klog.Background(), ControllerName), key)
	logger.V(4).Info("queueing ClusterRegistration")
	c.queue.Add(key)
}

func (c *Controller) Start(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	ctx = klog.NewContext(ctx, logger)
	logger.Info("Starting controller")
	defer logger.Info("Shutting down controller")

	for range numThreads {
		go wait.Until(func() { c.startWorker(ctx) }, time.Second, ctx.Done())
	}

	<-ctx.Done()
}

func (c *Controller) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
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

	if requeue, err := c.process(ctx, key); err != nil {
		utilruntime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", ControllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	} else if requeue {
		// only requeue if we didn't error, but we still want to requeue
		c.queue.Add(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *Controller) process(ctx context.Context, key string) (bool, error) {
	clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return false, nil
	}
	
	clusterRegistration, err := c.clusterRegistrationLister.Cluster(clusterName).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil // object deleted before we handled it
		}
		return false, err
	}

	old := clusterRegistration
	clusterRegistration = clusterRegistration.DeepCopy()

	logger := logging.WithObject(klog.FromContext(ctx), clusterRegistration)
	ctx = klog.NewContext(ctx, logger)

	var errs []error
	requeue, err := c.reconcile(ctx, clusterRegistration)
	if err != nil {
		errs = append(errs, err)
	}

	// If the object being reconciled changed as a result, update it.
	oldResource := &clusterRegistrationResource{ObjectMeta: old.ObjectMeta, Spec: &old.Spec, Status: &old.Status}
	newResource := &clusterRegistrationResource{ObjectMeta: clusterRegistration.ObjectMeta, Spec: &clusterRegistration.Spec, Status: &clusterRegistration.Status}
	if err := c.commit(ctx, oldResource, newResource); err != nil {
		errs = append(errs, err)
	}

	return requeue, utilerrors.NewAggregate(errs)
}