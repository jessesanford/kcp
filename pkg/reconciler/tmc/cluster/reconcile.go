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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/reconciler/committer"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)


const (
	ClusterReadyCondition         = "Ready"
	ClusterConnectivityCondition  = "Connectivity"
	ClusterAuthorizationCondition = "Authorization"
	ClusterHealthCondition        = "Health"
	
	// Health check intervals
	HealthCheckInterval     = 30 * time.Second
	HealthCheckTimeout      = 10 * time.Second
	ConnectivityTimeout     = 30 * time.Second
	ConnectivityRetryDelay  = 2 * time.Second
	ValidationTimeout       = 10 * time.Second
	ValidationRetryDelay    = time.Second
)

type reconcileStatus int

const (
	reconcileStatusContinue reconcileStatus = iota
	reconcileStatusStop
)

type reconciler struct {
	controller *Controller
}

func newReconciler(controller *Controller) *reconciler {
	return &reconciler{controller: controller}
}

func (r *reconciler) reconcileCluster(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) (reconcileStatus, error) {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("reconciling cluster")

	if cluster.GetDeletionTimestamp() != nil && !cluster.GetDeletionTimestamp().IsZero() {
		return r.handleClusterDeletion(ctx, cluster)
	}

	if err := r.ensureClusterConnectivity(ctx, cluster); err != nil {
		logger.Error(err, "connectivity check failed")
		if updateErr := r.updateClusterStatus(ctx, cluster, ClusterConnectivityCondition, conditionsv1alpha1.ConditionFalse, "ConnectivityError", err.Error()); updateErr != nil {
			logger.Error(updateErr, "failed to update connectivity status")
		}
		return reconcileStatusContinue, err
	}

	if err := r.validateClusterAccess(ctx, cluster); err != nil {
		logger.Error(err, "access validation failed")
		if updateErr := r.updateClusterStatus(ctx, cluster, ClusterAuthorizationCondition, conditionsv1alpha1.ConditionFalse, "AuthorizationError", err.Error()); updateErr != nil {
			logger.Error(updateErr, "failed to update authorization status")
		}
		return reconcileStatusContinue, err
	}

	// Perform health monitoring
	if err := r.performHealthCheck(ctx, cluster); err != nil {
		logger.Error(err, "health check failed")
		if updateErr := r.updateClusterStatus(ctx, cluster, ClusterHealthCondition, conditionsv1alpha1.ConditionFalse, "HealthCheckError", err.Error()); updateErr != nil {
			logger.Error(updateErr, "failed to update health status")
		}
		// Don't return error for health check failures, continue with reconciliation
	} else {
		if updateErr := r.updateClusterStatus(ctx, cluster, ClusterHealthCondition, conditionsv1alpha1.ConditionTrue, "HealthCheckPassed", "Cluster health check passed"); updateErr != nil {
			logger.Error(updateErr, "failed to update health status")
		}
	}

	// Update all positive conditions
	if updateErr := r.updateClusterStatus(ctx, cluster, ClusterReadyCondition, conditionsv1alpha1.ConditionTrue, "ClusterReady", "Cluster ready"); updateErr != nil {
		logger.Error(updateErr, "failed to update ready status")
	}
	if updateErr := r.updateClusterStatus(ctx, cluster, ClusterConnectivityCondition, conditionsv1alpha1.ConditionTrue, "ConnectivityHealthy", "API server accessible"); updateErr != nil {
		logger.Error(updateErr, "failed to update connectivity status")
	}
	if updateErr := r.updateClusterStatus(ctx, cluster, ClusterAuthorizationCondition, conditionsv1alpha1.ConditionTrue, "AuthorizationHealthy", "Authorization configured"); updateErr != nil {
		logger.Error(updateErr, "failed to update authorization status")
	}

	logger.V(2).Info("cluster reconciliation completed")
	return reconcileStatusContinue, nil
}

func (r *reconciler) updateClusterStatus(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration, conditionType string, status conditionsv1alpha1.ConditionStatus, reason, message string) error {
	logger := klog.FromContext(ctx).WithValues("condition", conditionType, "status", status)
	
	// Create the resource for committer pattern
	clusterWorkspace := logicalcluster.From(cluster)
	oldResource := &committer.Resource[tmcv1alpha1.ClusterRegistrationSpec, tmcv1alpha1.ClusterRegistrationStatus]{
		ObjectMeta: cluster.ObjectMeta,
		Spec:       cluster.Spec,
		Status:     cluster.Status,
	}
	newResource := oldResource.DeepCopy()
	newResource.Status.ObservedGeneration = cluster.GetGeneration()
	
	// Update or add the condition
	condition := conditionsv1alpha1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
		ObservedGeneration: cluster.GetGeneration(),
	}
	
	// Find and update existing condition or add new one
	updated := false
	for i, existingCondition := range newResource.Status.Conditions {
		if existingCondition.Type == conditionType {
			newResource.Status.Conditions[i] = condition
			updated = true
			break
		}
	}
	if !updated {
		newResource.Status.Conditions = append(newResource.Status.Conditions, condition)
	}
	
	// Set logical cluster for committer
	logicalcluster.Cluster(clusterWorkspace).Finalize(&oldResource.ObjectMeta)
	logicalcluster.Cluster(clusterWorkspace).Finalize(&newResource.ObjectMeta)
	
	// Commit the status update
	logger.V(3).Info("updating cluster condition", "reason", reason, "message", message)
	return r.controller.commitClusterRegistration(ctx, oldResource, newResource)
}

func (r *reconciler) ensureClusterConnectivity(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	clusterName := cluster.GetName()
	connectivityCtx, cancel := context.WithTimeout(ctx, ConnectivityTimeout)
	defer cancel()

	err := wait.PollUntilContextCancel(connectivityCtx, ConnectivityRetryDelay, true, func(ctx context.Context) (bool, error) {
		// Perform actual connectivity check using cluster endpoint
		client, err := r.getClusterClient(cluster)
		if err != nil {
			logger.V(4).Info("failed to create cluster client", "error", err)
			return false, nil // Don't return error, just retry
		}
		
		// Try to get server version as a connectivity test
		version, err := client.Discovery().ServerVersion()
		if err != nil {
			logger.V(4).Info("server version check failed", "error", err)
			return false, nil // Don't return error, just retry
		}
		
		logger.V(4).Info("connectivity check successful", "serverVersion", version.String())
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("connectivity check failed for %s: %w", clusterName, err)
	}
	return nil
}

func (r *reconciler) validateClusterAccess(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) error {
	clusterName := cluster.GetName()
	validateCtx, cancel := context.WithTimeout(ctx, ValidationTimeout)
	defer cancel()

	err := wait.PollUntilContextCancel(validateCtx, ValidationRetryDelay, true, func(ctx context.Context) (bool, error) {
		// Perform actual access validation
		client, err := r.getClusterClient(cluster)
		if err != nil {
			return false, nil // Don't return error, just retry
		}
		
		// Try to list namespaces as an authorization test
		_, err = client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
		if err != nil {
			return false, nil // Don't return error, just retry
		}
		
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("access validation failed for %s: %w", clusterName, err)
	}
	return nil
}

func (r *reconciler) handleClusterDeletion(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) (reconcileStatus, error) {
	if err := r.performClusterCleanup(ctx, cluster); err != nil {
		klog.FromContext(ctx).Error(err, "cleanup failed")
		return reconcileStatusContinue, err
	}
	// TODO: Remove finalizers when cleanup is complete
	return reconcileStatusStop, nil
}

func (r *reconciler) performClusterCleanup(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) error {
	clusterName := cluster.GetName()
	cleanupCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	err := wait.PollUntilContextCancel(cleanupCtx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		// TODO: Implement actual cleanup:
		// 1. List all workloads on this cluster
		// 2. Reschedule or remove workloads  
		// 3. Update placement decisions
		// 4. Clean up monitoring and metrics
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("cleanup failed for %s: %w", clusterName, err)
	}
	return nil
}

func (r *reconciler) getClusterClient(cluster *tmcv1alpha1.ClusterRegistration) (kubernetes.Interface, error) {
	// Create REST config from cluster endpoint
	config := &rest.Config{
		Host: cluster.Spec.ClusterEndpoint.ServerURL,
	}
	
	// Configure TLS
	if cluster.Spec.ClusterEndpoint.CABundle != nil {
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(cluster.Spec.ClusterEndpoint.CABundle) {
			return nil, fmt.Errorf("failed to parse CA bundle")
		}
		config.TLSClientConfig = rest.TLSClientConfig{
			CAData: cluster.Spec.ClusterEndpoint.CABundle,
		}
	}
	
	// Handle TLS configuration
	if cluster.Spec.ClusterEndpoint.TLSConfig != nil && cluster.Spec.ClusterEndpoint.TLSConfig.InsecureSkipVerify {
		config.TLSClientConfig.Insecure = true
	}
	
	// Set timeouts
	config.Timeout = 10 * time.Second
	
	// Create Kubernetes client
	return kubernetes.NewForConfig(config)
}

// performHealthCheck performs comprehensive health checks on the cluster
func (r *reconciler) performHealthCheck(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	clusterName := cluster.GetName()
	
	healthCtx, cancel := context.WithTimeout(ctx, HealthCheckTimeout)
	defer cancel()

	client, err := r.getClusterClient(cluster)
	if err != nil {
		return fmt.Errorf("failed to create cluster client for health check: %w", err)
	}

	// Perform multiple health checks
	err = wait.PollUntilContextCancel(healthCtx, time.Second, true, func(ctx context.Context) (bool, error) {
		// 1. Check API server health
		version, err := client.Discovery().ServerVersion()
		if err != nil {
			logger.V(4).Info("API server health check failed", "error", err)
			return false, nil
		}
		logger.V(4).Info("API server health check passed", "version", version.String())

		// 2. Check if we can list nodes (basic cluster health)
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 5})
		if err != nil {
			logger.V(4).Info("node list health check failed", "error", err)
			return false, nil
		}
		logger.V(4).Info("node list health check passed", "nodeCount", len(nodes.Items))

		// 3. Check if we can list system namespaces
		systemNamespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
			LabelSelector: "name=kube-system",
			Limit:         1,
		})
		if err != nil {
			logger.V(4).Info("system namespace health check failed", "error", err)
			return false, nil
		}
		if len(systemNamespaces.Items) == 0 {
			logger.V(4).Info("system namespace not found")
			return false, nil
		}
		logger.V(4).Info("system namespace health check passed")

		return true, nil
	})

	if err != nil {
		return fmt.Errorf("health check failed for %s: %w", clusterName, err)
	}
	
	logger.V(3).Info("comprehensive health check completed successfully", "cluster", clusterName)
	return nil
}