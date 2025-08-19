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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	workloadv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/workload/v1alpha1"
)

const (
	// SyncerNamespace is the namespace where syncer deployments are created
	SyncerNamespace = "kcp-syncer-system"

	// SyncerImageName is the default syncer image
	SyncerImageName = "ghcr.io/kcp-dev/kcp/syncer:latest"

	// SyncerPortName is the name of the syncer port
	SyncerPortName = "metrics"

	// SyncerPort is the port number for syncer metrics
	SyncerPort = 8080
)

// DeploymentManager manages syncer deployments in physical clusters
type DeploymentManager struct {
	physicalClient kubernetes.Interface
}

// NewDeploymentManager creates a new DeploymentManager
func NewDeploymentManager(physicalClient kubernetes.Interface) *DeploymentManager {
	return &DeploymentManager{
		physicalClient: physicalClient,
	}
}

// EnsureDeployment ensures a syncer deployment exists and is up-to-date
func (dm *DeploymentManager) EnsureDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
	klog.V(2).Infof("Ensuring syncer deployment for SyncTarget %s in cluster %s", syncTarget.Name, cluster.String())

	// Get the current deployment if it exists
	deploymentName := syncerDeploymentName(syncTarget)
	existing, err := dm.physicalClient.AppsV1().Deployments(SyncerNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create new deployment
			return dm.createDeployment(ctx, cluster, syncTarget)
		}
		return fmt.Errorf("failed to get existing deployment %s: %w", deploymentName, err)
	}

	// Update existing deployment if needed
	return dm.updateDeployment(ctx, cluster, syncTarget, existing)
}

// createDeployment creates a new syncer deployment
func (dm *DeploymentManager) createDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) error {
	deployment := dm.buildDeployment(cluster, syncTarget)

	klog.V(2).Infof("Creating syncer deployment %s for SyncTarget %s", deployment.Name, syncTarget.Name)

	_, err := dm.physicalClient.AppsV1().Deployments(SyncerNamespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create syncer deployment %s: %w", deployment.Name, err)
	}

	klog.V(2).Infof("Successfully created syncer deployment %s", deployment.Name)
	return nil
}

// updateDeployment updates an existing syncer deployment if changes are needed
func (dm *DeploymentManager) updateDeployment(ctx context.Context, cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget, existing *appsv1.Deployment) error {
	desired := dm.buildDeployment(cluster, syncTarget)

	// Check if update is needed
	if deploymentEqual(existing, desired) {
		klog.V(4).Infof("Syncer deployment %s is up-to-date", existing.Name)
		return nil
	}

	klog.V(2).Infof("Updating syncer deployment %s for SyncTarget %s", existing.Name, syncTarget.Name)

	// Copy resource version and other immutable fields
	desired.ResourceVersion = existing.ResourceVersion
	desired.UID = existing.UID
	desired.CreationTimestamp = existing.CreationTimestamp

	_, err := dm.physicalClient.AppsV1().Deployments(SyncerNamespace).Update(ctx, desired, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update syncer deployment %s: %w", desired.Name, err)
	}

	klog.V(2).Infof("Successfully updated syncer deployment %s", desired.Name)
	return nil
}

// buildDeployment constructs a syncer deployment specification
func (dm *DeploymentManager) buildDeployment(cluster logicalcluster.Path, syncTarget *workloadv1alpha1.SyncTarget) *appsv1.Deployment {
	labels := syncerLabels(syncTarget)
	replicas := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      syncerDeploymentName(syncTarget),
			Namespace: SyncerNamespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: syncTarget.APIVersion,
					Kind:       syncTarget.Kind,
					Name:       syncTarget.Name,
					UID:        syncTarget.UID,
				},
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: syncerServiceAccountName(syncTarget),
					Containers: []corev1.Container{
						{
							Name:  "syncer",
							Image: SyncerImageName,
							Args: []string{
								"syncer",
								"--cluster", cluster.String(),
								"--sync-target", syncTarget.Name,
								"--metrics-bind-address", fmt.Sprintf(":%d", SyncerPort),
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          SyncerPortName,
									ContainerPort: SyncerPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromInt(SyncerPort),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       10,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(SyncerPort),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       30,
							},
						},
					},
				},
			},
		},
	}
}

// DeleteDeployment deletes the syncer deployment for a SyncTarget
func (dm *DeploymentManager) DeleteDeployment(ctx context.Context, syncTarget *workloadv1alpha1.SyncTarget) error {
	deploymentName := syncerDeploymentName(syncTarget)

	klog.V(2).Infof("Deleting syncer deployment %s for SyncTarget %s", deploymentName, syncTarget.Name)

	err := dm.physicalClient.AppsV1().Deployments(SyncerNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete syncer deployment %s: %w", deploymentName, err)
	}

	klog.V(2).Infof("Successfully deleted syncer deployment %s", deploymentName)
	return nil
}

// Helper functions

// syncerDeploymentName generates the deployment name for a SyncTarget
func syncerDeploymentName(syncTarget *workloadv1alpha1.SyncTarget) string {
	return fmt.Sprintf("syncer-%s", syncTarget.Name)
}

// syncerServiceAccountName generates the service account name for a SyncTarget
func syncerServiceAccountName(syncTarget *workloadv1alpha1.SyncTarget) string {
	return fmt.Sprintf("syncer-%s", syncTarget.Name)
}

// syncerLabels generates the labels for syncer resources
func syncerLabels(syncTarget *workloadv1alpha1.SyncTarget) map[string]string {
	return map[string]string{
		"app":         "syncer",
		"sync-target": syncTarget.Name,
		"component":   "syncer",
		"part-of":     "kcp",
	}
}

// deploymentEqual compares two deployments for equality (ignoring status and metadata)
func deploymentEqual(a, b *appsv1.Deployment) bool {
	// Compare key fields that matter for updates
	if *a.Spec.Replicas != *b.Spec.Replicas {
		return false
	}

	// Compare container images
	aContainers := a.Spec.Template.Spec.Containers
	bContainers := b.Spec.Template.Spec.Containers

	if len(aContainers) != len(bContainers) {
		return false
	}

	for i, aContainer := range aContainers {
		bContainer := bContainers[i]
		if aContainer.Image != bContainer.Image {
			return false
		}
	}

	// Compare labels
	for key, aValue := range a.Labels {
		if bValue, ok := b.Labels[key]; !ok || aValue != bValue {
			return false
		}
	}

	return true
}