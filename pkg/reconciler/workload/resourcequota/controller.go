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

package resourcequota

import (
	"context"
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	kcpcorev1informers "github.com/kcp-dev/client-go/informers/core/v1"
	kcpkubernetesclientset "github.com/kcp-dev/client-go/kubernetes"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/logging"
)

const (
	// ControllerName is the name of this controller
	ControllerName = "kcp-resourcequota"
)

// Controller manages ResourceQuota enforcement and usage calculation.
type Controller struct {
	kubeClusterClient kcpkubernetesclientset.ClusterInterface
	workspace         logicalcluster.Name
	podInformer       kcpcorev1informers.PodClusterInformer
	pvcInformer       kcpcorev1informers.PersistentVolumeClaimClusterInformer
	queue             workqueue.TypedRateLimitingInterface[string]
}

// NewController creates a new ResourceQuota controller.
func NewController(
	kubeClusterClient kcpkubernetesclientset.ClusterInterface,
	podInformer kcpcorev1informers.PodClusterInformer,
	pvcInformer kcpcorev1informers.PersistentVolumeClaimClusterInformer,
	workspace logicalcluster.Name,
) *Controller {
	c := &Controller{
		kubeClusterClient: kubeClusterClient,
		workspace:         workspace,
		podInformer:       podInformer,
		pvcInformer:       pvcInformer,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: ControllerName},
		),
	}

	_, _ = podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.enqueuePod,
		UpdateFunc: func(old, new interface{}) { c.enqueuePod(new) },
		DeleteFunc: c.enqueuePod,
	})

	_, _ = pvcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.enqueuePVC,
		UpdateFunc: func(old, new interface{}) { c.enqueuePVC(new) },
		DeleteFunc: c.enqueuePVC,
	})

	return c
}

// Start begins the controller work loops.
func (c *Controller) Start(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger := logging.WithReconciler(klog.FromContext(ctx), ControllerName)
	logger.Info("Starting ResourceQuota controller")
	defer logger.Info("Shutting down ResourceQuota controller")

	if !cache.WaitForCacheSync(ctx.Done(), 
		c.podInformer.Informer().HasSynced,
		c.pvcInformer.Informer().HasSynced) {
		logger.Error(nil, "Failed to sync caches")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}
	<-ctx.Done()
}

// worker processes items from the queue.
func (c *Controller) worker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

// processNextItem handles one item from the queue.
func (c *Controller) processNextItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	if err := c.reconcile(ctx, key); err == nil {
		c.queue.Forget(key)
	} else {
		runtime.HandleError(fmt.Errorf("error reconciling %v: %v", key, err))
		c.queue.AddRateLimited(key)
	}
	return true
}

// reconcile processes namespace for quota checking.
func (c *Controller) reconcile(ctx context.Context, key string) error {
	namespace := key // Simplified - would parse key in real implementation
	usage, err := c.calculateNamespaceUsage(ctx, namespace)
	if err != nil {
		return err
	}

	klog.V(4).Infof("Calculated usage for namespace %s: %v", namespace, usage)
	return nil
}

// calculateNamespaceUsage aggregates resource usage from pods and PVCs.
func (c *Controller) calculateNamespaceUsage(ctx context.Context, namespace string) (corev1.ResourceList, error) {
	usage := make(corev1.ResourceList)
	
	if err := c.calculatePodUsage(namespace, usage); err != nil {
		return nil, err
	}
	if err := c.calculatePVCUsage(namespace, usage); err != nil {
		return nil, err
	}
	return usage, nil
}

// calculatePodUsage aggregates resource usage from running pods.
func (c *Controller) calculatePodUsage(namespace string, usage corev1.ResourceList) error {
	pods, err := c.podInformer.Lister().Cluster(c.workspace).Pods(namespace).List(labels.Everything())
	if err != nil {
		return err
	}

	podCount := int64(0)
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		podCount++

		for _, container := range pod.Spec.Containers {
			c.addResourceRequests(usage, container.Resources.Requests)
		}
	}

	if podCount > 0 {
		usage[corev1.ResourcePods] = *resource.NewQuantity(podCount, resource.DecimalSI)
	}
	return nil
}

// calculatePVCUsage aggregates storage usage from bound PVCs.
func (c *Controller) calculatePVCUsage(namespace string, usage corev1.ResourceList) error {
	pvcs, err := c.pvcInformer.Lister().Cluster(c.workspace).PersistentVolumeClaims(namespace).List(labels.Everything())
	if err != nil {
		return err
	}

	pvcCount := int64(0)
	totalStorage := resource.NewQuantity(0, resource.BinarySI)

	for _, pvc := range pvcs {
		if pvc.Status.Phase != corev1.ClaimBound {
			continue
		}
		pvcCount++

		if storage, exists := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; exists {
			totalStorage.Add(storage)
		}
	}

	if pvcCount > 0 {
		usage[corev1.ResourcePersistentVolumeClaims] = *resource.NewQuantity(pvcCount, resource.DecimalSI)
	}
	if !totalStorage.IsZero() {
		usage[corev1.ResourceRequestsStorage] = *totalStorage
	}
	return nil
}

// addResourceRequests adds resource requests to usage map.
func (c *Controller) addResourceRequests(usage corev1.ResourceList, requests corev1.ResourceList) {
	for name, quantity := range requests {
		resourceName := name
		switch name {
		case corev1.ResourceCPU:
			resourceName = corev1.ResourceRequestsCPU
		case corev1.ResourceMemory:
			resourceName = corev1.ResourceRequestsMemory
		}

		if current, exists := usage[resourceName]; exists {
			current.Add(quantity)
			usage[resourceName] = current
		} else {
			usage[resourceName] = quantity.DeepCopy()
		}
	}
}

// enqueuePod handles pod events.
func (c *Controller) enqueuePod(obj interface{}) {
	if pod, ok := obj.(*corev1.Pod); ok {
		c.queue.Add(pod.Namespace)
	}
}

// enqueuePVC handles PVC events.
func (c *Controller) enqueuePVC(obj interface{}) {
	if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
		c.queue.Add(pvc.Namespace)
	}
}

// CheckViolations identifies quota violations.
func CheckViolations(spec workloadv1alpha1.ResourceQuotaSpec, used corev1.ResourceList) []string {
	var violations []string
	for resourceName, hardLimit := range spec.Hard {
		if usedQuantity, exists := used[resourceName]; exists {
			if usedQuantity.Cmp(hardLimit) > 0 {
				violations = append(violations, fmt.Sprintf(
					"%s: used %s exceeds limit %s",
					resourceName, usedQuantity.String(), hardLimit.String()))
			}
		}
	}
	return violations
}

// TODO: Admission webhook and comprehensive quota object reconciliation
// deferred to follow-up PR to keep this PR focused on core logic.