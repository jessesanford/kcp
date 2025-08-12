/*
Copyright The KCP Authors.

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

package policies

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/scale"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/logging"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
)

// ScalingDecision represents a decision to scale a resource
type ScalingDecision struct {
	// Target resource information
	Target ScalingTarget `json:"target"`
	
	// Current replica count
	CurrentReplicas int32 `json:"currentReplicas"`
	
	// Desired replica count
	DesiredReplicas int32 `json:"desiredReplicas"`
	
	// Scaling action (up, down, none)
	Action ScalingAction `json:"action"`
	
	// Reason for the scaling decision
	Reason string `json:"reason"`
	
	// Triggered by policy
	TriggeredBy string `json:"triggeredBy"`
	
	// Timestamp of decision
	Timestamp metav1.Time `json:"timestamp"`
}

// ScalingAction represents the type of scaling action
type ScalingAction string

const (
	ScalingActionUp   ScalingAction = "up"
	ScalingActionDown ScalingAction = "down"
	ScalingActionNone ScalingAction = "none"
)

// MetricValue represents a metric measurement
type MetricValue struct {
	// Metric name
	Name string `json:"name"`
	
	// Current value
	Current string `json:"current"`
	
	// Timestamp of measurement
	Timestamp metav1.Time `json:"timestamp"`
	
	// Additional metadata
	Labels map[string]string `json:"labels,omitempty"`
}

// EvaluationContext holds context for policy evaluation
type EvaluationContext struct {
	// Logical cluster
	Cluster logicalcluster.Name
	
	// Current metrics
	Metrics []MetricValue
	
	// Current resource state
	CurrentState *ResourceState
	
	// Evaluation timestamp
	EvaluationTime metav1.Time
}

// ResourceState represents the current state of a scalable resource
type ResourceState struct {
	// Current replica count
	Replicas int32 `json:"replicas"`
	
	// Ready replica count
	ReadyReplicas int32 `json:"readyReplicas"`
	
	// Resource utilization
	Utilization map[string]string `json:"utilization,omitempty"`
}

// Evaluator evaluates scaling policies and makes scaling decisions
type Evaluator struct {
	kcpClusterClient kcpclientset.ClusterInterface
	scaleClient      scale.ScalesGetter
	
	// Policy manager reference
	policyManager *Manager
	
	// Evaluation history for decision tracking
	evaluationHistory map[string][]*ScalingDecision
	historyMutex      sync.RWMutex
	
	// Decision cooldown tracking
	cooldownTracker map[string]time.Time
	cooldownMutex   sync.RWMutex
	
	// Configuration
	config *EvaluatorConfig
}

// EvaluatorConfig holds evaluator configuration
type EvaluatorConfig struct {
	// Default evaluation interval
	EvaluationInterval time.Duration
	
	// Decision history retention
	HistoryRetention time.Duration
	
	// Default cooldown periods
	DefaultCooldownUp   time.Duration
	DefaultCooldownDown time.Duration
	
	// Maximum concurrent evaluations
	MaxConcurrentEvaluations int
}

// NewEvaluator creates a new policy evaluator
func NewEvaluator(
	kcpClusterClient kcpclientset.ClusterInterface,
	scaleClient scale.ScalesGetter,
	policyManager *Manager,
	config *EvaluatorConfig,
) *Evaluator {
	if config == nil {
		config = &EvaluatorConfig{
			EvaluationInterval:       30 * time.Second,
			HistoryRetention:         1 * time.Hour,
			DefaultCooldownUp:        3 * time.Minute,
			DefaultCooldownDown:      5 * time.Minute,
			MaxConcurrentEvaluations: 10,
		}
	}
	
	return &Evaluator{
		kcpClusterClient:  kcpClusterClient,
		scaleClient:       scaleClient,
		policyManager:     policyManager,
		evaluationHistory: make(map[string][]*ScalingDecision),
		cooldownTracker:   make(map[string]time.Time),
		config:            config,
	}
}

// EvaluatePolicy evaluates a single scaling policy against current conditions
func (e *Evaluator) EvaluatePolicy(ctx context.Context, cluster logicalcluster.Name, policy *ScalingPolicySpec, metrics []MetricValue) (*ScalingDecision, error) {
	logger := logging.WithObject(klog.FromContext(ctx), nil).WithValues(
		"evaluator", "scaling-policy",
		"cluster", cluster,
		"target", fmt.Sprintf("%s/%s", policy.Target.Kind, policy.Target.Name),
	)
	
	// Get current resource state
	resourceState, err := e.getCurrentResourceState(ctx, cluster, &policy.Target)
	if err != nil {
		return nil, fmt.Errorf("failed to get current resource state: %w", err)
	}
	
	// Create evaluation context
	evalCtx := &EvaluationContext{
		Cluster:        cluster,
		Metrics:        metrics,
		CurrentState:   resourceState,
		EvaluationTime: metav1.Now(),
	}
	
	// Check cooldown
	if e.isInCooldown(cluster, policy) {
		logger.V(4).Info("Policy evaluation skipped due to cooldown")
		return &ScalingDecision{
			Target:          policy.Target,
			CurrentReplicas: resourceState.Replicas,
			DesiredReplicas: resourceState.Replicas,
			Action:          ScalingActionNone,
			Reason:          "scaling action in cooldown period",
			Timestamp:       evalCtx.EvaluationTime,
		}, nil
	}
	
	// Evaluate triggers
	triggeredBy, targetReplicas, err := e.evaluateTriggers(ctx, policy, evalCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate triggers: %w", err)
	}
	
	if targetReplicas == resourceState.Replicas {
		logger.V(4).Info("No scaling needed", "currentReplicas", resourceState.Replicas)
		return &ScalingDecision{
			Target:          policy.Target,
			CurrentReplicas: resourceState.Replicas,
			DesiredReplicas: resourceState.Replicas,
			Action:          ScalingActionNone,
			Reason:          "no scaling triggers activated",
			Timestamp:       evalCtx.EvaluationTime,
		}, nil
	}
	
	// Apply constraints
	constrainedReplicas, reason := e.applyConstraints(targetReplicas, resourceState.Replicas, &policy.Constraints)
	
	// Apply behavior rules if specified
	if policy.Behavior != nil {
		constrainedReplicas, reason = e.applyBehaviorRules(constrainedReplicas, resourceState.Replicas, policy.Behavior, reason)
	}
	
	// Determine scaling action
	action := ScalingActionNone
	if constrainedReplicas > resourceState.Replicas {
		action = ScalingActionUp
	} else if constrainedReplicas < resourceState.Replicas {
		action = ScalingActionDown
	}
	
	decision := &ScalingDecision{
		Target:          policy.Target,
		CurrentReplicas: resourceState.Replicas,
		DesiredReplicas: constrainedReplicas,
		Action:          action,
		Reason:          reason,
		TriggeredBy:     triggeredBy,
		Timestamp:       evalCtx.EvaluationTime,
	}
	
	// Record decision in history
	e.recordDecision(cluster, policy, decision)
	
	logger.V(2).Info("Policy evaluation completed",
		"currentReplicas", resourceState.Replicas,
		"desiredReplicas", constrainedReplicas,
		"action", action,
		"reason", reason,
		"triggeredBy", triggeredBy,
	)
	
	return decision, nil
}

// evaluateTriggers evaluates all triggers in a policy and determines target replica count
func (e *Evaluator) evaluateTriggers(ctx context.Context, policy *ScalingPolicySpec, evalCtx *EvaluationContext) (string, int32, error) {
	currentReplicas := evalCtx.CurrentState.Replicas
	
	for _, trigger := range policy.Triggers {
		triggered, targetReplicas, err := e.evaluateSingleTrigger(ctx, &trigger, evalCtx)
		if err != nil {
			return "", currentReplicas, fmt.Errorf("failed to evaluate trigger %s: %w", trigger.Type, err)
		}
		
		if triggered {
			return trigger.Type, targetReplicas, nil
		}
	}
	
	// No triggers activated
	return "", currentReplicas, nil
}

// evaluateSingleTrigger evaluates a single trigger
func (e *Evaluator) evaluateSingleTrigger(ctx context.Context, trigger *ScalingTrigger, evalCtx *EvaluationContext) (bool, int32, error) {
	switch trigger.Type {
	case "cpu", "memory", "custom":
		return e.evaluateMetricTrigger(ctx, trigger, evalCtx)
	case "schedule":
		return e.evaluateScheduleTrigger(ctx, trigger, evalCtx)
	default:
		return false, evalCtx.CurrentState.Replicas, fmt.Errorf("unsupported trigger type: %s", trigger.Type)
	}
}

// evaluateMetricTrigger evaluates metric-based triggers
func (e *Evaluator) evaluateMetricTrigger(ctx context.Context, trigger *ScalingTrigger, evalCtx *EvaluationContext) (bool, int32, error) {
	if trigger.Threshold == nil {
		return false, evalCtx.CurrentState.Replicas, fmt.Errorf("threshold is required for metric trigger")
	}
	
	// Find matching metric
	var currentMetric *MetricValue
	for i, metric := range evalCtx.Metrics {
		if metric.Name == trigger.Threshold.Metric {
			currentMetric = &evalCtx.Metrics[i]
			break
		}
	}
	
	if currentMetric == nil {
		// Metric not available, don't trigger
		return false, evalCtx.CurrentState.Replicas, nil
	}
	
	// Parse current and target values
	currentVal, err := e.parseMetricValue(currentMetric.Current)
	if err != nil {
		return false, evalCtx.CurrentState.Replicas, fmt.Errorf("failed to parse current metric value: %w", err)
	}
	
	targetVal, err := e.parseMetricValue(trigger.Threshold.TargetValue)
	if err != nil {
		return false, evalCtx.CurrentState.Replicas, fmt.Errorf("failed to parse target metric value: %w", err)
	}
	
	// Compare values based on operator
	triggered := e.compareMetricValues(currentVal, targetVal, trigger.Threshold.Operator)
	
	if !triggered {
		return false, evalCtx.CurrentState.Replicas, nil
	}
	
	// Calculate target replica count based on metric
	targetReplicas := e.calculateTargetReplicasFromMetric(evalCtx.CurrentState.Replicas, currentVal, targetVal, trigger.Threshold.Operator)
	
	return true, targetReplicas, nil
}

// evaluateScheduleTrigger evaluates schedule-based triggers
func (e *Evaluator) evaluateScheduleTrigger(ctx context.Context, trigger *ScalingTrigger, evalCtx *EvaluationContext) (bool, int32, error) {
	if trigger.Schedule == nil {
		return false, evalCtx.CurrentState.Replicas, fmt.Errorf("schedule is required for schedule trigger")
	}
	
	// TODO: Implement cron-based schedule evaluation
	// This would require a cron parser and scheduler
	
	// For now, return false (not triggered)
	return false, evalCtx.CurrentState.Replicas, nil
}

// parseMetricValue parses a metric value string to a float64
func (e *Evaluator) parseMetricValue(value string) (float64, error) {
	// Try parsing as resource.Quantity first (for memory, storage)
	if quantity, err := resource.ParseQuantity(value); err == nil {
		return quantity.AsApproximateFloat64(), nil
	}
	
	// Try parsing as plain number (for CPU, percentages)
	return strconv.ParseFloat(value, 64)
}

// compareMetricValues compares two metric values using the specified operator
func (e *Evaluator) compareMetricValues(current, target float64, operator string) bool {
	switch operator {
	case ">":
		return current > target
	case "<":
		return current < target
	case ">=":
		return current >= target
	case "<=":
		return current <= target
	case "==":
		return current == target
	default:
		return false
	}
}

// calculateTargetReplicasFromMetric calculates target replica count based on metric comparison
func (e *Evaluator) calculateTargetReplicasFromMetric(currentReplicas int32, currentVal, targetVal float64, operator string) int32 {
	// Simple scaling algorithm based on ratio
	ratio := currentVal / targetVal
	
	var targetReplicas int32
	switch operator {
	case ">", ">=":
		// Scale up if current > target
		if ratio > 1.0 {
			targetReplicas = int32(float64(currentReplicas) * ratio)
		} else {
			targetReplicas = currentReplicas
		}
	case "<", "<=":
		// Scale down if current < target
		if ratio < 1.0 {
			targetReplicas = int32(float64(currentReplicas) * ratio)
		} else {
			targetReplicas = currentReplicas
		}
	default:
		targetReplicas = currentReplicas
	}
	
	// Ensure minimum of 1 replica
	if targetReplicas < 1 {
		targetReplicas = 1
	}
	
	return targetReplicas
}

// applyConstraints applies scaling constraints to the target replica count
func (e *Evaluator) applyConstraints(targetReplicas, currentReplicas int32, constraints *ScalingConstraints) (int32, string) {
	constrainedReplicas := targetReplicas
	reasons := []string{}
	
	// Apply min replicas constraint
	if constraints.MinReplicas != nil && constrainedReplicas < *constraints.MinReplicas {
		constrainedReplicas = *constraints.MinReplicas
		reasons = append(reasons, fmt.Sprintf("limited by minReplicas=%d", *constraints.MinReplicas))
	}
	
	// Apply max replicas constraint
	if constraints.MaxReplicas != nil && constrainedReplicas > *constraints.MaxReplicas {
		constrainedReplicas = *constraints.MaxReplicas
		reasons = append(reasons, fmt.Sprintf("limited by maxReplicas=%d", *constraints.MaxReplicas))
	}
	
	// Apply scale up rate constraint
	if constrainedReplicas > currentReplicas && constraints.MaxScaleUp != nil {
		maxIncrease := currentReplicas + *constraints.MaxScaleUp
		if constrainedReplicas > maxIncrease {
			constrainedReplicas = maxIncrease
			reasons = append(reasons, fmt.Sprintf("limited by maxScaleUp=%d", *constraints.MaxScaleUp))
		}
	}
	
	// Apply scale down rate constraint
	if constrainedReplicas < currentReplicas && constraints.MaxScaleDown != nil {
		maxDecrease := currentReplicas - *constraints.MaxScaleDown
		if constrainedReplicas < maxDecrease {
			constrainedReplicas = maxDecrease
			reasons = append(reasons, fmt.Sprintf("limited by maxScaleDown=%d", *constraints.MaxScaleDown))
		}
	}
	
	reason := "triggered by policy"
	if len(reasons) > 0 {
		reason = "triggered by policy, " + fmt.Sprintf("constraints applied: %v", reasons)
	}
	
	return constrainedReplicas, reason
}

// applyBehaviorRules applies scaling behavior rules
func (e *Evaluator) applyBehaviorRules(targetReplicas, currentReplicas int32, behavior *ScalingBehavior, currentReason string) (int32, string) {
	// TODO: Implement sophisticated behavior rules
	// This would include:
	// - Stabilization windows
	// - Policy selection (Min, Max, Disabled)
	// - Custom scaling policies with periods
	
	// For now, return input values unchanged
	return targetReplicas, currentReason
}

// getCurrentResourceState gets the current state of a scalable resource
func (e *Evaluator) getCurrentResourceState(ctx context.Context, cluster logicalcluster.Name, target *ScalingTarget) (*ResourceState, error) {
	// Create GroupResource from target
	gv, err := schema.ParseGroupVersion(target.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid apiVersion: %w", err)
	}
	
	gr := schema.GroupResource{
		Group:    gv.Group,
		Resource: target.Kind, // Note: This should be the resource name, not kind
	}
	
	// Get scale subresource
	scale, err := e.scaleClient.Scales(target.Namespace).Get(ctx, gr, target.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get scale: %w", err)
	}
	
	return &ResourceState{
		Replicas:      scale.Spec.Replicas,
		ReadyReplicas: scale.Status.Replicas,
		Utilization:   make(map[string]string), // TODO: Populate from metrics
	}, nil
}

// isInCooldown checks if a policy is in cooldown period
func (e *Evaluator) isInCooldown(cluster logicalcluster.Name, policy *ScalingPolicySpec) bool {
	key := fmt.Sprintf("%s/%s/%s", cluster, policy.Target.Kind, policy.Target.Name)
	
	e.cooldownMutex.RLock()
	defer e.cooldownMutex.RUnlock()
	
	lastAction, exists := e.cooldownTracker[key]
	if !exists {
		return false
	}
	
	// Use default cooldown periods (could be made configurable per policy)
	cooldownPeriod := e.config.DefaultCooldownUp // TODO: Determine based on last action type
	
	return time.Since(lastAction) < cooldownPeriod
}

// recordDecision records a scaling decision in history and updates cooldown
func (e *Evaluator) recordDecision(cluster logicalcluster.Name, policy *ScalingPolicySpec, decision *ScalingDecision) {
	key := fmt.Sprintf("%s/%s/%s", cluster, policy.Target.Kind, policy.Target.Name)
	
	// Record in history
	e.historyMutex.Lock()
	if e.evaluationHistory[key] == nil {
		e.evaluationHistory[key] = make([]*ScalingDecision, 0)
	}
	e.evaluationHistory[key] = append(e.evaluationHistory[key], decision)
	
	// Clean old history
	cutoff := time.Now().Add(-e.config.HistoryRetention)
	filtered := make([]*ScalingDecision, 0)
	for _, d := range e.evaluationHistory[key] {
		if d.Timestamp.Time.After(cutoff) {
			filtered = append(filtered, d)
		}
	}
	e.evaluationHistory[key] = filtered
	e.historyMutex.Unlock()
	
	// Update cooldown tracker if action was taken
	if decision.Action != ScalingActionNone {
		e.cooldownMutex.Lock()
		e.cooldownTracker[key] = time.Now()
		e.cooldownMutex.Unlock()
	}
}

// GetEvaluationHistory returns the evaluation history for a resource
func (e *Evaluator) GetEvaluationHistory(cluster logicalcluster.Name, target *ScalingTarget) []*ScalingDecision {
	key := fmt.Sprintf("%s/%s/%s", cluster, target.Kind, target.Name)
	
	e.historyMutex.RLock()
	defer e.historyMutex.RUnlock()
	
	history := e.evaluationHistory[key]
	if history == nil {
		return []*ScalingDecision{}
	}
	
	// Return a copy
	result := make([]*ScalingDecision, len(history))
	copy(result, history)
	return result
}