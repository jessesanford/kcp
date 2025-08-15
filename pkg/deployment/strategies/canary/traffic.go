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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TrafficManager handles traffic shifting for canary deployments.
type TrafficManager interface {
	// SetTrafficWeight sets the traffic weight for the canary deployment.
	SetTrafficWeight(ctx context.Context, canary *CanaryDeployment, weight int32) error
	
	// GetCurrentTrafficWeight returns the current traffic weight.
	GetCurrentTrafficWeight(ctx context.Context, canary *CanaryDeployment) (int32, error)
	
	// ValidateTrafficConfiguration validates the traffic configuration.
	ValidateTrafficConfiguration(ctx context.Context, canary *CanaryDeployment) error
}

// DefaultTrafficManager implements TrafficManager for Kubernetes services and ingress.
type DefaultTrafficManager struct {
	client         client.Client
	updateInterval time.Duration
	maxRetries     int
}

// TrafficSplit represents a traffic split configuration.
type TrafficSplit struct {
	// CanaryWeight is the percentage of traffic to route to canary.
	CanaryWeight int32 `json:"canaryWeight"`
	// StableWeight is the percentage of traffic to route to stable.
	StableWeight int32 `json:"stableWeight"`
	// LastUpdated tracks when the split was last updated.
	LastUpdated metav1.Time `json:"lastUpdated"`
}

// NewDefaultTrafficManager creates a new default traffic manager.
func NewDefaultTrafficManager(client client.Client) TrafficManager {
	return &DefaultTrafficManager{
		client:         client,
		updateInterval: time.Second * 5,
		maxRetries:     3,
	}
}

// SetTrafficWeight implements TrafficManager.SetTrafficWeight.
func (tm *DefaultTrafficManager) SetTrafficWeight(ctx context.Context, canary *CanaryDeployment, weight int32) error {
	logger := klog.FromContext(ctx).WithValues("canaryName", canary.Name, "targetWeight", weight)
	logger.V(2).Info("Setting traffic weight")

	if err := tm.validateWeight(weight); err != nil {
		return fmt.Errorf("invalid weight: %w", err)
	}

	// Implement traffic shifting with retries
	return wait.PollImmediate(tm.updateInterval, time.Minute*2, func() (bool, error) {
		if err := tm.updateTrafficSplit(ctx, canary, weight); err != nil {
			logger.V(1).Info("Failed to update traffic split, retrying", "error", err)
			return false, nil // Retry
		}
		
		// Verify the traffic split was applied
		currentWeight, err := tm.GetCurrentTrafficWeight(ctx, canary)
		if err != nil {
			logger.V(1).Info("Failed to verify traffic split, retrying", "error", err)
			return false, nil // Retry
		}
		
		if currentWeight == weight {
			logger.V(2).Info("Traffic weight successfully updated", "currentWeight", currentWeight)
			return true, nil // Success
		}
		
		logger.V(1).Info("Traffic weight not yet updated, retrying", 
			"expected", weight, "actual", currentWeight)
		return false, nil // Retry
	})
}

// GetCurrentTrafficWeight implements TrafficManager.GetCurrentTrafficWeight.
func (tm *DefaultTrafficManager) GetCurrentTrafficWeight(ctx context.Context, canary *CanaryDeployment) (int32, error) {
	logger := klog.FromContext(ctx).WithValues("canaryName", canary.Name)
	
	// This would query the actual traffic management resources (Service, Ingress, etc.)
	// For now, we'll use the status from the canary deployment
	return canary.Status.CurrentTrafficPercentage, nil
}

// ValidateTrafficConfiguration implements TrafficManager.ValidateTrafficConfiguration.
func (tm *DefaultTrafficManager) ValidateTrafficConfiguration(ctx context.Context, canary *CanaryDeployment) error {
	logger := klog.FromContext(ctx).WithValues("canaryName", canary.Name)
	logger.V(2).Info("Validating traffic configuration")

	// Validate traffic percentages in spec
	total := int32(0)
	for _, percentage := range canary.Spec.TrafficPercentages {
		if percentage < 0 || percentage > 100 {
			return fmt.Errorf("invalid traffic percentage %d: must be between 0 and 100", percentage)
		}
		if percentage <= total {
			return fmt.Errorf("traffic percentages must be in ascending order, found %d after %d", percentage, total)
		}
		total = percentage
	}

	if len(canary.Spec.TrafficPercentages) == 0 {
		return fmt.Errorf("at least one traffic percentage must be specified")
	}

	// Validate that required resources exist
	return tm.validateRequiredResources(ctx, canary)
}

// validateWeight ensures the weight is valid.
func (tm *DefaultTrafficManager) validateWeight(weight int32) error {
	if weight < 0 || weight > 100 {
		return fmt.Errorf("weight must be between 0 and 100, got %d", weight)
	}
	return nil
}

// updateTrafficSplit updates the actual traffic split configuration.
func (tm *DefaultTrafficManager) updateTrafficSplit(ctx context.Context, canary *CanaryDeployment, weight int32) error {
	logger := klog.FromContext(ctx).WithValues("canaryName", canary.Name, "weight", weight)
	
	// In a real implementation, this would update Service, Ingress, or Istio VirtualService
	// For now, we simulate the operation
	logger.V(2).Info("Updating traffic split configuration")
	
	split := TrafficSplit{
		CanaryWeight: weight,
		StableWeight: 100 - weight,
		LastUpdated:  metav1.Now(),
	}
	
	// This would apply the actual traffic split to infrastructure
	return tm.applyTrafficSplit(ctx, canary, split)
}

// applyTrafficSplit applies the traffic split to the underlying infrastructure.
func (tm *DefaultTrafficManager) applyTrafficSplit(ctx context.Context, canary *CanaryDeployment, split TrafficSplit) error {
	logger := klog.FromContext(ctx).WithValues("canaryName", canary.Name)
	
	// In a real implementation, this would:
	// 1. Update Service weights if using weighted services
	// 2. Update Ingress annotations for traffic splitting
	// 3. Update Istio VirtualService for traffic management
	// 4. Update any other traffic management resources
	
	logger.V(2).Info("Applying traffic split", 
		"canaryWeight", split.CanaryWeight,
		"stableWeight", split.StableWeight,
	)
	
	// Simulate applying the configuration
	time.Sleep(time.Millisecond * 100)
	
	return nil
}

// validateRequiredResources ensures required infrastructure resources exist.
func (tm *DefaultTrafficManager) validateRequiredResources(ctx context.Context, canary *CanaryDeployment) error {
	logger := klog.FromContext(ctx).WithValues("canaryName", canary.Name)
	
	// This would validate that:
	// 1. Required Services exist
	// 2. Ingress controllers are configured
	// 3. Service mesh (if used) is properly configured
	// 4. Load balancers support traffic splitting
	
	logger.V(3).Info("Validating required traffic management resources")
	
	// For now, we'll just check that the canary exists
	if canary.Name == "" {
		return fmt.Errorf("canary deployment name cannot be empty")
	}
	
	return nil
}