/*
Copyright 2024 The KCP Authors.
*/

package cluster

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

// Controller manages ClusterRegistration resources
type Controller struct {
	queue                     workqueue.RateLimitingInterface
	kcpClient                kcpclientset.ClusterInterface
	kubeClient               kubernetes.Interface
	tmcClient                interface{}
	informer                 cache.SharedIndexInformer
	commitClusterRegistration func(context.Context, *ClusterRegistrationResource, *ClusterRegistrationResource) error
}

// ClusterRegistrationResource wraps a ClusterRegistration for committing
type ClusterRegistrationResource struct {
	*tmcv1alpha1.ClusterRegistration
}

// NewController creates a new TMC cluster controller
func NewController(
	kcpClient kcpclientset.ClusterInterface,
	kubeClient kubernetes.Interface,
	tmcClient interface{},
	informer cache.SharedIndexInformer,
) (*Controller, error) {
	c := &Controller{
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "tmc-cluster"),
		kcpClient:  kcpClient,
		kubeClient: kubeClient,
		tmcClient:  tmcClient,
		informer:   informer,
		commitClusterRegistration: func(ctx context.Context, old, new *ClusterRegistrationResource) error {
			// Stub implementation for testing
			return nil
		},
	}
	return c, nil
}

// enqueue adds a cluster to the work queue
func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.queue.Add(key)
}

// process handles a single cluster resource
func (c *Controller) process(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) error {
	// Stub implementation for testing
	return nil
}

// Len returns the queue length
func (c *Controller) Len() int {
	return c.queue.Len()
}
