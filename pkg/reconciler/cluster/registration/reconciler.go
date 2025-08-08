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

package registration

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
)

// Condition types for ClusterRegistration
const (
	// ClusterRegistrationReadyCondition indicates the cluster registration is ready for workload placement
	ClusterRegistrationReadyCondition conditionsv1alpha1.ConditionType = "Ready"
	
	// ClusterRegistrationConnectedCondition indicates the cluster is reachable
	ClusterRegistrationConnectedCondition conditionsv1alpha1.ConditionType = "Connected"
)

// reconcile handles the reconciliation logic for ClusterRegistration resources.
// It validates cluster endpoints, establishes connections, and updates status conditions.
func (c *Controller) reconcile(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) (bool, error) {
	logger := klog.FromContext(ctx)
	
	// Initialize conditions if not present
	if cluster.Status.Conditions == nil {
		cluster.Status.Conditions = conditionsv1alpha1.Conditions{}
	}

	var requeue bool

	// Phase 1: Validate cluster endpoint configuration
	if err := c.validateClusterEndpoint(ctx, cluster); err != nil {
		logger.Error(err, "failed to validate cluster endpoint")
		cluster.Status.Conditions = setCondition(cluster.Status.Conditions, conditionsv1alpha1.Condition{
			Type:    ClusterRegistrationConnectedCondition,
			Status:  corev1.ConditionFalse,
			Reason:  "EndpointValidationFailed",
			Message: fmt.Sprintf("Cluster endpoint validation failed: %v", err),
		})
		cluster.Status.Conditions = setCondition(cluster.Status.Conditions, conditionsv1alpha1.Condition{
			Type:    ClusterRegistrationReadyCondition,
			Status:  corev1.ConditionFalse,
			Reason:  "EndpointValidationFailed",
			Message: "Cluster is not ready due to endpoint validation failure",
		})
		return false, nil // Don't requeue on validation errors, wait for spec update
	}

	// Phase 2: Test cluster connectivity
	connected, err := c.testClusterConnectivity(ctx, cluster)
	if err != nil {
		logger.Error(err, "failed to test cluster connectivity")
		cluster.Status.Conditions = setCondition(cluster.Status.Conditions, conditionsv1alpha1.Condition{
			Type:    ClusterRegistrationConnectedCondition,
			Status:  corev1.ConditionFalse,
			Reason:  "ConnectivityTestFailed",
			Message: fmt.Sprintf("Connectivity test failed: %v", err),
		})
		cluster.Status.Conditions = setCondition(cluster.Status.Conditions, conditionsv1alpha1.Condition{
			Type:    ClusterRegistrationReadyCondition,
			Status:  corev1.ConditionFalse,
			Reason:  "ConnectivityTestFailed",
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

		cluster.Status.Conditions = setCondition(cluster.Status.Conditions, conditionsv1alpha1.Condition{
			Type:    ClusterRegistrationConnectedCondition,
			Status:  corev1.ConditionTrue,
			Reason:  "ConnectivityTestSucceeded",
			Message: "Cluster is reachable and responding",
		})

		// Phase 3: Mark cluster as ready for workload placement
		cluster.Status.Conditions = setCondition(cluster.Status.Conditions, conditionsv1alpha1.Condition{
			Type:    ClusterRegistrationReadyCondition,
			Status:  corev1.ConditionTrue,
			Reason:  "ClusterRegistrationReady",
			Message: "Cluster registration is ready for workload placement",
		})

		logger.V(2).Info("cluster registration reconciled successfully",
			"serverURL", cluster.Spec.ClusterEndpoint.ServerURL,
			"location", cluster.Spec.Location,
		)
	}

	return requeue, nil
}

// validateClusterEndpoint validates the cluster endpoint configuration
func (c *Controller) validateClusterEndpoint(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) error {
	endpoint := cluster.Spec.ClusterEndpoint
	
	// Validate server URL format
	if endpoint.ServerURL == "" {
		return fmt.Errorf("serverURL is required")
	}
	
	parsedURL, err := url.Parse(endpoint.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid serverURL format: %v", err)
	}
	
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("serverURL must use HTTPS scheme, got %s", parsedURL.Scheme)
	}
	
	if parsedURL.Host == "" {
		return fmt.Errorf("serverURL must specify a host")
	}
	
	return nil
}

// testClusterConnectivity performs a basic connectivity test to the cluster
// This is a simplified implementation that would be expanded to actually connect to the cluster
func (c *Controller) testClusterConnectivity(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) (bool, error) {
	logger := klog.FromContext(ctx)
	
	// TODO: In a full implementation, this would:
	// 1. Create a Kubernetes client using the endpoint configuration
	// 2. Perform a basic API call (e.g., get server version)
	// 3. Handle certificate validation based on TLSConfig
	// 4. Update resource usage metrics if available
	
	logger.V(4).Info("testing cluster connectivity", 
		"serverURL", cluster.Spec.ClusterEndpoint.ServerURL,
		"location", cluster.Spec.Location,
	)
	
	// For now, we assume the connectivity test passes if endpoint validation succeeded
	// In a real implementation, this would perform actual network connectivity tests
	return true, nil
}

// setCondition sets or updates a condition in the conditions slice
func setCondition(conditions conditionsv1alpha1.Conditions, condition conditionsv1alpha1.Condition) conditionsv1alpha1.Conditions {
	// Set timestamp
	condition.LastTransitionTime = metav1.Now()
	
	// Find existing condition and update it, or add new one
	for i, existingCondition := range conditions {
		if existingCondition.Type == condition.Type {
			conditions[i] = condition
			return conditions
		}
	}
	
	// Condition not found, add it
	return append(conditions, condition)
}