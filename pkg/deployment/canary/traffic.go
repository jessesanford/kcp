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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	deploymentv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/deployment/v1alpha1"
)

// trafficManager implements TrafficManager for managing traffic distribution in canary deployments.
type trafficManager struct {
	client kubernetes.Interface
}

// NewTrafficManager creates a new traffic manager for canary deployments.
func NewTrafficManager(client kubernetes.Interface) TrafficManager {
	return &trafficManager{
		client: client,
	}
}

// SetTrafficWeight configures traffic distribution between canary and stable versions.
func (tm *trafficManager) SetTrafficWeight(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, canaryWeight int) error {
	klog.V(2).Infof("Setting traffic weight to %d%% canary for %s/%s", 
		canaryWeight, canary.Namespace, canary.Name)

	// Validate weight
	if canaryWeight < 0 || canaryWeight > 100 {
		return fmt.Errorf("invalid canary weight %d, must be between 0 and 100", canaryWeight)
	}

	// Get the target deployment
	deployment, err := tm.getTargetDeployment(ctx, canary)
	if err != nil {
		return fmt.Errorf("failed to get target deployment: %w", err)
	}

	// Calculate replica distribution
	totalReplicas := *deployment.Spec.Replicas
	canaryReplicas, stableReplicas := tm.calculateReplicaDistribution(totalReplicas, canaryWeight)

	klog.V(3).Infof("Replica distribution for %s/%s: canary=%d, stable=%d (total=%d)", 
		canary.Namespace, canary.Name, canaryReplicas, stableReplicas, totalReplicas)

	// Update deployment with traffic split
	if err := tm.updateDeploymentTraffic(ctx, canary, deployment, canaryReplicas, stableReplicas); err != nil {
		return fmt.Errorf("failed to update deployment traffic: %w", err)
	}

	// Update services if they exist
	if err := tm.updateServiceTraffic(ctx, canary, canaryWeight); err != nil {
		klog.Warningf("Failed to update service traffic for %s/%s: %v", canary.Namespace, canary.Name, err)
		// Don't fail the whole operation for service updates
	}

	klog.V(2).Infof("Successfully set traffic weight to %d%% canary for %s/%s", 
		canaryWeight, canary.Namespace, canary.Name)

	return nil
}

// GetCurrentTrafficWeights returns the current traffic distribution.
func (tm *trafficManager) GetCurrentTrafficWeights(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) (canaryWeight, stableWeight int, err error) {
	// Get the target deployment
	deployment, err := tm.getTargetDeployment(ctx, canary)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get target deployment: %w", err)
	}

	// Get current replica counts
	canaryReplicas, stableReplicas, err := tm.getCurrentReplicaCounts(ctx, canary, deployment)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get current replica counts: %w", err)
	}

	totalReplicas := canaryReplicas + stableReplicas
	if totalReplicas == 0 {
		return 0, 100, nil // Default to stable when no replicas
	}

	// Calculate percentage distribution
	canaryWeight = int((canaryReplicas * 100) / totalReplicas)
	stableWeight = 100 - canaryWeight

	klog.V(4).Infof("Current traffic weights for %s/%s: canary=%d%%, stable=%d%%", 
		canary.Namespace, canary.Name, canaryWeight, stableWeight)

	return canaryWeight, stableWeight, nil
}

// ValidateTrafficConfig validates the traffic configuration for the canary.
func (tm *trafficManager) ValidateTrafficConfig(canary *deploymentv1alpha1.CanaryDeployment) error {
	// Validate target reference
	if canary.Spec.TargetRef.Name == "" {
		return fmt.Errorf("target deployment name is required")
	}

	if canary.Spec.TargetRef.Kind != "" && canary.Spec.TargetRef.Kind != "Deployment" {
		return fmt.Errorf("only Deployment targets are supported, got %s", canary.Spec.TargetRef.Kind)
	}

	// Validate strategy steps
	for i, step := range canary.Spec.Strategy.Steps {
		if step < 0 || step > 100 {
			return fmt.Errorf("invalid traffic percentage at step %d: %d (must be 0-100)", i, step)
		}
	}

	// Validate current traffic percentage
	if canary.Spec.TrafficPercentage < 0 || canary.Spec.TrafficPercentage > 100 {
		return fmt.Errorf("invalid current traffic percentage: %d (must be 0-100)", canary.Spec.TrafficPercentage)
	}

	return nil
}

// Helper methods

// getTargetDeployment retrieves the target deployment for the canary.
func (tm *trafficManager) getTargetDeployment(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment) (*appsv1.Deployment, error) {
	namespace := canary.Spec.TargetRef.Namespace
	if namespace == "" {
		namespace = canary.Namespace
	}

	deployment, err := tm.client.AppsV1().Deployments(namespace).Get(ctx, canary.Spec.TargetRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, canary.Spec.TargetRef.Name, err)
	}

	return deployment, nil
}

// calculateReplicaDistribution calculates how to distribute replicas between canary and stable.
func (tm *trafficManager) calculateReplicaDistribution(totalReplicas int32, canaryWeight int) (canaryReplicas, stableReplicas int32) {
	if totalReplicas == 0 {
		return 0, 0
	}

	// Calculate canary replicas based on percentage
	canaryReplicas = (totalReplicas * int32(canaryWeight)) / 100
	stableReplicas = totalReplicas - canaryReplicas

	// Ensure at least one replica if weight > 0
	if canaryWeight > 0 && canaryReplicas == 0 {
		canaryReplicas = 1
		stableReplicas = totalReplicas - 1
	}

	// Ensure at least one stable replica if not 100% canary
	if canaryWeight < 100 && stableReplicas == 0 {
		stableReplicas = 1
		canaryReplicas = totalReplicas - 1
	}

	return canaryReplicas, stableReplicas
}

// updateDeploymentTraffic updates the deployment to split traffic between versions.
func (tm *trafficManager) updateDeploymentTraffic(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, deployment *appsv1.Deployment, canaryReplicas, stableReplicas int32) error {
	// Create canary deployment if needed
	if canaryReplicas > 0 {
		if err := tm.ensureCanaryDeployment(ctx, canary, deployment, canaryReplicas); err != nil {
			return fmt.Errorf("failed to ensure canary deployment: %w", err)
		}
	}

	// Update stable deployment replica count
	if stableReplicas != *deployment.Spec.Replicas {
		deployment.Spec.Replicas = &stableReplicas
		
		// Add labels to identify this as the stable version
		if deployment.Spec.Template.Labels == nil {
			deployment.Spec.Template.Labels = make(map[string]string)
		}
		deployment.Spec.Template.Labels["canary.kcp.io/version"] = canary.Spec.StableVersion
		deployment.Spec.Template.Labels["canary.kcp.io/type"] = "stable"

		// Update selector to include stable version
		if deployment.Spec.Selector.MatchLabels == nil {
			deployment.Spec.Selector.MatchLabels = make(map[string]string)
		}
		deployment.Spec.Selector.MatchLabels["canary.kcp.io/version"] = canary.Spec.StableVersion
		deployment.Spec.Selector.MatchLabels["canary.kcp.io/type"] = "stable"

		_, err := tm.client.AppsV1().Deployments(deployment.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update stable deployment: %w", err)
		}

		klog.V(3).Infof("Updated stable deployment %s/%s to %d replicas", deployment.Namespace, deployment.Name, stableReplicas)
	}

	return nil
}

// ensureCanaryDeployment ensures the canary deployment exists with the specified replica count.
func (tm *trafficManager) ensureCanaryDeployment(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, baseDeployment *appsv1.Deployment, replicas int32) error {
	canaryName := fmt.Sprintf("%s-canary", baseDeployment.Name)
	
	// Check if canary deployment already exists
	existing, err := tm.client.AppsV1().Deployments(baseDeployment.Namespace).Get(ctx, canaryName, metav1.GetOptions{})
	if err == nil {
		// Update existing canary deployment
		existing.Spec.Replicas = &replicas
		_, err := tm.client.AppsV1().Deployments(baseDeployment.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update canary deployment: %w", err)
		}
		klog.V(3).Infof("Updated canary deployment %s/%s to %d replicas", existing.Namespace, existing.Name, replicas)
		return nil
	}

	// Create new canary deployment
	canaryDeployment := tm.createCanaryDeployment(canary, baseDeployment, canaryName, replicas)
	
	_, err = tm.client.AppsV1().Deployments(baseDeployment.Namespace).Create(ctx, canaryDeployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create canary deployment: %w", err)
	}

	klog.V(2).Infof("Created canary deployment %s/%s with %d replicas", baseDeployment.Namespace, canaryName, replicas)
	return nil
}

// createCanaryDeployment creates a new canary deployment based on the base deployment.
func (tm *trafficManager) createCanaryDeployment(canary *deploymentv1alpha1.CanaryDeployment, baseDeployment *appsv1.Deployment, canaryName string, replicas int32) *appsv1.Deployment {
	// Deep copy the base deployment
	canaryDeployment := baseDeployment.DeepCopy()
	
	// Update metadata
	canaryDeployment.Name = canaryName
	canaryDeployment.ResourceVersion = ""
	canaryDeployment.UID = ""
	canaryDeployment.CreationTimestamp = metav1.Time{}
	
	// Add canary labels
	if canaryDeployment.Labels == nil {
		canaryDeployment.Labels = make(map[string]string)
	}
	canaryDeployment.Labels["canary.kcp.io/name"] = canary.Name
	canaryDeployment.Labels["canary.kcp.io/version"] = canary.Spec.CanaryVersion
	canaryDeployment.Labels["canary.kcp.io/type"] = "canary"
	
	// Update spec
	canaryDeployment.Spec.Replicas = &replicas
	
	// Update pod template labels
	if canaryDeployment.Spec.Template.Labels == nil {
		canaryDeployment.Spec.Template.Labels = make(map[string]string)
	}
	canaryDeployment.Spec.Template.Labels["canary.kcp.io/version"] = canary.Spec.CanaryVersion
	canaryDeployment.Spec.Template.Labels["canary.kcp.io/type"] = "canary"
	
	// Update selector
	if canaryDeployment.Spec.Selector.MatchLabels == nil {
		canaryDeployment.Spec.Selector.MatchLabels = make(map[string]string)
	}
	canaryDeployment.Spec.Selector.MatchLabels["canary.kcp.io/version"] = canary.Spec.CanaryVersion
	canaryDeployment.Spec.Selector.MatchLabels["canary.kcp.io/type"] = "canary"
	
	// TODO: Update image tag to canary version if needed
	// This would typically involve parsing and updating container images
	
	return canaryDeployment
}

// getCurrentReplicaCounts gets the current replica counts for canary and stable versions.
func (tm *trafficManager) getCurrentReplicaCounts(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, baseDeployment *appsv1.Deployment) (canaryReplicas, stableReplicas int32, err error) {
	// Get stable replicas from base deployment
	stableReplicas = baseDeployment.Status.ReadyReplicas
	
	// Get canary replicas from canary deployment if it exists
	canaryName := fmt.Sprintf("%s-canary", baseDeployment.Name)
	canaryDeployment, err := tm.client.AppsV1().Deployments(baseDeployment.Namespace).Get(ctx, canaryName, metav1.GetOptions{})
	if err != nil {
		// Canary deployment doesn't exist, so 0 canary replicas
		canaryReplicas = 0
	} else {
		canaryReplicas = canaryDeployment.Status.ReadyReplicas
	}

	return canaryReplicas, stableReplicas, nil
}

// updateServiceTraffic updates services to split traffic between versions.
func (tm *trafficManager) updateServiceTraffic(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, canaryWeight int) error {
	namespace := canary.Spec.TargetRef.Namespace
	if namespace == "" {
		namespace = canary.Namespace
	}

	// Find services that select the target deployment
	services, err := tm.findServicesForDeployment(ctx, canary, namespace)
	if err != nil {
		return fmt.Errorf("failed to find services: %w", err)
	}

	if len(services) == 0 {
		klog.V(3).Infof("No services found for deployment %s/%s", namespace, canary.Spec.TargetRef.Name)
		return nil
	}

	// Update each service to handle traffic splitting
	for _, service := range services {
		if err := tm.updateServiceSelectors(ctx, &service, canary, canaryWeight); err != nil {
			klog.Errorf("Failed to update service %s/%s: %v", service.Namespace, service.Name, err)
		}
	}

	return nil
}

// findServicesForDeployment finds services that select the target deployment.
func (tm *trafficManager) findServicesForDeployment(ctx context.Context, canary *deploymentv1alpha1.CanaryDeployment, namespace string) ([]corev1.Service, error) {
	serviceList, err := tm.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var matchingServices []corev1.Service
	
	// Get the deployment to check its labels
	deployment, err := tm.getTargetDeployment(ctx, canary)
	if err != nil {
		return nil, fmt.Errorf("failed to get target deployment: %w", err)
	}

	for _, service := range serviceList.Items {
		if tm.serviceMatchesDeployment(&service, deployment) {
			matchingServices = append(matchingServices, service)
		}
	}

	return matchingServices, nil
}

// serviceMatchesDeployment checks if a service's selector matches a deployment's pod template.
func (tm *trafficManager) serviceMatchesDeployment(service *corev1.Service, deployment *appsv1.Deployment) bool {
	if len(service.Spec.Selector) == 0 {
		return false
	}

	deploymentLabels := deployment.Spec.Template.Labels
	serviceSelector := labels.Set(service.Spec.Selector)

	return serviceSelector.AsSelector().Matches(labels.Set(deploymentLabels))
}

// updateServiceSelectors updates service selectors to handle traffic splitting.
func (tm *trafficManager) updateServiceSelectors(ctx context.Context, service *corev1.Service, canary *deploymentv1alpha1.CanaryDeployment, canaryWeight int) error {
	// For now, we'll use a simple approach: create separate services for canary and stable
	// In a production system, this might use service mesh features or ingress controllers
	
	if canaryWeight > 0 {
		// Ensure canary service exists
		if err := tm.ensureCanaryService(ctx, service, canary); err != nil {
			return fmt.Errorf("failed to ensure canary service: %w", err)
		}
	}

	// Update the main service to only route to stable pods
	if service.Spec.Selector == nil {
		service.Spec.Selector = make(map[string]string)
	}
	service.Spec.Selector["canary.kcp.io/type"] = "stable"

	_, err := tm.client.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}

	return nil
}

// ensureCanaryService ensures a service exists for canary traffic.
func (tm *trafficManager) ensureCanaryService(ctx context.Context, baseService *corev1.Service, canary *deploymentv1alpha1.CanaryDeployment) error {
	canaryServiceName := fmt.Sprintf("%s-canary", baseService.Name)
	
	// Check if canary service already exists
	_, err := tm.client.CoreV1().Services(baseService.Namespace).Get(ctx, canaryServiceName, metav1.GetOptions{})
	if err == nil {
		return nil // Service already exists
	}

	// Create canary service
	canaryService := baseService.DeepCopy()
	canaryService.Name = canaryServiceName
	canaryService.ResourceVersion = ""
	canaryService.UID = ""
	canaryService.CreationTimestamp = metav1.Time{}

	// Update selector to target canary pods
	if canaryService.Spec.Selector == nil {
		canaryService.Spec.Selector = make(map[string]string)
	}
	canaryService.Spec.Selector["canary.kcp.io/type"] = "canary"

	_, err = tm.client.CoreV1().Services(baseService.Namespace).Create(ctx, canaryService, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create canary service: %w", err)
	}

	klog.V(2).Infof("Created canary service %s/%s", baseService.Namespace, canaryServiceName)
	return nil
}