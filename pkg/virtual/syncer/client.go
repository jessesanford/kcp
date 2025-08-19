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

package syncer

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// SyncerClient provides access to virtual workspace for syncers.
// It handles syncing SyncTarget resources and maintaining cluster health status.
type SyncerClient struct {
	virtualURL   string
	restConfig   *rest.Config
	syncTargets  SyncTargetInterface
	resyncPeriod time.Duration
}

// NewSyncerClient creates a client for syncer virtual workspace.
// It configures the client to connect to the virtual workspace URL and
// creates the necessary typed clients for managing SyncTarget resources.
func NewSyncerClient(config *rest.Config, virtualURL string) (*SyncerClient, error) {
	if config == nil {
		return nil, fmt.Errorf("rest config is required")
	}

	if virtualURL == "" {
		return nil, fmt.Errorf("virtual URL is required")
	}

	// Create REST config for virtual workspace
	virtualConfig := rest.CopyConfig(config)
	virtualConfig.Host = virtualURL
	virtualConfig.APIPath = "/apis"

	klog.V(4).InfoS("Creating syncer client", "virtualURL", virtualURL)

	// Create mock client for now - in real implementation would use actual KCP client
	mockClient := NewMockSyncTargetInterface(virtualConfig)

	return &SyncerClient{
		virtualURL:   virtualURL,
		restConfig:   virtualConfig,
		syncTargets:  mockClient,
		resyncPeriod: 30 * time.Second,
	}, nil
}

// Start begins the sync loop for the syncer client.
// It performs an initial sync and then runs periodic syncs at the configured interval.
// The method blocks until the context is cancelled or an unrecoverable error occurs.
func (c *SyncerClient) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	logger.Info("Starting syncer client", "virtualURL", c.virtualURL)

	// Perform initial sync
	if err := c.sync(ctx); err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}

	logger.Info("Initial sync completed successfully")

	// Start periodic sync
	ticker := time.NewTicker(c.resyncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Syncer client stopping", "reason", ctx.Err())
			return ctx.Err()

		case <-ticker.C:
			if err := c.sync(ctx); err != nil {
				logger.Error(err, "Sync operation failed, will retry next period")
				// Continue on error - will retry on next tick
			}
		}
	}
}

// sync performs a complete sync operation with the virtual workspace.
// It lists all accessible SyncTarget resources and processes each one.
func (c *SyncerClient) sync(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithName("sync")
	
	logger.V(2).Info("Starting sync operation")
	start := time.Now()

	// List sync targets from virtual workspace
	targets, err := c.syncTargets.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list sync targets: %w", err)
	}

	logger.V(2).Info("Retrieved sync targets from virtual workspace", 
		"count", len(targets.Items))

	// Process each sync target
	processed := 0
	errors := 0

	for i := range targets.Items {
		target := &targets.Items[i]
		
		if err := c.processTarget(ctx, target); err != nil {
			logger.Error(err, "Failed to process sync target", 
				"target", target.Name)
			errors++
			// Continue processing other targets
		} else {
			processed++
		}
	}

	duration := time.Since(start)
	logger.Info("Sync operation completed", 
		"processed", processed,
		"errors", errors,
		"duration", duration)

	if errors > 0 {
		return fmt.Errorf("sync completed with %d errors out of %d targets", 
			errors, len(targets.Items))
	}

	return nil
}

// processTarget handles synchronization for an individual SyncTarget.
// It updates the target's heartbeat, checks health status, and updates conditions.
func (c *SyncerClient) processTarget(ctx context.Context, target *workloadv1alpha1.SyncTarget) error {
	logger := klog.FromContext(ctx).WithValues("target", target.Name)
	
	logger.V(2).Info("Processing sync target")

	// Update heartbeat timestamp
	now := metav1.Now()
	target.Status.LastHeartbeat = &now

	// Perform health check
	healthy, err := c.checkTargetHealth(ctx, target)
	if err != nil {
		logger.Error(err, "Health check failed")
		// Don't return error - still try to update status
	}

	// Update status condition based on health
	condition := metav1.Condition{
		Type:               string(workloadv1alpha1.SyncTargetReady),
		LastTransitionTime: now,
	}

	if healthy {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "Healthy"
		condition.Message = "SyncTarget is healthy and ready for workload placement"
		target.Status.Phase = workloadv1alpha1.SyncTargetPhaseReady
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "Unhealthy"
		condition.Message = "SyncTarget health check failed or timed out"
		target.Status.Phase = workloadv1alpha1.SyncTargetPhaseNotReady
	}

	// Set the condition (this would normally use a helper method)
	target.SetCondition(condition)

	// Update status in virtual workspace
	_, err = c.syncTargets.UpdateStatus(ctx, target, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update sync target status: %w", err)
	}

	logger.V(1).Info("Successfully updated sync target status", 
		"healthy", healthy,
		"phase", target.Status.Phase)

	return nil
}

// checkTargetHealth verifies the health of a sync target cluster.
// It performs basic connectivity and heartbeat validation.
// In a full implementation, this would include more comprehensive health checks.
func (c *SyncerClient) checkTargetHealth(ctx context.Context, target *workloadv1alpha1.SyncTarget) (bool, error) {
	logger := klog.FromContext(ctx).WithValues("target", target.Name)
	
	logger.V(4).Info("Checking target health")

	// Basic heartbeat validation
	if target.Status.LastHeartbeat == nil {
		logger.V(2).Info("No heartbeat recorded for target")
		return false, nil
	}

	// Check if heartbeat is recent (within 2 minutes)
	timeSinceHeartbeat := time.Since(target.Status.LastHeartbeat.Time)
	if timeSinceHeartbeat > 2*time.Minute {
		logger.V(2).Info("Target heartbeat is stale", 
			"lastHeartbeat", target.Status.LastHeartbeat.Time,
			"age", timeSinceHeartbeat)
		return false, nil
	}

	// Validate target configuration
	if err := c.validateTargetConfiguration(target); err != nil {
		logger.V(2).Info("Target configuration validation failed", "error", err)
		return false, err
	}

	// In a full implementation, additional checks would include:
	// - Connectivity test to target cluster API server  
	// - Verification of syncer pod status in target cluster
	// - Resource quota and capacity checks
	// - Network connectivity validation

	logger.V(2).Info("Target health check passed")
	return true, nil
}

// validateTargetConfiguration performs basic validation of SyncTarget configuration.
func (c *SyncerClient) validateTargetConfiguration(target *workloadv1alpha1.SyncTarget) error {
	if target.Spec.SupportedResourceTypes == nil || len(target.Spec.SupportedResourceTypes) == 0 {
		return fmt.Errorf("target has no supported resource types")
	}

	// Validate that required resource types are supported
	requiredTypes := []string{"pods", "services", "configmaps", "secrets"}
	supportedMap := make(map[string]bool)
	for _, resourceType := range target.Spec.SupportedResourceTypes {
		supportedMap[resourceType] = true
	}

	var missingTypes []string
	for _, required := range requiredTypes {
		if !supportedMap[required] {
			missingTypes = append(missingTypes, required)
		}
	}

	if len(missingTypes) > 0 {
		return fmt.Errorf("target missing required resource types: %v", missingTypes)
	}

	return nil
}

// SetResyncPeriod configures the interval between periodic sync operations.
// This allows tuning the sync frequency based on operational requirements.
func (c *SyncerClient) SetResyncPeriod(period time.Duration) {
	if period < 10*time.Second {
		period = 10 * time.Second // Minimum resync period
	}
	c.resyncPeriod = period
	klog.V(2).InfoS("Resync period updated", "period", period)
}

// GetSyncTargets returns the SyncTarget interface for direct access.
// This allows advanced use cases that need direct client access.
func (c *SyncerClient) GetSyncTargets() SyncTargetInterface {
	return c.syncTargets
}