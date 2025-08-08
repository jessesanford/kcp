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

package autoscaling

import (
	"context"
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	tmclisters "github.com/kcp-dev/kcp/sdk/client/listers/tmc/v1alpha1"
)

const (
	// ControllerName is the name of this controller
	ControllerName = "tmc-hpa-controller"

	// DefaultSyncPeriod is how often we re-evaluate HPA policies
	DefaultSyncPeriod = 30 * time.Second
)

// Controller manages HorizontalPodAutoscalerPolicy resources across logical clusters.
// It implements cluster-aware horizontal pod autoscaling that can distribute scaling
// decisions across multiple physical clusters based on placement policies.
type Controller struct {
	// kcpClusterClient provides cluster-aware access to TMC APIs
	kcpClusterClient kcpclientset.ClusterInterface

	// hpaLister can list/get HorizontalPodAutoscalerPolicy resources from the shared informer's store
	hpaLister tmclisters.HorizontalPodAutoscalerPolicyClusterLister

	// hpaListerSynced returns true if the HPA shared informer has synced at least once
	hpaListerSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens.
	workqueue workqueue.RateLimitingInterface

	// workspace is the logical cluster this controller operates within
	workspace logicalcluster.Name

	// syncPeriod is how often we re-evaluate HPA policies
	syncPeriod time.Duration

	// metricsCollector provides metrics collection capabilities
	metricsCollector MetricsCollector

	// scalingExecutor handles the actual scaling decisions
	scalingExecutor ScalingExecutor
}

// NewController creates a new TMC HPA controller.
//
// The controller watches HorizontalPodAutoscalerPolicy resources and implements
// cluster-aware horizontal pod autoscaling. It integrates with TMC's placement
// engine to distribute scaling decisions across multiple clusters.
//
// Parameters:
//   - kcpClusterClient: Cluster-aware KCP client for TMC APIs
//   - informerFactory: Shared informer factory for the workspace
//   - workspace: Logical cluster name for workspace isolation
//
// Returns:
//   - *Controller: Configured controller ready to start
//   - error: Configuration or setup error
func NewController(
	kcpClusterClient kcpclientset.ClusterInterface,
	informerFactory kcpinformers.SharedInformerFactory,
	workspace logicalcluster.Name,
) (*Controller, error) {
	
	// Create informer for HorizontalPodAutoscalerPolicy resources
	hpaInformer := informerFactory.Tmc().V1alpha1().HorizontalPodAutoscalerPolicies()

	controller := &Controller{
		kcpClusterClient: kcpClusterClient,
		hpaLister:        hpaInformer.Lister(),
		hpaListerSynced:  hpaInformer.Informer().HasSynced,
		workqueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		workspace:        workspace,
		syncPeriod:       DefaultSyncPeriod,
		metricsCollector: NewPrometheusMetricsCollector(),
		scalingExecutor:  NewDistributedScalingExecutor(kcpClusterClient, workspace),
	}

	klog.InfoS("Setting up HPA event handlers", "workspace", workspace)

	// Set up event handlers for HorizontalPodAutoscalerPolicy resources
	hpaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.enqueueHPA(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueHPA(new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueueHPA(obj)
		},
	})

	return controller, nil
}

// Run starts the controller and blocks until the context is cancelled.
//
// It waits for the informer caches to sync, then starts the specified number
// of worker goroutines to process work items from the queue. It also starts
// a periodic sync to re-evaluate all HPA policies.
func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.InfoS("Starting TMC HPA controller", "workspace", c.workspace)

	// Wait for the caches to be synced before starting workers
	klog.InfoS("Waiting for HPA controller caches to sync")
	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), c.hpaListerSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.InfoS("Starting HPA controller workers", "workers", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	// Start periodic sync for re-evaluating HPA policies
	go wait.UntilWithContext(ctx, c.periodicSync, c.syncPeriod)

	klog.InfoS("TMC HPA controller started")
	<-ctx.Done()
	klog.InfoS("Shutting down TMC HPA controller")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the workqueue.
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		// Run the syncHandler, passing it the namespace/name string of the
		// HorizontalPodAutoscalerPolicy resource to be synced.
		if err := c.syncHandler(ctx, key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.V(2).InfoS("Successfully synced HPA policy", "key", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the HorizontalPodAutoscalerPolicy
// resource with the current status of the resource.
func (c *Controller) syncHandler(ctx context.Context, key string) error {
	klog.V(4).InfoS("Processing HPA policy", "key", key)

	// Convert the namespace/name string into a distinct namespace and name
	clusterName, _, name, err := kcpcache.SplitMetaClusterNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the HorizontalPodAutoscalerPolicy resource with this namespace/name
	hpaPolicy, err := c.hpaLister.Cluster(clusterName).Get(name)
	if err != nil {
		// The HorizontalPodAutoscalerPolicy resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("HPA policy '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// Process the HPA policy
	return c.processHPAPolicy(ctx, hpaPolicy)
}

// processHPAPolicy handles the core logic for processing an HPA policy.
func (c *Controller) processHPAPolicy(ctx context.Context, hpaPolicy *tmcv1alpha1.HorizontalPodAutoscalerPolicy) error {
	klog.V(2).InfoS("Processing HPA policy", 
		"policy", hpaPolicy.Name, 
		"strategy", hpaPolicy.Spec.Strategy,
		"minReplicas", hpaPolicy.Spec.MinReplicas,
		"maxReplicas", hpaPolicy.Spec.MaxReplicas)

	// Collect current metrics from all relevant clusters
	metricsData, err := c.metricsCollector.CollectMetrics(ctx, hpaPolicy)
	if err != nil {
		klog.ErrorS(err, "Failed to collect metrics", "policy", hpaPolicy.Name)
		return c.updateHPAPolicyStatus(ctx, hpaPolicy, err, nil)
	}

	// Make scaling decision based on collected metrics
	scalingDecision, err := c.calculateScalingDecision(hpaPolicy, metricsData)
	if err != nil {
		klog.ErrorS(err, "Failed to calculate scaling decision", "policy", hpaPolicy.Name)
		return c.updateHPAPolicyStatus(ctx, hpaPolicy, err, metricsData)
	}

	// Execute scaling action if needed
	if scalingDecision.ShouldScale {
		err = c.scalingExecutor.ExecuteScaling(ctx, hpaPolicy, scalingDecision)
		if err != nil {
			klog.ErrorS(err, "Failed to execute scaling", "policy", hpaPolicy.Name)
			return c.updateHPAPolicyStatus(ctx, hpaPolicy, err, metricsData)
		}
		
		klog.V(1).InfoS("Scaling executed successfully", 
			"policy", hpaPolicy.Name,
			"currentReplicas", scalingDecision.CurrentReplicas,
			"desiredReplicas", scalingDecision.DesiredReplicas,
			"reason", scalingDecision.Reason)
	}

	// Update the status with current metrics and scaling information
	return c.updateHPAPolicyStatus(ctx, hpaPolicy, nil, metricsData)
}

// calculateScalingDecision determines if scaling is needed based on metrics.
func (c *Controller) calculateScalingDecision(
	hpaPolicy *tmcv1alpha1.HorizontalPodAutoscalerPolicy,
	metricsData *MetricsData,
) (*ScalingDecision, error) {
	
	// This is a simplified scaling algorithm
	// In production, this would implement sophisticated algorithms
	// including stability windows, scaling policies, etc.
	
	if len(metricsData.ClusterMetrics) == 0 {
		return &ScalingDecision{
			ShouldScale: false,
			Reason:      "No metrics available",
		}, nil
	}

	// Calculate desired replicas based on resource utilization
	totalCurrentReplicas := int32(0)
	totalResourceUtilization := float64(0)
	clusterCount := 0

	for _, clusterMetrics := range metricsData.ClusterMetrics {
		if clusterMetrics.CurrentReplicas != nil {
			totalCurrentReplicas += *clusterMetrics.CurrentReplicas
		}
		if clusterMetrics.ResourceUtilization != nil {
			totalResourceUtilization += *clusterMetrics.ResourceUtilization
			clusterCount++
		}
	}

	if clusterCount == 0 {
		return &ScalingDecision{
			ShouldScale: false,
			Reason:      "No resource utilization metrics available",
		}, nil
	}

	avgUtilization := totalResourceUtilization / float64(clusterCount)
	targetUtilization := float64(70) // Default target CPU utilization

	// Simple scaling algorithm: scale when utilization deviates significantly from target
	utilizationRatio := avgUtilization / targetUtilization
	desiredReplicas := int32(float64(totalCurrentReplicas) * utilizationRatio)

	// Apply min/max constraints
	minReplicas := int32(1)
	if hpaPolicy.Spec.MinReplicas != nil {
		minReplicas = *hpaPolicy.Spec.MinReplicas
	}
	
	if desiredReplicas < minReplicas {
		desiredReplicas = minReplicas
	}
	if desiredReplicas > hpaPolicy.Spec.MaxReplicas {
		desiredReplicas = hpaPolicy.Spec.MaxReplicas
	}

	shouldScale := desiredReplicas != totalCurrentReplicas
	reason := fmt.Sprintf("Current utilization: %.1f%%, target: %.1f%%", avgUtilization, targetUtilization)

	return &ScalingDecision{
		ShouldScale:      shouldScale,
		CurrentReplicas:  totalCurrentReplicas,
		DesiredReplicas:  desiredReplicas,
		Reason:          reason,
		ClusterDecisions: c.distributeReplicas(hpaPolicy, desiredReplicas, metricsData),
	}, nil
}

// distributeReplicas determines how to distribute replicas across clusters.
func (c *Controller) distributeReplicas(
	hpaPolicy *tmcv1alpha1.HorizontalPodAutoscalerPolicy,
	totalReplicas int32,
	metricsData *MetricsData,
) []ClusterScalingDecision {
	
	decisions := make([]ClusterScalingDecision, 0, len(metricsData.ClusterMetrics))
	
	// Simple even distribution for now
	// In production, this would consider cluster capacity, load, policies, etc.
	clustersCount := int32(len(metricsData.ClusterMetrics))
	if clustersCount == 0 {
		return decisions
	}

	replicasPerCluster := totalReplicas / clustersCount
	remainingReplicas := totalReplicas % clustersCount

	i := int32(0)
	for clusterName, clusterMetrics := range metricsData.ClusterMetrics {
		replicas := replicasPerCluster
		if i < remainingReplicas {
			replicas++
		}

		decisions = append(decisions, ClusterScalingDecision{
			ClusterName:     clusterName,
			CurrentReplicas: getInt32Value(clusterMetrics.CurrentReplicas),
			DesiredReplicas: replicas,
		})
		i++
	}

	return decisions
}

// periodicSync re-evaluates all HPA policies periodically.
func (c *Controller) periodicSync(ctx context.Context) {
	klog.V(4).InfoS("Running periodic HPA sync")

	hpaPolicies, err := c.hpaLister.Cluster(c.workspace).List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "Failed to list HPA policies for periodic sync")
		return
	}

	for _, hpaPolicy := range hpaPolicies {
		key, err := kcpcache.MetaClusterNamespaceKeyFunc(hpaPolicy)
		if err != nil {
			klog.ErrorS(err, "Failed to get key for HPA policy", "policy", hpaPolicy.Name)
			continue
		}
		c.workqueue.Add(key)
	}

	klog.V(4).InfoS("Periodic HPA sync completed", "policies", len(hpaPolicies))
}

// updateHPAPolicyStatus updates the status of an HPA policy.
func (c *Controller) updateHPAPolicyStatus(
	ctx context.Context,
	hpaPolicy *tmcv1alpha1.HorizontalPodAutoscalerPolicy,
	processError error,
	metricsData *MetricsData,
) error {
	
	// Create a copy to modify
	hpaPolicyCopy := hpaPolicy.DeepCopy()
	now := metav1.NewTime(time.Now())
	
	// Update observed generation
	hpaPolicyCopy.Status.ObservedGeneration = &hpaPolicy.Generation

	// Set conditions based on processing result
	if processError != nil {
		setHPAPolicyCondition(hpaPolicyCopy, tmcv1alpha1.HorizontalPodAutoscalerPolicyReady, metav1.ConditionFalse, "ProcessingError", processError.Error())
		setHPAPolicyCondition(hpaPolicyCopy, tmcv1alpha1.HorizontalPodAutoscalerPolicyActive, metav1.ConditionFalse, "ProcessingError", processError.Error())
	} else {
		setHPAPolicyCondition(hpaPolicyCopy, tmcv1alpha1.HorizontalPodAutoscalerPolicyReady, metav1.ConditionTrue, "ProcessingSuccessful", "HPA policy is processing successfully")
		setHPAPolicyCondition(hpaPolicyCopy, tmcv1alpha1.HorizontalPodAutoscalerPolicyActive, metav1.ConditionTrue, "Active", "HPA policy is actively making scaling decisions")
	}

	// Update metrics and cluster status if available
	if metricsData != nil {
		hpaPolicyCopy.Status.CurrentMetrics = convertToMetricStatus(metricsData)
		hpaPolicyCopy.Status.ClusterStatus = convertToClusterStatus(metricsData)
		
		// Calculate total replicas
		totalCurrent := int32(0)
		totalDesired := int32(0)
		for _, clusterMetrics := range metricsData.ClusterMetrics {
			if clusterMetrics.CurrentReplicas != nil {
				totalCurrent += *clusterMetrics.CurrentReplicas
			}
			if clusterMetrics.DesiredReplicas != nil {
				totalDesired += *clusterMetrics.DesiredReplicas
			}
		}
		
		hpaPolicyCopy.Status.CurrentReplicas = &totalCurrent
		hpaPolicyCopy.Status.DesiredReplicas = &totalDesired
	}

	// Update status
	_, err := c.kcpClusterClient.Cluster(c.workspace.Path()).TmcV1alpha1().
		HorizontalPodAutoscalerPolicies().
		UpdateStatus(ctx, hpaPolicyCopy, metav1.UpdateOptions{})
	
	return err
}

// enqueueHPA takes a HorizontalPodAutoscalerPolicy resource and converts it into a namespace/name
// string which is then put onto the work queue.
func (c *Controller) enqueueHPA(obj interface{}) {
	var key string
	var err error
	if key, err = kcpcache.MetaClusterNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// Helper functions

func getInt32Value(ptr *int32) int32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func setHPAPolicyCondition(
	hpaPolicy *tmcv1alpha1.HorizontalPodAutoscalerPolicy,
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	now := metav1.NewTime(time.Now())
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	// Find and update existing condition or append new one
	for i, existingCondition := range hpaPolicy.Status.Conditions {
		if existingCondition.Type == conditionType {
			if existingCondition.Status != status {
				hpaPolicy.Status.Conditions[i] = condition
			}
			return
		}
	}
	hpaPolicy.Status.Conditions = append(hpaPolicy.Status.Conditions, condition)
}

func convertToMetricStatus(metricsData *MetricsData) []tmcv1alpha1.MetricStatus {
	// Simplified conversion - in production this would aggregate metrics across clusters
	return []tmcv1alpha1.MetricStatus{}
}

func convertToClusterStatus(metricsData *MetricsData) []tmcv1alpha1.ClusterAutoScalingStatus {
	clusterStatus := make([]tmcv1alpha1.ClusterAutoScalingStatus, 0, len(metricsData.ClusterMetrics))
	
	for clusterName, metrics := range metricsData.ClusterMetrics {
		status := tmcv1alpha1.ClusterAutoScalingStatus{
			ClusterName:     clusterName,
			CurrentReplicas: metrics.CurrentReplicas,
			DesiredReplicas: metrics.DesiredReplicas,
			LastScaleTime:   metrics.LastScaleTime,
		}
		clusterStatus = append(clusterStatus, status)
	}
	
	return clusterStatus
}