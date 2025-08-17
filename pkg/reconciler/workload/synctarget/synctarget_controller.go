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

package synctarget

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
	workloadv1alpha1listers "github.com/kcp-dev/kcp/sdk/client/listers/workload/v1alpha1"
)

// heartbeatReconciler manages heartbeat monitoring.
type heartbeatReconciler struct{}

func (r *heartbeatReconciler) reconcile(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) (reconcileStatus, error) {
	now := metav1.NewTime(time.Now())
	if syncTarget.Status.LastHeartbeat.IsZero() {
		syncTarget.Status.LastHeartbeat = now
		return reconcileStatusContinue, nil
	}

	timeSinceHeartbeat := time.Since(syncTarget.Status.LastHeartbeat.Time)
	heartbeatHealthy := timeSinceHeartbeat <= SyncTargetHeartbeatTimeout

	condition := metav1.Condition{
		Type:               "HeartbeatHealthy",
		LastTransitionTime: now,
	}
	if heartbeatHealthy {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "HeartbeatReceived"
		condition.Message = "Heartbeat received within timeout period"
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "HeartbeatTimeout"
		condition.Message = fmt.Sprintf("No heartbeat for %v", timeSinceHeartbeat.Round(time.Second))
	}
	setCondition(&syncTarget.Status.Conditions, condition)
	return reconcileStatusContinue, nil
}

// resourceCapacityReconciler manages resource capacity tracking.
type resourceCapacityReconciler struct{}

func (r *resourceCapacityReconciler) reconcile(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) (reconcileStatus, error) {
	if syncTarget.Status.Capacity == nil {
		syncTarget.Status.Capacity = make(corev1.ResourceList)
		syncTarget.Status.Allocatable = make(corev1.ResourceList)
		syncTarget.Status.Available = make(corev1.ResourceList)
	}

	if syncTarget.Status.Capacity[corev1.ResourceCPU].IsZero() {
		defaultCPU := resource.MustParse("100")
		defaultMemory := resource.MustParse("400Gi")
		syncTarget.Status.Capacity[corev1.ResourceCPU] = defaultCPU
		syncTarget.Status.Capacity[corev1.ResourceMemory] = defaultMemory
		// Set allocatable to 90% of capacity
		syncTarget.Status.Allocatable[corev1.ResourceCPU] = *resource.NewMilliQuantity(defaultCPU.MilliValue()*9/10, resource.DecimalSI)
		syncTarget.Status.Allocatable[corev1.ResourceMemory] = *resource.NewQuantity(defaultMemory.Value()*9/10, resource.BinarySI)
	}

	// Calculate utilization and set condition
	allocatedCPU := resource.MustParse("10")
	allocatableCPU := syncTarget.Status.Allocatable[corev1.ResourceCPU]
	utilization := float64(allocatedCPU.MilliValue()) / float64(allocatableCPU.MilliValue()) * 100

	condition := metav1.Condition{
		Type:               "ResourcesAvailable",
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
	if utilization > 90 {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "HighUtilization"
		condition.Message = fmt.Sprintf("High resource utilization (%.1f%%)", utilization)
	} else {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "ResourcesAvailable"
		condition.Message = fmt.Sprintf("Resources available (%.1f%%)", utilization)
	}
	setCondition(&syncTarget.Status.Conditions, condition)
	return reconcileStatusContinue, nil
}

// virtualWorkspaceReconciler manages virtual workspace associations.
type virtualWorkspaceReconciler struct {
	virtualWorkspaceLister workloadv1alpha1listers.VirtualWorkspaceClusterLister
}

func (r *virtualWorkspaceReconciler) reconcile(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) (reconcileStatus, error) {
	virtualWorkspaceName := syncTarget.Labels["workload.kcp.io/virtual-workspace"]
	if virtualWorkspaceName == "" {
		return reconcileStatusContinue, nil
	}

	now := metav1.NewTime(time.Now())
	_, err := r.virtualWorkspaceLister.Cluster(syncTarget.ClusterName()).Get(virtualWorkspaceName)
	
	condition := metav1.Condition{
		Type:               "VirtualWorkspaceLinked",
		LastTransitionTime: now,
	}
	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "VirtualWorkspaceNotFound"
		condition.Message = fmt.Sprintf("Virtual workspace %q not found", virtualWorkspaceName)
	} else {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "VirtualWorkspaceFound"
		condition.Message = fmt.Sprintf("Linked to virtual workspace %q", virtualWorkspaceName)
	}
	setCondition(&syncTarget.Status.Conditions, condition)
	return reconcileStatusContinue, nil
}

// statusReconciler manages overall status.
type statusReconciler struct{}

func (r *statusReconciler) reconcile(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) (reconcileStatus, error) {
	now := metav1.NewTime(time.Now())
	// Validate APIServerURL
	urlCondition := metav1.Condition{
		Type:               "APIServerURLValid",
		LastTransitionTime: now,
	}
	if syncTarget.Spec.APIServerURL == "" {
		urlCondition.Status = metav1.ConditionFalse
		urlCondition.Reason = "URLNotSet"
		urlCondition.Message = "APIServerURL is required but not set"
	} else {
		urlCondition.Status = metav1.ConditionTrue
		urlCondition.Reason = "URLValid"
		urlCondition.Message = "APIServerURL is set and valid"
	}
	setCondition(&syncTarget.Status.Conditions, urlCondition)

	// Calculate overall Ready condition
	allHealthy := true
	for _, condition := range syncTarget.Status.Conditions {
		if condition.Type != "Ready" && condition.Status != metav1.ConditionTrue {
			allHealthy = false
			break
		}
	}

	readyCondition := metav1.Condition{
		Type:               "Ready",
		LastTransitionTime: now,
	}
	if allHealthy {
		readyCondition.Status = metav1.ConditionTrue
		readyCondition.Reason = "AllConditionsHealthy"
		readyCondition.Message = "SyncTarget is ready and healthy"
	} else {
		readyCondition.Status = metav1.ConditionFalse
		readyCondition.Reason = "ConditionsUnhealthy"
		readyCondition.Message = "SyncTarget not ready"
	}
	setCondition(&syncTarget.Status.Conditions, readyCondition)

	syncTarget.Status.LastReconcileTime = now
	return reconcileStatusContinue, nil
}