/*
Copyright 2023 The KCP Authors.

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

package canary

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	deploymentv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/deployment/v1alpha1"
	deploymentclient "github.com/kcp-dev/kcp/pkg/client/clientset/versioned"
	deploymentinformers "github.com/kcp-dev/kcp/pkg/client/informers/externalversions/deployment/v1alpha1"
	deploymentlisters "github.com/kcp-dev/kcp/pkg/client/listers/deployment/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/metrics"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

const (
	// ControllerName is the name of the canary controller
	ControllerName = "canary-controller"

	// DefaultResyncPeriod is the default resync period for the controller
	DefaultResyncPeriod = 30 * time.Second

	// DefaultConcurrency is the default number of workers
	DefaultConcurrency = 5
)

// Controller manages canary deployments with integrated metrics analysis and traffic management.
type Controller struct {
	// Kubernetes client
	kubeClient kubernetes.Interface

	// KCP deployment client
	deploymentClient deploymentclient.Interface

	// Listers and Informers
	canaryLister   deploymentlisters.CanaryDeploymentLister
	canaryInformer deploymentinformers.CanaryDeploymentInformer

	// Work queue
	workqueue workqueue.RateLimitingInterface

	// Components
	config           *CanaryConfiguration
	stateManager     StateManager
	metricsAnalyzer  MetricsAnalyzer
	trafficManager   TrafficManager
	promotionManager *PromotionManager

	// Control
	stopCh <-chan struct{}
}

// NewController creates a new canary deployment controller.
func NewController(
	kubeClient kubernetes.Interface,
	deploymentClient deploymentclient.Interface,
	canaryInformer deploymentinformers.CanaryDeploymentInformer,
	metricsRegistry *metrics.MetricsRegistry,
) (*Controller, error) {

	// Create configuration
	config := &CanaryConfiguration{
		MetricsRegistry:         metricsRegistry,
		DefaultAnalysisInterval: 60 * time.Second,
		DefaultStepDuration:     5 * time.Minute,
		DefaultSuccessThreshold: 95,
		MaxAnalysisAttempts:     5,
		EnableWebhookChecks:     false,
	}

	// Create metrics analyzer
	analyzer, err := NewMetricsAnalyzer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics analyzer: %w", err)
	}

	// Create traffic manager
	trafficManager := NewTrafficManager(kubeClient)

	// Create state manager
	stateManager := NewStateManager(config, analyzer, trafficManager)

	// Create promotion manager
	promotionManager := NewPromotionManager(kubeClient, trafficManager, analyzer)

	controller := &Controller{
		kubeClient:       kubeClient,
		deploymentClient: deploymentClient,
		canaryLister:     canaryInformer.Lister(),
		canaryInformer:   canaryInformer,
		workqueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			ControllerName,
		),
		config:           config,
		stateManager:     stateManager,
		metricsAnalyzer:  analyzer,
		trafficManager:   trafficManager,
		promotionManager: promotionManager,
	}

	klog.Info("Setting up event handlers for canary controller")

	// Set up event handlers
	canaryInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleAdd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.handleUpdate(oldObj, newObj)
		},
		DeleteFunc: controller.handleDelete,
	})

	return controller, nil
}

// Run starts the controller workers and blocks until stopCh is closed.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	c.stopCh = stopCh

	klog.Infof("Starting %s with %d workers", ControllerName, workers)

	// Wait for the caches to sync
	if !cache.WaitForCacheSync(stopCh, c.canaryInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Caches synced, starting workers")

	// Start workers
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		c.workqueue.Forget(obj)
		klog.V(4).Infof("Successfully synced '%s'", key)

		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler handles the reconciliation of a single canary deployment.
func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	klog.V(4).Infof("Syncing canary deployment %s/%s", namespace, name)

	// Get the canary deployment
	canary, err := c.canaryLister.CanaryDeployments(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("Canary deployment %s/%s no longer exists", namespace, name)
			return nil
		}
		return fmt.Errorf("failed to get canary deployment %s/%s: %w", namespace, name, err)
	}

	// Process the canary deployment
	return c.processCanaryDeployment(context.TODO(), canary.DeepCopy())
}

// processCanaryDeployment processes a single canary deployment through the state machine.
func (c *Controller) processCanaryDeployment(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) error {
	klog.V(3).Infof("Processing canary deployment %s/%s (phase: %s, step: %d)", 
		canary.Namespace, canary.Name, canary.Status.Phase, canary.Status.CurrentStep)

	// Validate the canary configuration
	if err := c.validateCanaryConfiguration(canary); err != nil {
		return c.updateCanaryStatus(ctx, canary, deploymentv1alpha1.CanaryPhaseFailed, fmt.Sprintf("Configuration validation failed: %v", err))
	}

	// Check if we should rollback
	if shouldRollback, reason, err := c.promotionManager.ShouldRollback(ctx, canary); err != nil {
		return fmt.Errorf("failed to check rollback conditions: %w", err)
	} else if shouldRollback {
		klog.V(2).Infof("Rolling back canary %s/%s: %s", canary.Namespace, canary.Name, reason)
		if err := c.promotionManager.RollbackCanary(ctx, canary); err != nil {
			return fmt.Errorf("failed to rollback canary: %w", err)
		}
		return c.updateCanaryWithStatus(ctx, canary)
	}

	// Check if we should transition to a new state
	if shouldTransition, newState, err := c.stateManager.ShouldTransition(ctx, canary); err != nil {
		return fmt.Errorf("failed to check state transition: %w", err)
	} else if shouldTransition {
		klog.V(3).Infof("Transitioning canary %s/%s from %s to %s", 
			canary.Namespace, canary.Name, canary.Status.Phase, newState.Phase)
		
		if err := c.stateManager.TransitionTo(ctx, canary, newState); err != nil {
			return fmt.Errorf("failed to transition state: %w", err)
		}
	}

	// Handle promotion logic
	if canary.Status.Phase == deploymentv1alpha1.CanaryPhasePromoting {
		if shouldPromote, err := c.promotionManager.ShouldPromote(ctx, canary); err != nil {
			return fmt.Errorf("failed to check promotion conditions: %w", err)
		} else if shouldPromote {
			klog.V(2).Infof("Promoting canary %s/%s", canary.Namespace, canary.Name)
			if err := c.promotionManager.PromoteCanary(ctx, canary); err != nil {
				return fmt.Errorf("failed to promote canary: %w", err)
			}
		}
	}

	// Update the canary status
	if err := c.updateCanaryWithStatus(ctx, canary); err != nil {
		return fmt.Errorf("failed to update canary status: %w", err)
	}

	// Requeue if the canary is still active
	if c.shouldRequeue(canary) {
		c.workqueue.AddAfter(fmt.Sprintf("%s/%s", canary.Namespace, canary.Name), DefaultResyncPeriod)
	}

	return nil
}

// validateCanaryConfiguration validates the canary deployment configuration.
func (c *Controller) validateCanaryConfiguration(canary *deploymentv1alpha1.CanaryDeployment) error {
	// Validate traffic configuration
	if err := c.trafficManager.ValidateTrafficConfig(canary); err != nil {
		return fmt.Errorf("traffic configuration validation failed: %w", err)
	}

	// Validate strategy
	if len(canary.Spec.Strategy.Steps) == 0 {
		return fmt.Errorf("strategy must have at least one step")
	}

	// Validate analysis configuration
	if len(canary.Spec.Analysis.MetricQueries) == 0 {
		klog.V(3).Infof("No metric queries configured for canary %s/%s, will use defaults", canary.Namespace, canary.Name)
	}

	return nil
}

// updateCanaryStatus updates the canary status with the given phase and message.
func (c *Controller) updateCanaryStatus(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, phase deploymentv1alpha1.CanaryPhase, message string) error {
	canary.Status.Phase = phase
	canary.Status.Message = message
	canary.Status.ObservedGeneration = canary.Generation

	// Update conditions
	now := metav1.Now()
	var conditionType string
	var conditionStatus metav1.ConditionStatus
	var reason string

	switch phase {
	case deploymentv1alpha1.CanaryPhaseSucceeded:
		conditionType = "Ready"
		conditionStatus = metav1.ConditionTrue
		reason = "CanarySucceeded"
	case deploymentv1alpha1.CanaryPhaseFailed:
		conditionType = "Ready"
		conditionStatus = metav1.ConditionFalse
		reason = "CanaryFailed"
	default:
		conditionType = "Progressing"
		conditionStatus = metav1.ConditionTrue
		reason = "CanaryProgressing"
	}

	conditionsv1alpha1.Set(&canary.Status.Conditions, conditionsv1alpha1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})

	return c.updateCanaryWithStatus(ctx, canary)
}

// updateCanaryWithStatus updates the canary deployment status in the API server.
func (c *Controller) updateCanaryWithStatus(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) error {
	_, err := c.deploymentClient.DeploymentV1alpha1().CanaryDeployments(canary.Namespace).UpdateStatus(ctx, canary, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update canary status: %w", err)
	}
	return nil
}

// shouldRequeue determines if the canary should be requeued for further processing.
func (c *Controller) shouldRequeue(canary *deploymentv1alpha1.CanaryDeployment) bool {
	switch canary.Status.Phase {
	case deploymentv1alpha1.CanaryPhaseSucceeded, deploymentv1alpha1.CanaryPhaseFailed:
		// Terminal states - no requeue needed
		return false
	default:
		// Active states - continue processing
		return true
	}
}

// Event handlers

// handleAdd handles add events for canary deployments.
func (c *Controller) handleAdd(obj interface{}) {
	canary, ok := obj.(*deploymentv1alpha1.CanaryDeployment)
	if !ok {
		runtime.HandleError(fmt.Errorf("expected CanaryDeployment, got %T", obj))
		return
	}

	klog.V(3).Infof("Added canary deployment %s/%s", canary.Namespace, canary.Name)
	c.enqueueCanary(canary)
}

// handleUpdate handles update events for canary deployments.
func (c *Controller) handleUpdate(oldObj, newObj interface{}) {
	oldCanary, ok := oldObj.(*deploymentv1alpha1.CanaryDeployment)
	if !ok {
		runtime.HandleError(fmt.Errorf("expected CanaryDeployment, got %T", oldObj))
		return
	}

	newCanary, ok := newObj.(*deploymentv1alpha1.CanaryDeployment)
	if !ok {
		runtime.HandleError(fmt.Errorf("expected CanaryDeployment, got %T", newObj))
		return
	}

	// Only process if generation changed (spec update)
	if oldCanary.Generation != newCanary.Generation {
		klog.V(3).Infof("Updated canary deployment %s/%s (generation: %d -> %d)", 
			newCanary.Namespace, newCanary.Name, oldCanary.Generation, newCanary.Generation)
		c.enqueueCanary(newCanary)
	}
}

// handleDelete handles delete events for canary deployments.
func (c *Controller) handleDelete(obj interface{}) {
	canary, ok := obj.(*deploymentv1alpha1.CanaryDeployment)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		canary, ok = tombstone.Obj.(*deploymentv1alpha1.CanaryDeployment)
		if !ok {
			runtime.HandleError(fmt.Errorf("tombstone contained object that is not a CanaryDeployment %#v", obj))
			return
		}
	}

	klog.V(3).Infof("Deleted canary deployment %s/%s", canary.Namespace, canary.Name)
	
	// Clean up any remaining resources for the deleted canary
	go func() {
		ctx := context.TODO()
		if err := c.promotionManager.RollbackCanary(ctx, canary); err != nil {
			klog.Errorf("Failed to cleanup resources for deleted canary %s/%s: %v", canary.Namespace, canary.Name, err)
		}
	}()
}

// enqueueCanary enqueues a canary deployment for processing.
func (c *Controller) enqueueCanary(canary *deploymentv1alpha1.CanaryDeployment) {
	key, err := cache.MetaNamespaceKeyFunc(canary)
	if err != nil {
		runtime.HandleError(fmt.Errorf("couldn't get key for canary %s/%s: %v", canary.Namespace, canary.Name, err))
		return
	}
	c.workqueue.Add(key)
}