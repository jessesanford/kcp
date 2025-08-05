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

// Package controller implements the WorkloadPlacement controller for TMC.
package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/placement/engine"
)

const (
	// WorkloadPlacementControllerName is the name of the controller
	WorkloadPlacementControllerName = "workload-placement-controller"

	// ControllerFinalizer is the finalizer added to WorkloadPlacement resources
	ControllerFinalizer = "workloadplacement.tmc.kcp.io/controller"
)

// WorkloadPlacementController manages WorkloadPlacement resources and makes placement decisions.
type WorkloadPlacementController struct {
	// name is the controller name
	name string

	// placementEngine handles placement algorithm decisions
	placementEngine *engine.SimplePlacementEngine

	// clusterProvider provides access to cluster information
	clusterProvider ClusterProvider

	// workQueue is the work queue for processing WorkloadPlacement resources
	workQueue workqueue.RateLimitingInterface

	// placementLister provides cached access to WorkloadPlacement resources
	placementLister WorkloadPlacementLister

	// clusterLister provides cached access to ClusterRegistration resources
	clusterLister ClusterRegistrationLister

	// eventRecorder records events for WorkloadPlacement resources
	eventRecorder EventRecorder
}

// WorkloadPlacementLister provides read access to WorkloadPlacement resources.
type WorkloadPlacementLister interface {
	Get(name string) (*tmcv1alpha1.WorkloadPlacement, error)
	List() ([]*tmcv1alpha1.WorkloadPlacement, error)
}

// ClusterRegistrationLister provides read access to ClusterRegistration resources.
type ClusterRegistrationLister interface {
	Get(name string) (*tmcv1alpha1.ClusterRegistration, error)
	List() ([]*tmcv1alpha1.ClusterRegistration, error)
}

// EventRecorder records events for Kubernetes resources.
type EventRecorder interface {
	Event(object runtime.Object, eventtype, reason, message string)
	Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{})
}

// ClusterProvider provides cluster information for placement decisions.
type ClusterProvider interface {
	GetAvailableClusters(ctx context.Context) ([]*engine.ClusterInfo, error)
}

// NewWorkloadPlacementController creates a new WorkloadPlacement controller.
func NewWorkloadPlacementController(
	placementLister WorkloadPlacementLister,
	clusterLister ClusterRegistrationLister,
	clusterProvider ClusterProvider,
	eventRecorder EventRecorder,
) (*WorkloadPlacementController, error) {

	placementEngine := engine.NewSimplePlacementEngine(clusterProvider)

	controller := &WorkloadPlacementController{
		name:            WorkloadPlacementControllerName,
		placementEngine: placementEngine,
		clusterProvider: clusterProvider,
		workQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), WorkloadPlacementControllerName),
		placementLister: placementLister,
		clusterLister:   clusterLister,
		eventRecorder:   eventRecorder,
	}

	return controller, nil
}

// Run starts the controller and blocks until the context is cancelled.
func (c *WorkloadPlacementController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workQueue.ShutDown()

	klog.InfoS("Starting controller", "controller", c.name)
	defer klog.InfoS("Stopping controller", "controller", c.name)

	klog.InfoS("Starting workers", "controller", c.name, "workers", workers)
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	<-ctx.Done()
	return nil
}

// runWorker processes work items from the queue.
func (c *WorkloadPlacementController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes the next work item from the queue.
func (c *WorkloadPlacementController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.workQueue.Get()
	if shutdown {
		return false
	}

	defer c.workQueue.Done(obj)

	key, ok := obj.(string)
	if !ok {
		c.workQueue.Forget(obj)
		utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
		return true
	}

	err := c.syncWorkloadPlacement(ctx, key)
	if err == nil {
		c.workQueue.Forget(obj)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("error syncing WorkloadPlacement %q: %w", key, err))

	if c.workQueue.NumRequeues(obj) < 5 {
		klog.V(2).InfoS("Retrying WorkloadPlacement sync", "key", key, "error", err)
		c.workQueue.AddRateLimited(obj)
		return true
	}

	c.workQueue.Forget(obj)
	utilruntime.HandleError(fmt.Errorf("dropping WorkloadPlacement %q out of the queue: %w", key, err))
	return true
}

// syncWorkloadPlacement reconciles a single WorkloadPlacement resource.
func (c *WorkloadPlacementController) syncWorkloadPlacement(ctx context.Context, key string) error {
	klog.V(4).InfoS("Syncing WorkloadPlacement", "key", key)

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid key %q: %w", key, err)
	}

	placement, err := c.placementLister.Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("WorkloadPlacement not found, skipping", "key", key)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get WorkloadPlacement %q: %w", key, err)
	}

	// Create a copy to avoid mutating the cache
	placement = placement.DeepCopy()

	return c.reconcileWorkloadPlacement(ctx, placement)
}

// reconcileWorkloadPlacement performs the main reconciliation logic.
func (c *WorkloadPlacementController) reconcileWorkloadPlacement(ctx context.Context, placement *tmcv1alpha1.WorkloadPlacement) error {
	klog.V(2).InfoS("Reconciling WorkloadPlacement", 
		"name", placement.Name, 
		"namespace", placement.Namespace,
		"policy", placement.Spec.PlacementPolicy)

	// Handle deletion
	if !placement.DeletionTimestamp.IsZero() {
		return c.handleDeletion(ctx, placement)
	}

	// Ensure finalizer is present
	if !c.hasFinalizer(placement) {
		c.addFinalizer(placement)
		return c.updatePlacement(ctx, placement)
	}

	// Perform placement decision
	err := c.performPlacement(ctx, placement)
	if err != nil {
		c.eventRecorder.Eventf(placement, corev1.EventTypeWarning, "PlacementFailed", 
			"Failed to make placement decision: %v", err)
		return c.updatePlacementStatus(ctx, placement, corev1.ConditionFalse, 
			"PlacementFailed", err.Error())
	}

	c.eventRecorder.Event(placement, corev1.EventTypeNormal, "PlacementSucceeded", 
		"Successfully determined cluster placement")
	return c.updatePlacementStatus(ctx, placement, corev1.ConditionTrue, 
		"PlacementSucceeded", "Placement decision completed successfully")
}

// performPlacement executes the placement algorithm and updates the placement status.
func (c *WorkloadPlacementController) performPlacement(ctx context.Context, placement *tmcv1alpha1.WorkloadPlacement) error {
	// Build placement request
	request := &engine.PlacementRequest{
		Policy:            placement.Spec.PlacementPolicy,
		RequestedClusters: 1, // Default to 1 cluster
	}
	
	// Set location filter if specified
	if len(placement.Spec.ClusterSelector.LocationSelector) > 0 {
		request.LocationFilter = placement.Spec.ClusterSelector.LocationSelector[0]
	}

	// Use more clusters if specified in affinity or other selectors
	if len(placement.Spec.ClusterSelector.ClusterNames) > 0 {
		request.RequestedClusters = len(placement.Spec.ClusterSelector.ClusterNames)
	}

	// Call placement engine
	result, err := c.placementEngine.SelectClusters(ctx, request)
	if err != nil {
		return fmt.Errorf("placement engine failed: %w", err)
	}

	if len(result.SelectedClusters) == 0 {
		return fmt.Errorf("no clusters selected for placement")
	}

	// Update placement status with results
	placement.Status.SelectedClusters = result.SelectedClusters
	placement.Status.LastPlacementTime = &metav1.Time{Time: time.Now()}

	// Add placement history entry
	historyEntry := tmcv1alpha1.PlacementHistoryEntry{
		Timestamp:        metav1.Time{Time: time.Now()},
		Policy:           placement.Spec.PlacementPolicy,
		SelectedClusters: result.SelectedClusters,
		Reason:           result.Reason,
	}

	// Keep only last 10 history entries
	placement.Status.PlacementHistory = append(placement.Status.PlacementHistory, historyEntry)
	if len(placement.Status.PlacementHistory) > 10 {
		placement.Status.PlacementHistory = placement.Status.PlacementHistory[len(placement.Status.PlacementHistory)-10:]
	}

	klog.V(2).InfoS("Placement decision completed",
		"placement", placement.Name,
		"policy", placement.Spec.PlacementPolicy,
		"selectedClusters", result.SelectedClusters,
		"reason", result.Reason)

	return nil
}

// handleDeletion handles the deletion of a WorkloadPlacement resource.
func (c *WorkloadPlacementController) handleDeletion(ctx context.Context, placement *tmcv1alpha1.WorkloadPlacement) error {
	klog.V(2).InfoS("Handling WorkloadPlacement deletion", "name", placement.Name)

	// Perform cleanup if needed
	// For now, just remove the finalizer
	c.removeFinalizer(placement)
	return c.updatePlacement(ctx, placement)
}

// updatePlacementStatus updates the status of a WorkloadPlacement resource.
func (c *WorkloadPlacementController) updatePlacementStatus(ctx context.Context, placement *tmcv1alpha1.WorkloadPlacement, status corev1.ConditionStatus, reason, message string) error {
	// Update Ready condition
	condition := conditionsv1alpha1.Condition{
		Type:               conditionsv1alpha1.ReadyCondition,
		Status:             status,
		LastTransitionTime: metav1.Time{Time: time.Now()},
		Reason:             reason,
		Message:            message,
	}

	// Update or add condition
	c.setCondition(&placement.Status.Conditions, condition)

	return c.updatePlacement(ctx, placement)
}

// updatePlacement updates a WorkloadPlacement resource.
func (c *WorkloadPlacementController) updatePlacement(ctx context.Context, placement *tmcv1alpha1.WorkloadPlacement) error {
	// This would typically use a client to update the resource
	// For now, we'll just log the update
	klog.V(4).InfoS("Updating WorkloadPlacement", "name", placement.Name, "namespace", placement.Namespace)
	return nil
}

// hasFinalizer checks if the placement has the controller finalizer.
func (c *WorkloadPlacementController) hasFinalizer(placement *tmcv1alpha1.WorkloadPlacement) bool {
	for _, finalizer := range placement.Finalizers {
		if finalizer == ControllerFinalizer {
			return true
		}
	}
	return false
}

// addFinalizer adds the controller finalizer to the placement.
func (c *WorkloadPlacementController) addFinalizer(placement *tmcv1alpha1.WorkloadPlacement) {
	placement.Finalizers = append(placement.Finalizers, ControllerFinalizer)
}

// removeFinalizer removes the controller finalizer from the placement.
func (c *WorkloadPlacementController) removeFinalizer(placement *tmcv1alpha1.WorkloadPlacement) {
	var finalizers []string
	for _, finalizer := range placement.Finalizers {
		if finalizer != ControllerFinalizer {
			finalizers = append(finalizers, finalizer)
		}
	}
	placement.Finalizers = finalizers
}

// AddToQueue adds a WorkloadPlacement to the controller's work queue.
func (c *WorkloadPlacementController) AddToQueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to get key for object %#v: %w", obj, err))
		return
	}
	c.workQueue.Add(key)
}

// GetName returns the controller name.
func (c *WorkloadPlacementController) GetName() string {
	return c.name
}

// setCondition adds or updates a condition in the conditions list.
func (c *WorkloadPlacementController) setCondition(conditions *conditionsv1alpha1.Conditions, newCondition conditionsv1alpha1.Condition) {
	if conditions == nil {
		conditions = &conditionsv1alpha1.Conditions{}
	}

	existingCondition := c.findCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		*conditions = append(*conditions, newCondition)
	} else {
		existingCondition.Status = newCondition.Status
		existingCondition.LastTransitionTime = newCondition.LastTransitionTime
		existingCondition.Reason = newCondition.Reason
		existingCondition.Message = newCondition.Message
	}
}

// findCondition finds a condition by type in the conditions list.
func (c *WorkloadPlacementController) findCondition(conditions conditionsv1alpha1.Conditions, conditionType conditionsv1alpha1.ConditionType) *conditionsv1alpha1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}