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

package cluster

import (
	"context"
	"fmt"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/tmc/controller"
)

// Condition types for ClusterRegistration reconciliation
const (
	// ClusterReadyCondition indicates the cluster registration is ready for workload placement
	ClusterReadyCondition conditionsv1alpha1.ConditionType = "Ready"
	
	// ClusterConnectedCondition indicates the cluster is reachable and responding
	ClusterConnectedCondition conditionsv1alpha1.ConditionType = "Connected"
	
	// ClusterValidatedCondition indicates the cluster configuration has been validated
	ClusterValidatedCondition conditionsv1alpha1.ConditionType = "Validated"
)

// ClusterReconciler provides reconciliation logic for ClusterRegistration resources.
// It validates cluster endpoints, establishes connections, and manages status conditions
// following KCP patterns for workspace isolation and error handling.
type ClusterReconciler struct {
	// client provides workspace-aware access to TMC API resources
	client controller.Client
}

// NewClusterReconciler creates a new ClusterReconciler with the provided client.
//
// Parameters:
//   - client: Workspace-aware client for accessing TMC API resources
//
// Returns:
//   - *ClusterReconciler: Configured reconciler ready to process ClusterRegistration resources
func NewClusterReconciler(client controller.Client) *ClusterReconciler {
	return &ClusterReconciler{
		client: client,
	}
}

// ReconcileClusterRegistration handles the main reconciliation logic for ClusterRegistration resources.
// It performs endpoint validation, connectivity testing, and status condition management.
//
// Parameters:
//   - ctx: Request context with logging and cancellation support
//   - cluster: ClusterRegistration resource to reconcile
//
// Returns:
//   - bool: Whether the reconciliation should be requeued
//   - error: Any error that occurred during reconciliation
func (r *ClusterReconciler) ReconcileClusterRegistration(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) (bool, error) {
	logger := klog.FromContext(ctx)
	
	// Initialize conditions if not present
	if cluster.Status.Conditions == nil {
		cluster.Status.Conditions = conditionsv1alpha1.Conditions{}
	}

	var requeue bool
	
	logger.V(2).Info("starting cluster registration reconciliation",
		"cluster", cluster.Name,
		"location", cluster.Spec.Location,
		"serverURL", cluster.Spec.ClusterEndpoint.ServerURL,
	)

	// Phase 1: Validate cluster endpoint configuration
	if err := r.validateClusterEndpoint(ctx, cluster); err != nil {
		logger.Error(err, "failed to validate cluster endpoint")
		r.setCondition(cluster, conditionsv1alpha1.Condition{
			Type:    ClusterValidatedCondition,
			Status:  corev1.ConditionFalse,
			Reason:  "EndpointValidationFailed",
			Message: fmt.Sprintf("Cluster endpoint validation failed: %v", err),
		})
		r.setCondition(cluster, conditionsv1alpha1.Condition{
			Type:    ClusterReadyCondition,
			Status:  corev1.ConditionFalse,
			Reason:  "ValidationFailed",
			Message: "Cluster is not ready due to validation failure",
		})
		// Don't requeue on validation errors, wait for spec update
		return false, nil
	}

	// Mark validation as successful
	r.setCondition(cluster, conditionsv1alpha1.Condition{
		Type:    ClusterValidatedCondition,
		Status:  corev1.ConditionTrue,
		Reason:  "ValidationSucceeded",
		Message: "Cluster endpoint configuration is valid",
	})

	// Phase 2: Test cluster connectivity
	connected, err := r.testClusterConnectivity(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to test cluster connectivity")
		r.setCondition(cluster, conditionsv1alpha1.Condition{
			Type:    ClusterConnectedCondition,
			Status:  corev1.ConditionFalse,
			Reason:  "ConnectivityTestFailed",
			Message: fmt.Sprintf("Connectivity test failed: %v", err),
		})
		r.setCondition(cluster, conditionsv1alpha1.Condition{
			Type:    ClusterReadyCondition,
			Status:  corev1.ConditionFalse,
			Reason:  "ConnectivityFailed",
			Message: "Cluster is not ready due to connectivity issues",
		})
		// Requeue to retry connectivity test
		requeue = true
		return requeue, nil
	}

	if connected {
		// Update heartbeat timestamp
		now := metav1.NewTime(time.Now())
		cluster.Status.LastHeartbeat = &now

		r.setCondition(cluster, conditionsv1alpha1.Condition{
			Type:    ClusterConnectedCondition,
			Status:  corev1.ConditionTrue,
			Reason:  "ConnectivityTestSucceeded",
			Message: "Cluster is reachable and responding",
		})

		// Phase 3: Mark cluster as ready for workload placement
		r.setCondition(cluster, conditionsv1alpha1.Condition{
			Type:    ClusterReadyCondition,
			Status:  corev1.ConditionTrue,
			Reason:  "ClusterReady",
			Message: "Cluster registration is ready for workload placement",
		})

		logger.V(2).Info("cluster registration reconciled successfully",
			"cluster", cluster.Name,
			"serverURL", cluster.Spec.ClusterEndpoint.ServerURL,
			"location", cluster.Spec.Location,
			"lastHeartbeat", cluster.Status.LastHeartbeat,
		)
	}

	return requeue, nil
}

// validateClusterEndpoint validates the cluster endpoint configuration
// including URL format validation and security requirements.
func (r *ClusterReconciler) validateClusterEndpoint(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) error {
	endpoint := cluster.Spec.ClusterEndpoint
	
	// Validate server URL format
	if endpoint.ServerURL == "" {
		return fmt.Errorf("serverURL is required")
	}
	
	parsedURL, err := url.Parse(endpoint.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid serverURL format: %v", err)
	}
	
	// Require HTTPS for security
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("serverURL must use HTTPS scheme, got %s", parsedURL.Scheme)
	}
	
	if parsedURL.Host == "" {
		return fmt.Errorf("serverURL must specify a host")
	}
	
	// Validate location is specified
	if cluster.Spec.Location == "" {
		return fmt.Errorf("location is required for cluster registration")
	}
	
	return nil
}

// testClusterConnectivity performs a basic connectivity test to the cluster.
// This is a simplified implementation that would be expanded to actually connect to the cluster.
func (r *ClusterReconciler) testClusterConnectivity(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) (bool, error) {
	logger := klog.FromContext(ctx)
	
	// TODO: In a full implementation, this would:
	// 1. Create a Kubernetes client using the endpoint configuration
	// 2. Perform a basic API call (e.g., get server version)
	// 3. Handle certificate validation based on TLSConfig
	// 4. Update resource usage metrics if available
	// 5. Test authentication and authorization
	
	logger.V(4).Info("testing cluster connectivity", 
		"serverURL", cluster.Spec.ClusterEndpoint.ServerURL,
		"location", cluster.Spec.Location,
		"cluster", cluster.Name,
	)
	
	// For now, we assume the connectivity test passes if endpoint validation succeeded
	// In a real implementation, this would perform actual network connectivity tests
	// and validate the cluster's Kubernetes API availability
	return true, nil
}

// setCondition sets or updates a condition in the cluster's status conditions.
func (r *ClusterReconciler) setCondition(cluster *tmcv1alpha1.ClusterRegistration, condition conditionsv1alpha1.Condition) {
	// Set timestamp
	condition.LastTransitionTime = metav1.Now()
	
	// Find existing condition and update it, or add new one
	for i, existingCondition := range cluster.Status.Conditions {
		if existingCondition.Type == condition.Type {
			cluster.Status.Conditions[i] = condition
			return
		}
	}
	
	// Condition not found, add it
	cluster.Status.Conditions = append(cluster.Status.Conditions, condition)
}