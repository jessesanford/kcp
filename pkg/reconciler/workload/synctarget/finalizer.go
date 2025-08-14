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

	"github.com/kcp-dev/logicalcluster/v3"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

const (
	// SyncTargetFinalizer is the finalizer used to ensure proper cleanup
	SyncTargetFinalizer = "workload.kcp.io/synctarget"
)

// FinalizerManager handles finalizer operations for SyncTarget resources
type FinalizerManager struct {
	kcpClient      kcpclientset.ClusterInterface
	physicalClient kubernetes.Interface
	deploymentMgr  *DeploymentManager
}

// NewFinalizerManager creates a new FinalizerManager
func NewFinalizerManager(kcpClient kcpclientset.ClusterInterface, physicalClient kubernetes.Interface, deploymentMgr *DeploymentManager) *FinalizerManager {
	return &FinalizerManager{
		kcpClient:      kcpClient,
		physicalClient: physicalClient,
		deploymentMgr:  deploymentMgr,
	}
}

// EnsureFinalizer ensures the SyncTarget finalizer is present
func (fm *FinalizerManager) EnsureFinalizer(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
	// Check if finalizer already exists
	if hasFinalizer(syncTarget, SyncTargetFinalizer) {
		klog.V(4).Infof("Finalizer %s already present on SyncTarget %s", SyncTargetFinalizer, syncTarget.Name)
		return nil
	}

	klog.V(2).Infof("Adding finalizer %s to SyncTarget %s", SyncTargetFinalizer, syncTarget.Name)

	// Add finalizer to the SyncTarget
	syncTargetCopy := syncTarget.DeepCopy()
	syncTargetCopy.Finalizers = append(syncTargetCopy.Finalizers, SyncTargetFinalizer)

	_, err := fm.kcpClient.Cluster(cluster).WorkloadV1alpha1().SyncTargets().Update(ctx, syncTargetCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to add finalizer to SyncTarget %s: %w", syncTarget.Name, err)
	}

	klog.V(2).Infof("Successfully added finalizer %s to SyncTarget %s", SyncTargetFinalizer, syncTarget.Name)
	return nil
}

// RemoveFinalizer removes the SyncTarget finalizer
func (fm *FinalizerManager) RemoveFinalizer(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
	// Check if finalizer exists
	if !hasFinalizer(syncTarget, SyncTargetFinalizer) {
		klog.V(4).Infof("Finalizer %s not present on SyncTarget %s", SyncTargetFinalizer, syncTarget.Name)
		return nil
	}

	klog.V(2).Infof("Removing finalizer %s from SyncTarget %s", SyncTargetFinalizer, syncTarget.Name)

	// Remove finalizer from the SyncTarget
	syncTargetCopy := syncTarget.DeepCopy()
	syncTargetCopy.Finalizers = removeFinalizer(syncTargetCopy.Finalizers, SyncTargetFinalizer)

	_, err := fm.kcpClient.Cluster(cluster).WorkloadV1alpha1().SyncTargets().Update(ctx, syncTargetCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from SyncTarget %s: %w", syncTarget.Name, err)
	}

	klog.V(2).Infof("Successfully removed finalizer %s from SyncTarget %s", SyncTargetFinalizer, syncTarget.Name)
	return nil
}

// HandleDeletion handles the deletion process for a SyncTarget
func (fm *FinalizerManager) HandleDeletion(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
	klog.V(2).Infof("Handling deletion of SyncTarget %s in cluster %s", syncTarget.Name, cluster.String())

	// Check if our finalizer is present
	if !hasFinalizer(syncTarget, SyncTargetFinalizer) {
		klog.V(4).Infof("Finalizer %s not present, skipping cleanup for SyncTarget %s", SyncTargetFinalizer, syncTarget.Name)
		return nil
	}

	// Perform cleanup
	if err := fm.cleanupResources(ctx, cluster, syncTarget); err != nil {
		return fmt.Errorf("failed to cleanup resources for SyncTarget %s: %w", syncTarget.Name, err)
	}

	// Remove the finalizer
	return fm.RemoveFinalizer(ctx, cluster, syncTarget)
}

// CleanupResources performs cleanup of all resources associated with the SyncTarget
func (fm *FinalizerManager) cleanupResources(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
	klog.V(2).Infof("Cleaning up resources for SyncTarget %s", syncTarget.Name)

	// Clean up deployment
	if err := fm.deploymentMgr.DeleteDeployment(ctx, syncTarget); err != nil {
		return fmt.Errorf("failed to cleanup deployment for SyncTarget %s: %w", syncTarget.Name, err)
	}

	// Clean up service account
	if err := fm.cleanupServiceAccount(ctx, syncTarget); err != nil {
		return fmt.Errorf("failed to cleanup service account for SyncTarget %s: %w", syncTarget.Name, err)
	}

	// Clean up config maps
	if err := fm.cleanupConfigMaps(ctx, syncTarget); err != nil {
		return fmt.Errorf("failed to cleanup config maps for SyncTarget %s: %w", syncTarget.Name, err)
	}

	// Clean up secrets
	if err := fm.cleanupSecrets(ctx, syncTarget); err != nil {
		return fmt.Errorf("failed to cleanup secrets for SyncTarget %s: %w", syncTarget.Name, err)
	}

	klog.V(2).Infof("Successfully cleaned up resources for SyncTarget %s", syncTarget.Name)
	return nil
}

// cleanupServiceAccount removes the service account for the SyncTarget
func (fm *FinalizerManager) cleanupServiceAccount(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) error {
	serviceAccountName := syncerServiceAccountName(syncTarget)

	klog.V(2).Infof("Cleaning up service account %s for SyncTarget %s", serviceAccountName, syncTarget.Name)

	err := fm.physicalClient.CoreV1().ServiceAccounts(SyncerNamespace).Delete(ctx, serviceAccountName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete service account %s: %w", serviceAccountName, err)
	}

	return nil
}

// cleanupConfigMaps removes config maps associated with the SyncTarget
func (fm *FinalizerManager) cleanupConfigMaps(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) error {
	labels := syncerLabels(syncTarget)
	labelSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: labels})

	klog.V(2).Infof("Cleaning up config maps for SyncTarget %s with labels %s", syncTarget.Name, labelSelector)

	err := fm.physicalClient.CoreV1().ConfigMaps(SyncerNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete config maps with selector %s: %w", labelSelector, err)
	}

	return nil
}

// cleanupSecrets removes secrets associated with the SyncTarget
func (fm *FinalizerManager) cleanupSecrets(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) error {
	labels := syncerLabels(syncTarget)
	labelSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: labels})

	klog.V(2).Infof("Cleaning up secrets for SyncTarget %s with labels %s", syncTarget.Name, labelSelector)

	err := fm.physicalClient.CoreV1().Secrets(SyncerNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete secrets with selector %s: %w", labelSelector, err)
	}

	return nil
}

// Utility functions

// hasFinalizer checks if a finalizer is present on the SyncTarget
func hasFinalizer(syncTarget *workloadv1alpha1.SyncTarget, finalizer string) bool {
	finalizers := sets.NewString(syncTarget.Finalizers...)
	return finalizers.Has(finalizer)
}

// removeFinalizer removes a finalizer from the list
func removeFinalizer(finalizers []string, finalizer string) []string {
	result := make([]string, 0, len(finalizers))
	for _, f := range finalizers {
		if f != finalizer {
			result = append(result, f)
		}
	}
	return result
}