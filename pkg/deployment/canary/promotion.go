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

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	deploymentv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/deployment/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// PromotionManager handles canary promotion and rollback operations.
type PromotionManager struct {
	client         kubernetes.Interface
	trafficManager TrafficManager
	analyzer       MetricsAnalyzer
}

// NewPromotionManager creates a new promotion manager.
func NewPromotionManager(client kubernetes.Interface, trafficManager TrafficManager, analyzer MetricsAnalyzer) *PromotionManager {
	return &PromotionManager{
		client:         client,
		trafficManager: trafficManager,
		analyzer:       analyzer,
	}
}

// PromoteCanary promotes the canary to the next step or completes the rollout.
func (pm *PromotionManager) PromoteCanary(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) error {
	klog.V(2).Infof("Promoting canary %s/%s", canary.Namespace, canary.Name)

	// Validate current state allows promotion
	if err := pm.validatePromotionAllowed(canary); err != nil {
		return fmt.Errorf("promotion not allowed: %w", err)
	}

	// Check if this is the final promotion
	if canary.Status.CurrentStep >= len(canary.Spec.Strategy.Steps)-1 {
		return pm.completeFinalPromotion(ctx, canary)
	}

	// Promote to next step
	return pm.promoteToNextStep(ctx, canary)
}

// RollbackCanary performs rollback to the stable version.
func (pm *PromotionManager) RollbackCanary(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) error {
	klog.V(2).Infof("Rolling back canary %s/%s", canary.Namespace, canary.Name)

	// Set traffic back to 0% canary (100% stable)
	if err := pm.trafficManager.SetTrafficWeight(ctx, canary, 0); err != nil {
		return fmt.Errorf("failed to set traffic to stable: %w", err)
	}

	// Clean up canary resources
	if err := pm.cleanupCanaryResources(ctx, canary); err != nil {
		klog.Errorf("Failed to cleanup canary resources for %s/%s: %v", canary.Namespace, canary.Name, err)
		// Don't fail the rollback operation for cleanup errors
	}

	// Update canary status
	pm.updateRollbackStatus(canary)

	klog.V(2).Infof("Successfully rolled back canary %s/%s", canary.Namespace, canary.Name)
	return nil
}

// ShouldPromote determines if the canary should be promoted based on analysis.
func (pm *PromotionManager) ShouldPromote(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) (bool, error) {
	// Check if auto-promotion is enabled
	if canary.Spec.Strategy.AutoPromotion == nil || !*canary.Spec.Strategy.AutoPromotion {
		klog.V(3).Infof("Auto-promotion disabled for canary %s/%s", canary.Namespace, canary.Name)
		return false, nil
	}

	// Check if we've waited long enough at the current step
	stepDuration := 5 * time.Minute // Default step duration
	if canary.Spec.Strategy.StepDuration != nil {
		stepDuration = canary.Spec.Strategy.StepDuration.Duration
	}

	if canary.Status.StepStartTime == nil {
		klog.V(3).Infof("Step start time not set for canary %s/%s", canary.Namespace, canary.Name)
		return false, nil
	}

	if time.Since(canary.Status.StepStartTime.Time) < stepDuration {
		klog.V(4).Infof("Not enough time elapsed for canary %s/%s (elapsed: %v, required: %v)", 
			canary.Namespace, canary.Name, time.Since(canary.Status.StepStartTime.Time), stepDuration)
		return false, nil
	}

	// Perform metrics analysis
	analysisResults, err := pm.analyzer.AnalyzeMetrics(ctx, canary)
	if err != nil {
		return false, fmt.Errorf("failed to analyze metrics: %w", err)
	}

	// Calculate success rate
	success := pm.calculateAnalysisSuccess(analysisResults, canary)
	
	klog.V(3).Infof("Analysis success for canary %s/%s: %t", canary.Namespace, canary.Name, success)
	return success, nil
}

// ShouldRollback determines if the canary should be rolled back due to failures.
func (pm *PromotionManager) ShouldRollback(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) (bool, string, error) {
	// Check if the canary has exceeded the progress deadline
	if pm.hasExceededProgressDeadline(canary) {
		return true, "exceeded progress deadline", nil
	}

	// Check if analysis consistently fails
	if pm.hasConsistentAnalysisFailures(canary) {
		return true, "consistent analysis failures", nil
	}

	// Check if deployment is failing
	if failing, reason := pm.isDeploymentFailing(ctx, canary); failing {
		return true, reason, nil
	}

	return false, "", nil
}

// Helper methods

// validatePromotionAllowed checks if promotion is allowed in the current state.
func (pm *PromotionManager) validatePromotionAllowed(canary *deploymentv1alpha1.CanaryDeployment) error {
	switch canary.Status.Phase {
	case deploymentv1alpha1.CanaryPhasePromoting:
		return nil // Promotion is allowed
	case deploymentv1alpha1.CanaryPhaseSucceeded:
		return fmt.Errorf("canary already succeeded")
	case deploymentv1alpha1.CanaryPhaseFailed:
		return fmt.Errorf("canary has failed")
	case deploymentv1alpha1.CanaryPhaseRollingBack:
		return fmt.Errorf("canary is rolling back")
	default:
		return fmt.Errorf("promotion not allowed in phase %s", canary.Status.Phase)
	}
}

// completeFinalPromotion completes the final promotion to make canary the new stable version.
func (pm *PromotionManager) completeFinalPromotion(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) error {
	klog.V(2).Infof("Completing final promotion for canary %s/%s", canary.Namespace, canary.Name)

	// Set traffic to 100% canary
	if err := pm.trafficManager.SetTrafficWeight(ctx, canary, 100); err != nil {
		return fmt.Errorf("failed to set traffic to 100%% canary: %w", err)
	}

	// Update the stable deployment to use the canary version
	if err := pm.promoteCanaryToStable(ctx, canary); err != nil {
		return fmt.Errorf("failed to promote canary to stable: %w", err)
	}

	// Clean up temporary canary resources
	if err := pm.cleanupCanaryResources(ctx, canary); err != nil {
		klog.Errorf("Failed to cleanup canary resources after promotion: %v", err)
		// Don't fail the promotion for cleanup errors
	}

	// Update status to succeeded
	pm.updateSuccessStatus(canary)

	klog.V(2).Infof("Successfully completed final promotion for canary %s/%s", canary.Namespace, canary.Name)
	return nil
}

// promoteToNextStep promotes the canary to the next step in the rollout.
func (pm *PromotionManager) promoteToNextStep(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) error {
	nextStep := canary.Status.CurrentStep + 1
	if nextStep >= len(canary.Spec.Strategy.Steps) {
		return fmt.Errorf("no more steps to promote to")
	}

	nextTrafficPercentage := canary.Spec.Strategy.Steps[nextStep]

	klog.V(3).Infof("Promoting canary %s/%s to step %d (%d%% traffic)", 
		canary.Namespace, canary.Name, nextStep, nextTrafficPercentage)

	// Update traffic distribution
	if err := pm.trafficManager.SetTrafficWeight(ctx, canary, nextTrafficPercentage); err != nil {
		return fmt.Errorf("failed to update traffic to %d%%: %w", nextTrafficPercentage, err)
	}

	// Update canary status
	canary.Status.CurrentStep = nextStep
	canary.Status.Phase = deploymentv1alpha1.CanaryPhaseProgressing
	now := metav1.Now()
	canary.Status.StepStartTime = &now
	canary.Spec.TrafficPercentage = nextTrafficPercentage
	canary.Status.Message = fmt.Sprintf("Promoted to step %d/%d (%d%% traffic)", 
		nextStep+1, len(canary.Spec.Strategy.Steps), nextTrafficPercentage)

	// Update conditions
	conditionsv1alpha1.Set(&canary.Status.Conditions, conditionsv1alpha1.Condition{
		Type:               "Progressing",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "PromotedToNextStep",
		Message:            canary.Status.Message,
	})

	return nil
}

// promoteCanaryToStable updates the stable deployment to use the canary version.
func (pm *PromotionManager) promoteCanaryToStable(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) error {
	// Get the target deployment
	namespace := canary.Spec.TargetRef.Namespace
	if namespace == "" {
		namespace = canary.Namespace
	}

	deployment, err := pm.client.AppsV1().Deployments(namespace).Get(ctx, canary.Spec.TargetRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get target deployment: %w", err)
	}

	// Get the canary deployment to copy its configuration
	canaryDeployment, err := pm.client.AppsV1().Deployments(namespace).Get(ctx, fmt.Sprintf("%s-canary", deployment.Name), metav1.GetOptions{})
	if err != nil {
		klog.Warningf("Canary deployment not found, using current configuration: %v", err)
	} else {
		// Copy canary pod template to stable deployment
		deployment.Spec.Template = canaryDeployment.Spec.Template
	}

	// Update version labels
	if deployment.Spec.Template.Labels == nil {
		deployment.Spec.Template.Labels = make(map[string]string)
	}
	deployment.Spec.Template.Labels["canary.kcp.io/version"] = canary.Spec.CanaryVersion
	deployment.Spec.Template.Labels["canary.kcp.io/type"] = "stable"

	// Update selector
	if deployment.Spec.Selector.MatchLabels == nil {
		deployment.Spec.Selector.MatchLabels = make(map[string]string)
	}
	deployment.Spec.Selector.MatchLabels["canary.kcp.io/version"] = canary.Spec.CanaryVersion
	deployment.Spec.Selector.MatchLabels["canary.kcp.io/type"] = "stable"

	// Update the deployment
	_, err = pm.client.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update stable deployment: %w", err)
	}

	klog.V(2).Infof("Updated stable deployment %s/%s to canary version %s", namespace, deployment.Name, canary.Spec.CanaryVersion)
	return nil
}

// cleanupCanaryResources removes temporary canary resources.
func (pm *PromotionManager) cleanupCanaryResources(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) error {
	namespace := canary.Spec.TargetRef.Namespace
	if namespace == "" {
		namespace = canary.Namespace
	}

	deploymentName := canary.Spec.TargetRef.Name
	canaryDeploymentName := fmt.Sprintf("%s-canary", deploymentName)
	canaryServiceName := fmt.Sprintf("%s-canary", deploymentName)

	// Delete canary deployment
	err := pm.client.AppsV1().Deployments(namespace).Delete(ctx, canaryDeploymentName, metav1.DeleteOptions{})
	if err != nil {
		klog.Warningf("Failed to delete canary deployment %s/%s: %v", namespace, canaryDeploymentName, err)
	} else {
		klog.V(3).Infof("Deleted canary deployment %s/%s", namespace, canaryDeploymentName)
	}

	// Delete canary service if it exists
	err = pm.client.CoreV1().Services(namespace).Delete(ctx, canaryServiceName, metav1.DeleteOptions{})
	if err != nil {
		klog.V(4).Infof("Canary service %s/%s not found or failed to delete: %v", namespace, canaryServiceName, err)
	} else {
		klog.V(3).Infof("Deleted canary service %s/%s", namespace, canaryServiceName)
	}

	return nil
}

// calculateAnalysisSuccess calculates if the analysis results indicate success.
func (pm *PromotionManager) calculateAnalysisSuccess(results []deploymentv1alpha1.AnalysisResult, canary *deploymentv1alpha1.CanaryDeployment) bool {
	if len(results) == 0 {
		return false
	}

	totalWeight := 0
	passedWeight := 0

	for _, result := range results {
		totalWeight += result.Weight
		if result.Passed {
			passedWeight += result.Weight
		}
	}

	if totalWeight == 0 {
		return false
	}

	successPercentage := (passedWeight * 100) / totalWeight
	threshold := 95 // Default threshold

	if canary.Spec.Analysis.Threshold != nil {
		threshold = *canary.Spec.Analysis.Threshold
	}

	return successPercentage >= threshold
}

// hasExceededProgressDeadline checks if the canary has exceeded its progress deadline.
func (pm *PromotionManager) hasExceededProgressDeadline(canary *deploymentv1alpha1.CanaryDeployment) bool {
	deadlineSeconds := int32(1800) // Default 30 minutes
	if canary.Spec.ProgressDeadlineSeconds != nil {
		deadlineSeconds = *canary.Spec.ProgressDeadlineSeconds
	}

	deadline := time.Duration(deadlineSeconds) * time.Second
	return time.Since(canary.CreationTimestamp.Time) > deadline
}

// hasConsistentAnalysisFailures checks if the canary has consistent analysis failures.
func (pm *PromotionManager) hasConsistentAnalysisFailures(canary *deploymentv1alpha1.CanaryDeployment) bool {
	// This is a simplified check - in practice, you might track failure history
	if len(canary.Status.AnalysisResults) == 0 {
		return false
	}

	failureCount := 0
	for _, result := range canary.Status.AnalysisResults {
		if !result.Passed {
			failureCount++
		}
	}

	// Consider it a consistent failure if more than 75% of recent analyses failed
	failureRate := float64(failureCount) / float64(len(canary.Status.AnalysisResults))
	return failureRate > 0.75
}

// isDeploymentFailing checks if the deployment is in a failing state.
func (pm *PromotionManager) isDeploymentFailing(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) (bool, string) {
	namespace := canary.Spec.TargetRef.Namespace
	if namespace == "" {
		namespace = canary.Namespace
	}

	// Check canary deployment health
	canaryDeploymentName := fmt.Sprintf("%s-canary", canary.Spec.TargetRef.Name)
	canaryDep, err := pm.client.AppsV1().Deployments(namespace).Get(ctx, canaryDeploymentName, metav1.GetOptions{})
	if err == nil {
		if pm.isDeploymentUnhealthy(canaryDep) {
			return true, "canary deployment is unhealthy"
		}
	}

	return false, ""
}

// isDeploymentUnhealthy checks if a deployment is in an unhealthy state.
func (pm *PromotionManager) isDeploymentUnhealthy(deployment *appsv1.Deployment) bool {
	// Check if deployment has been stuck for too long
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == metav1.ConditionFalse {
			if time.Since(condition.LastUpdateTime.Time) > 10*time.Minute {
				return true
			}
		}
	}

	// Check replica readiness
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0 {
		if deployment.Status.ReadyReplicas == 0 {
			return true
		}
		
		readinessRatio := float64(deployment.Status.ReadyReplicas) / float64(*deployment.Spec.Replicas)
		if readinessRatio < 0.5 { // Less than 50% ready
			return true
		}
	}

	return false
}

// updateSuccessStatus updates the canary status for successful completion.
func (pm *PromotionManager) updateSuccessStatus(canary *deploymentv1alpha1.CanaryDeployment) {
	canary.Status.Phase = deploymentv1alpha1.CanaryPhaseSucceeded
	canary.Status.Message = "Canary rollout completed successfully"
	canary.Spec.TrafficPercentage = 100

	now := metav1.Now()
	conditionsv1alpha1.Set(&canary.Status.Conditions, conditionsv1alpha1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             "CanarySucceeded",
		Message:            "Canary rollout completed successfully",
	})
}

// updateRollbackStatus updates the canary status for rollback completion.
func (pm *PromotionManager) updateRollbackStatus(canary *deploymentv1alpha1.CanaryDeployment) {
	canary.Status.Phase = deploymentv1alpha1.CanaryPhaseFailed
	canary.Status.Message = "Canary rollout rolled back due to failure"
	canary.Spec.TrafficPercentage = 0

	now := metav1.Now()
	conditionsv1alpha1.Set(&canary.Status.Conditions, conditionsv1alpha1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             "CanaryRolledBack",
		Message:            "Canary rollout was rolled back",
	})
}