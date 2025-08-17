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
	"crypto/x509"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// DefaultClientBuilder provides default implementation for ClientBuilder interface.
type DefaultClientBuilder struct{}

// NewDefaultClientBuilder creates a new default client builder.
func NewDefaultClientBuilder() *DefaultClientBuilder {
	return &DefaultClientBuilder{}
}

// BuildClient creates a kubernetes client from kubeconfig data.
func (b *DefaultClientBuilder) BuildClient(kubeconfigData []byte) (kubernetes.Interface, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return client, nil
}

// BuildDiscoveryClient creates a discovery client from kubeconfig data.
func (b *DefaultClientBuilder) BuildDiscoveryClient(kubeconfigData []byte) (discovery.DiscoveryInterface, error) {
	client, err := b.BuildClient(kubeconfigData)
	if err != nil {
		return nil, err
	}

	return client.Discovery(), nil
}

// DefaultCertificateValidator provides default implementation for CertificateValidator interface.
type DefaultCertificateValidator struct{}

// NewDefaultCertificateValidator creates a new default certificate validator.
func NewDefaultCertificateValidator() *DefaultCertificateValidator {
	return &DefaultCertificateValidator{}
}

// ValidateCertificate validates a certificate authority certificate.
func (v *DefaultCertificateValidator) ValidateCertificate(certData []byte) error {
	if len(certData) == 0 {
		return fmt.Errorf("certificate data is empty")
	}

	// Parse the certificate
	certs, err := x509.ParseCertificates(certData)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	if len(certs) == 0 {
		return fmt.Errorf("no certificates found in data")
	}

	// Validate the first certificate (CA)
	cert := certs[0]
	if !cert.IsCA {
		return fmt.Errorf("certificate is not a CA certificate")
	}

	// Check if certificate is expired
	if cert.NotAfter.Before(metav1.Now().Time) {
		return fmt.Errorf("certificate is expired")
	}

	return nil
}

// ValidateCertificateChain validates a full certificate chain.
func (v *DefaultCertificateValidator) ValidateCertificateChain(chainData []byte) error {
	if len(chainData) == 0 {
		return fmt.Errorf("certificate chain data is empty")
	}

	certs, err := x509.ParseCertificates(chainData)
	if err != nil {
		return fmt.Errorf("failed to parse certificate chain: %w", err)
	}

	if len(certs) == 0 {
		return fmt.Errorf("no certificates found in chain")
	}

	// Validate each certificate in the chain
	for i, cert := range certs {
		if cert.NotAfter.Before(metav1.Now().Time) {
			return fmt.Errorf("certificate %d in chain is expired", i)
		}
	}

	return nil
}

// DefaultRBACManager provides default implementation for RBACManager interface.
type DefaultRBACManager struct{}

// NewDefaultRBACManager creates a new default RBAC manager.
func NewDefaultRBACManager() *DefaultRBACManager {
	return &DefaultRBACManager{}
}

// SetupSyncerRBAC creates necessary RBAC resources for syncer.
func (r *DefaultRBACManager) SetupSyncerRBAC(ctx context.Context, cluster *ClusterRegistration, client kubernetes.Interface) error {
	logger := klog.FromContext(ctx)
	
	// Create service account for syncer
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateSyncerServiceAccountName(cluster),
			Namespace: getSyncerNamespace(cluster),
			Labels: map[string]string{
				"app.kubernetes.io/name":      "kcp-syncer",
				"app.kubernetes.io/instance":  cluster.Name,
				"app.kubernetes.io/component": "syncer",
				"kcp.io/cluster":              cluster.Name,
			},
		},
	}

	_, err := client.CoreV1().ServiceAccounts(sa.Namespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create syncer service account: %w", err)
	}

	// Create cluster role for syncer
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: generateSyncerClusterRoleName(cluster),
			Labels: map[string]string{
				"app.kubernetes.io/name":      "kcp-syncer",
				"app.kubernetes.io/instance":  cluster.Name,
				"app.kubernetes.io/component": "syncer",
				"kcp.io/cluster":              cluster.Name,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}

	_, err = client.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create syncer cluster role: %w", err)
	}

	// Create cluster role binding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: generateSyncerClusterRoleBindingName(cluster),
			Labels: map[string]string{
				"app.kubernetes.io/name":      "kcp-syncer",
				"app.kubernetes.io/instance":  cluster.Name,
				"app.kubernetes.io/component": "syncer",
				"kcp.io/cluster":              cluster.Name,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
	}

	_, err = client.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create syncer cluster role binding: %w", err)
	}

	logger.V(2).Info("RBAC setup completed", 
		"cluster", cluster.Name,
		"serviceAccount", sa.Name,
		"clusterRole", clusterRole.Name)

	return nil
}

// CleanupSyncerRBAC removes RBAC resources when cluster is deleted.
func (r *DefaultRBACManager) CleanupSyncerRBAC(ctx context.Context, cluster *ClusterRegistration, client kubernetes.Interface) error {
	logger := klog.FromContext(ctx)

	// Delete cluster role binding
	crbName := generateSyncerClusterRoleBindingName(cluster)
	err := client.RbacV1().ClusterRoleBindings().Delete(ctx, crbName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to delete cluster role binding", "name", crbName)
	}

	// Delete cluster role
	crName := generateSyncerClusterRoleName(cluster)
	err = client.RbacV1().ClusterRoles().Delete(ctx, crName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to delete cluster role", "name", crName)
	}

	// Delete service account
	saName := generateSyncerServiceAccountName(cluster)
	saNamespace := getSyncerNamespace(cluster)
	err = client.CoreV1().ServiceAccounts(saNamespace).Delete(ctx, saName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to delete service account", "name", saName, "namespace", saNamespace)
	}

	logger.V(2).Info("RBAC cleanup completed", "cluster", cluster.Name)
	return nil
}

// DefaultSyncTargetManager provides default implementation for SyncTargetManager interface.
type DefaultSyncTargetManager struct{}

// NewDefaultSyncTargetManager creates a new default SyncTarget manager.
func NewDefaultSyncTargetManager() *DefaultSyncTargetManager {
	return &DefaultSyncTargetManager{}
}

// CreateSyncTarget creates a SyncTarget for the cluster.
func (m *DefaultSyncTargetManager) CreateSyncTarget(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Creating SyncTarget", "cluster", cluster.Name)
	
	// TODO: Implement actual SyncTarget creation when SyncTarget API is available
	// This is a placeholder that will be implemented in integration with Phase 5 APIs
	
	return nil
}

// UpdateSyncTarget updates an existing SyncTarget.
func (m *DefaultSyncTargetManager) UpdateSyncTarget(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Updating SyncTarget", "cluster", cluster.Name)
	
	// TODO: Implement actual SyncTarget update when SyncTarget API is available
	// This is a placeholder that will be implemented in integration with Phase 5 APIs
	
	return nil
}

// DeleteSyncTarget removes a SyncTarget.
func (m *DefaultSyncTargetManager) DeleteSyncTarget(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Deleting SyncTarget", "cluster", cluster.Name)
	
	// TODO: Implement actual SyncTarget deletion when SyncTarget API is available
	// This is a placeholder that will be implemented in integration with Phase 5 APIs
	
	return nil
}

// DefaultPlacementNotifier provides default implementation for PlacementNotifier interface.
type DefaultPlacementNotifier struct{}

// NewDefaultPlacementNotifier creates a new default placement notifier.
func NewDefaultPlacementNotifier() *DefaultPlacementNotifier {
	return &DefaultPlacementNotifier{}
}

// NotifyClusterAdded notifies that a cluster was added.
func (n *DefaultPlacementNotifier) NotifyClusterAdded(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Notifying placement system: cluster added", "cluster", cluster.Name)
	
	// TODO: Implement actual placement system notification when placement APIs are available
	// This is a placeholder that will be implemented in integration with placement system
	
	return nil
}

// NotifyClusterUpdated notifies that a cluster was updated.
func (n *DefaultPlacementNotifier) NotifyClusterUpdated(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Notifying placement system: cluster updated", "cluster", cluster.Name)
	
	// TODO: Implement actual placement system notification when placement APIs are available
	// This is a placeholder that will be implemented in integration with placement system
	
	return nil
}

// NotifyClusterRemoved notifies that a cluster was removed.
func (n *DefaultPlacementNotifier) NotifyClusterRemoved(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Notifying placement system: cluster removed", "cluster", cluster.Name)
	
	// TODO: Implement actual placement system notification when placement APIs are available
	// This is a placeholder that will be implemented in integration with placement system
	
	return nil
}

// Helper functions

func generateSyncerServiceAccountName(cluster *ClusterRegistration) string {
	return fmt.Sprintf("kcp-syncer-%s", cluster.Name)
}

func generateSyncerClusterRoleName(cluster *ClusterRegistration) string {
	return fmt.Sprintf("kcp-syncer-%s", cluster.Name)
}

func generateSyncerClusterRoleBindingName(cluster *ClusterRegistration) string {
	return fmt.Sprintf("kcp-syncer-%s", cluster.Name)
}

func getSyncerNamespace(cluster *ClusterRegistration) string {
	// Default to kcp-system namespace, but this could be configurable
	return "kcp-system"
}