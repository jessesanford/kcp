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

package canary

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kcp-dev/logicalcluster/v3"
)

// CanaryDeployment represents a canary deployment resource.
// This would typically be defined in the API types but is included here for the strategy.
type CanaryDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CanaryDeploymentSpec   `json:"spec,omitempty"`
	Status CanaryDeploymentStatus `json:"status,omitempty"`
}

// CanaryDeploymentSpec defines the desired state of CanaryDeployment.
type CanaryDeploymentSpec struct {
	// TrafficPercentages defines the progressive traffic split percentages.
	TrafficPercentages []int32 `json:"trafficPercentages"`
	// AnalysisInterval defines how long to wait before analyzing metrics.
	AnalysisInterval metav1.Duration `json:"analysisInterval"`
	// SuccessThreshold defines the success rate threshold for promotion.
	SuccessThreshold float64 `json:"successThreshold"`
	// RollbackThreshold defines the failure rate threshold for rollback.
	RollbackThreshold float64 `json:"rollbackThreshold"`
}

// CanaryDeploymentStatus defines the observed state of CanaryDeployment.
type CanaryDeploymentStatus struct {
	// State represents the current canary state.
	State CanaryState `json:"state"`
	// CurrentTrafficPercentage shows the current traffic split.
	CurrentTrafficPercentage int32 `json:"currentTrafficPercentage"`
	// Conditions represent the latest available observations.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// LastAnalysisTime records when metrics were last analyzed.
	LastAnalysisTime *metav1.Time `json:"lastAnalysisTime,omitempty"`
}

// Controller manages canary deployments using the state machine.
type Controller struct {
	client         client.Client
	scheme         *runtime.Scheme
	analysisEngine AnalysisEngine
	trafficManager TrafficManager
	metricsCollector MetricsCollector
}

// NewController creates a new canary deployment controller.
func NewController(
	client client.Client,
	scheme *runtime.Scheme,
	analysisEngine AnalysisEngine,
	trafficManager TrafficManager,
	metricsCollector MetricsCollector,
) *Controller {
	return &Controller{
		client:           client,
		scheme:           scheme,
		analysisEngine:   analysisEngine,
		trafficManager:   trafficManager,
		metricsCollector: metricsCollector,
	}
}

// Reconcile handles canary deployment reconciliation.
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := klog.FromContext(ctx).WithValues("canary", req.NamespacedName)
	ctx = klog.NewContext(ctx, logger)

	logger.V(2).Info("Starting canary reconciliation")

	// Fetch the CanaryDeployment instance
	canary := &CanaryDeployment{}
	if err := c.client.Get(ctx, req.NamespacedName, canary); err != nil {
		if errors.IsNotFound(err) {
			logger.V(2).Info("CanaryDeployment resource not found, ignoring")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get CanaryDeployment: %w", err)
	}

	// Create state machine from current status
	stateMachine := NewStateMachine()
	if canary.Status.State != "" {
		stateMachine.currentState = canary.Status.State
	}

	// Handle reconciliation based on current state
	result, err := c.reconcileCanaryState(ctx, canary, stateMachine)
	if err != nil {
		logger.Error(err, "Failed to reconcile canary state")
		c.updateStatusCondition(canary, "ReconcileFailed", metav1.ConditionTrue, "ReconciliationError", err.Error())
	}

	// Update status if changed
	if err := c.updateStatus(ctx, canary); err != nil {
		logger.Error(err, "Failed to update canary status")
		return reconcile.Result{}, err
	}

	return result, err
}

// reconcileCanaryState handles state-specific reconciliation logic.
func (c *Controller) reconcileCanaryState(ctx context.Context, canary *CanaryDeployment, sm *StateMachine) (reconcile.Result, error) {
	logger := klog.FromContext(ctx)
	
	switch sm.GetCurrentState() {
	case StateInitializing:
		return c.reconcileInitializing(ctx, canary, sm)
	case StateProgressing:
		return c.reconcileProgressing(ctx, canary, sm)
	case StateAnalyzing:
		return c.reconcileAnalyzing(ctx, canary, sm)
	case StatePromoting:
		return c.reconcilePromoting(ctx, canary, sm)
	case StateRollingBack:
		return c.reconcileRollingBack(ctx, canary, sm)
	case StateCompleted, StateFailed:
		logger.V(2).Info("Canary deployment in terminal state", "state", sm.GetCurrentState())
		return reconcile.Result{}, nil
	default:
		return reconcile.Result{}, fmt.Errorf("unknown canary state: %s", sm.GetCurrentState())
	}
}

// reconcileInitializing handles the initialization phase.
func (c *Controller) reconcileInitializing(ctx context.Context, canary *CanaryDeployment, sm *StateMachine) (reconcile.Result, error) {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Initializing canary deployment")

	// Initialize traffic to 0% for canary
	if err := c.trafficManager.SetTrafficWeight(ctx, canary, 0); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to initialize traffic: %w", err)
	}

	// Transition to progressing state
	if err := sm.TransitionTo(ctx, StateProgressing, "Initialization complete"); err != nil {
		return reconcile.Result{}, err
	}

	canary.Status.State = StateProgressing
	canary.Status.CurrentTrafficPercentage = 0
	c.updateStatusCondition(canary, "Initialized", metav1.ConditionTrue, "InitializationComplete", "Canary deployment initialized successfully")

	return reconcile.Result{RequeueAfter: time.Second * 30}, nil
}

// reconcileProgressing handles the traffic progression phase.
func (c *Controller) reconcileProgressing(ctx context.Context, canary *CanaryDeployment, sm *StateMachine) (reconcile.Result, error) {
	logger := klog.FromContext(ctx)
	
	currentIndex := c.getCurrentTrafficIndex(canary.Status.CurrentTrafficPercentage, canary.Spec.TrafficPercentages)
	if currentIndex >= len(canary.Spec.TrafficPercentages) {
		// All traffic percentages reached, move to promotion
		if err := sm.TransitionTo(ctx, StatePromoting, "All traffic percentages completed"); err != nil {
			return reconcile.Result{}, err
		}
		canary.Status.State = StatePromoting
		return reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}

	nextPercentage := canary.Spec.TrafficPercentages[currentIndex]
	logger.V(2).Info("Progressing canary traffic", "targetPercentage", nextPercentage)

	if err := c.trafficManager.SetTrafficWeight(ctx, canary, nextPercentage); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to set traffic weight: %w", err)
	}

	canary.Status.CurrentTrafficPercentage = nextPercentage
	
	// Transition to analysis phase
	if err := sm.TransitionTo(ctx, StateAnalyzing, fmt.Sprintf("Traffic set to %d%%", nextPercentage)); err != nil {
		return reconcile.Result{}, err
	}

	canary.Status.State = StateAnalyzing
	c.updateStatusCondition(canary, "TrafficProgressed", metav1.ConditionTrue, "TrafficUpdated", 
		fmt.Sprintf("Traffic weight updated to %d%%", nextPercentage))

	return reconcile.Result{RequeueAfter: canary.Spec.AnalysisInterval.Duration}, nil
}

// reconcileAnalyzing handles the metrics analysis phase.
func (c *Controller) reconcileAnalyzing(ctx context.Context, canary *CanaryDeployment, sm *StateMachine) (reconcile.Result, error) {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Analyzing canary metrics")

	// Collect and analyze metrics
	metrics, err := c.metricsCollector.CollectMetrics(ctx, canary)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to collect metrics: %w", err)
	}

	analysis, err := c.analysisEngine.AnalyzeMetrics(ctx, metrics, canary)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to analyze metrics: %w", err)
	}

	now := metav1.Now()
	canary.Status.LastAnalysisTime = &now

	// Make decision based on analysis
	switch analysis.Decision {
	case AnalysisDecisionContinue:
		// Continue to next traffic percentage
		if err := sm.TransitionTo(ctx, StateProgressing, "Analysis passed, continuing progression"); err != nil {
			return reconcile.Result{}, err
		}
		canary.Status.State = StateProgressing
		c.updateStatusCondition(canary, "AnalysisPassed", metav1.ConditionTrue, "MetricsHealthy", "Analysis passed, continuing progression")
		
	case AnalysisDecisionPromote:
		// Promote to production
		if err := sm.TransitionTo(ctx, StatePromoting, "Analysis excellent, promoting to production"); err != nil {
			return reconcile.Result{}, err
		}
		canary.Status.State = StatePromoting
		c.updateStatusCondition(canary, "PromotionTriggered", metav1.ConditionTrue, "MetricsExcellent", "Metrics excellent, promoting to production")
		
	case AnalysisDecisionRollback:
		// Rollback due to poor metrics
		if err := sm.TransitionTo(ctx, StateRollingBack, "Analysis failed, rolling back"); err != nil {
			return reconcile.Result{}, err
		}
		canary.Status.State = StateRollingBack
		c.updateStatusCondition(canary, "RollbackTriggered", metav1.ConditionTrue, "MetricsFailed", "Metrics failed threshold, rolling back")
	}

	return reconcile.Result{RequeueAfter: time.Second * 10}, nil
}

// reconcilePromoting handles the promotion phase.
func (c *Controller) reconcilePromoting(ctx context.Context, canary *CanaryDeployment, sm *StateMachine) (reconcile.Result, error) {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Promoting canary to production")

	// Set traffic to 100% for canary version
	if err := c.trafficManager.SetTrafficWeight(ctx, canary, 100); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to promote canary: %w", err)
	}

	// Transition to completed
	if err := sm.TransitionTo(ctx, StateCompleted, "Canary promotion completed"); err != nil {
		return reconcile.Result{}, err
	}

	canary.Status.State = StateCompleted
	canary.Status.CurrentTrafficPercentage = 100
	c.updateStatusCondition(canary, "PromotionCompleted", metav1.ConditionTrue, "DeploymentSuccessful", "Canary deployment promoted successfully")

	return reconcile.Result{}, nil
}

// reconcileRollingBack handles the rollback phase.
func (c *Controller) reconcileRollingBack(ctx context.Context, canary *CanaryDeployment, sm *StateMachine) (reconcile.Result, error) {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Rolling back canary deployment")

	// Set traffic back to 0% for canary (100% to stable)
	if err := c.trafficManager.SetTrafficWeight(ctx, canary, 0); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to rollback canary: %w", err)
	}

	// Transition to failed
	if err := sm.TransitionTo(ctx, StateFailed, "Canary rollback completed"); err != nil {
		return reconcile.Result{}, err
	}

	canary.Status.State = StateFailed
	canary.Status.CurrentTrafficPercentage = 0
	c.updateStatusCondition(canary, "RollbackCompleted", metav1.ConditionTrue, "DeploymentRolledBack", "Canary deployment rolled back due to failures")

	return reconcile.Result{}, nil
}

// getCurrentTrafficIndex finds the current index in the traffic percentages array.
func (c *Controller) getCurrentTrafficIndex(currentPercentage int32, percentages []int32) int {
	for i, percentage := range percentages {
		if currentPercentage < percentage {
			return i
		}
	}
	return len(percentages)
}

// updateStatusCondition updates a condition in the canary status.
func (c *Controller) updateStatusCondition(canary *CanaryDeployment, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	// Update or append condition
	for i, existing := range canary.Status.Conditions {
		if existing.Type == conditionType {
			canary.Status.Conditions[i] = condition
			return
		}
	}
	canary.Status.Conditions = append(canary.Status.Conditions, condition)
}

// updateStatus updates the canary deployment status.
func (c *Controller) updateStatus(ctx context.Context, canary *CanaryDeployment) error {
	return c.client.Status().Update(ctx, canary)
}

// SetupWithManager sets up the controller with the Manager.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&CanaryDeployment{}).
		Complete(c)
}