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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/apis/conditions/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/apis/third_party/conditions/util/conditions"
)

const (
	// Condition types for cluster registration
	ConditionCredentialsValid  = "CredentialsValid"
	ConditionConnected         = "Connected"
	ConditionCapabilitiesReady = "CapabilitiesReady"
	ConditionSyncTargetReady   = "SyncTargetReady"
	ConditionRBACReady         = "RBACReady"
	ConditionReady             = "Ready"
)

// ClusterManager manages cluster registration operations and lifecycle.
// It coordinates between cluster validation, SyncTarget creation, and 
// placement system integration following KCP patterns.
type ClusterManager struct {
	// kubeClientBuilder creates kubernetes clients for target clusters
	kubeClientBuilder ClientBuilder
	
	// certValidator validates cluster certificates
	certValidator CertificateValidator
	
	// rbacManager handles RBAC setup for syncers
	rbacManager RBACManager
	
	// syncTargetManager handles SyncTarget lifecycle
	syncTargetManager SyncTargetManager
	
	// placementNotifier notifies placement system of cluster changes
	placementNotifier PlacementNotifier
}

// ClientBuilder creates kubernetes clients for target clusters.
type ClientBuilder interface {
	// BuildClient creates a kubernetes client from kubeconfig data
	BuildClient(kubeconfigData []byte) (kubernetes.Interface, error)
	
	// BuildDiscoveryClient creates a discovery client from kubeconfig data
	BuildDiscoveryClient(kubeconfigData []byte) (discovery.DiscoveryInterface, error)
}

// CertificateValidator validates cluster certificates.
type CertificateValidator interface {
	// ValidateCertificate validates a certificate authority certificate
	ValidateCertificate(certData []byte) error
	
	// ValidateCertificateChain validates a full certificate chain
	ValidateCertificateChain(chainData []byte) error
}

// RBACManager handles RBAC setup for syncers.
type RBACManager interface {
	// SetupSyncerRBAC creates necessary RBAC resources for syncer
	SetupSyncerRBAC(ctx context.Context, cluster *ClusterRegistration, client kubernetes.Interface) error
	
	// CleanupSyncerRBAC removes RBAC resources when cluster is deleted
	CleanupSyncerRBAC(ctx context.Context, cluster *ClusterRegistration, client kubernetes.Interface) error
}

// SyncTargetManager handles SyncTarget lifecycle.
type SyncTargetManager interface {
	// CreateSyncTarget creates a SyncTarget for the cluster
	CreateSyncTarget(ctx context.Context, cluster *ClusterRegistration) error
	
	// UpdateSyncTarget updates an existing SyncTarget
	UpdateSyncTarget(ctx context.Context, cluster *ClusterRegistration) error
	
	// DeleteSyncTarget removes a SyncTarget
	DeleteSyncTarget(ctx context.Context, cluster *ClusterRegistration) error
}

// PlacementNotifier notifies the placement system of cluster changes.
type PlacementNotifier interface {
	// NotifyClusterAdded notifies that a cluster was added
	NotifyClusterAdded(ctx context.Context, cluster *ClusterRegistration) error
	
	// NotifyClusterUpdated notifies that a cluster was updated
	NotifyClusterUpdated(ctx context.Context, cluster *ClusterRegistration) error
	
	// NotifyClusterRemoved notifies that a cluster was removed
	NotifyClusterRemoved(ctx context.Context, cluster *ClusterRegistration) error
}

// ClusterCapabilities represents discovered cluster capabilities.
type ClusterCapabilities struct {
	// KubernetesVersion is the Kubernetes version of the cluster
	KubernetesVersion string `json:"kubernetesVersion"`
	
	// APIGroups are the API groups available in the cluster
	APIGroups []metav1.APIGroup `json:"apiGroups"`
	
	// Features are detected cluster features
	Features []string `json:"features"`
	
	// ResourceCapacity represents the cluster's resource capacity
	ResourceCapacity corev1.ResourceList `json:"resourceCapacity,omitempty"`
}

// NewClusterManager creates a new cluster manager with the provided dependencies.
func NewClusterManager(
	kubeClientBuilder ClientBuilder,
	certValidator CertificateValidator,
	rbacManager RBACManager,
	syncTargetManager SyncTargetManager,
	placementNotifier PlacementNotifier,
) *ClusterManager {
	return &ClusterManager{
		kubeClientBuilder: kubeClientBuilder,
		certValidator:     certValidator,
		rbacManager:       rbacManager,
		syncTargetManager: syncTargetManager,
		placementNotifier: placementNotifier,
	}
}

// ReconcileCluster performs the complete cluster registration reconciliation process.
// This follows the phased approach outlined in the Phase 6 implementation plan.
func (m *ClusterManager) ReconcileCluster(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Starting cluster reconciliation", "cluster", cluster.Name)

	// Phase 1: Validate cluster credentials
	if err := m.validateCredentials(ctx, cluster); err != nil {
		conditions.MarkFalse(
			cluster,
			ConditionCredentialsValid,
			"InvalidCredentials",
			conditionsv1alpha1.ConditionSeverityError,
			"Failed to validate credentials: %v", err,
		)
		m.updatePhase(cluster, ClusterRegistrationPhaseFailed)
		return fmt.Errorf("credential validation failed: %w", err)
	}
	conditions.MarkTrue(cluster, ConditionCredentialsValid)

	// Phase 2: Test cluster connectivity
	kubeClient, err := m.testConnectivity(ctx, cluster)
	if err != nil {
		conditions.MarkFalse(
			cluster,
			ConditionConnected,
			"ConnectionFailed",
			conditionsv1alpha1.ConditionSeverityError,
			"Failed to connect: %v", err,
		)
		m.updatePhase(cluster, ClusterRegistrationPhaseFailed)
		return fmt.Errorf("connectivity test failed: %w", err)
	}
	conditions.MarkTrue(cluster, ConditionConnected)

	// Phase 3: Discover cluster capabilities
	capabilities, err := m.discoverCapabilities(ctx, cluster)
	if err != nil {
		logger.Error(err, "Failed to discover capabilities")
		conditions.MarkFalse(
			cluster,
			ConditionCapabilitiesReady,
			"CapabilityDiscoveryFailed",
			conditionsv1alpha1.ConditionSeverityWarning,
			"Failed to discover capabilities: %v", err,
		)
	} else {
		cluster.Status.Capabilities = convertToMap(capabilities)
		conditions.MarkTrue(cluster, ConditionCapabilitiesReady)
	}

	// Phase 4: Setup RBAC for syncer
	if err := m.setupSyncerRBAC(ctx, cluster, kubeClient); err != nil {
		conditions.MarkFalse(
			cluster,
			ConditionRBACReady,
			"RBACSetupFailed",
			conditionsv1alpha1.ConditionSeverityError,
			"Failed to setup RBAC: %v", err,
		)
		return fmt.Errorf("RBAC setup failed: %w", err)
	}
	conditions.MarkTrue(cluster, ConditionRBACReady)

	// Phase 5: Create or update associated SyncTarget
	if err := m.ensureSyncTarget(ctx, cluster); err != nil {
		conditions.MarkFalse(
			cluster,
			ConditionSyncTargetReady,
			"SyncTargetFailed",
			conditionsv1alpha1.ConditionSeverityError,
			"Failed to ensure SyncTarget: %v", err,
		)
		return fmt.Errorf("SyncTarget creation failed: %w", err)
	}
	conditions.MarkTrue(cluster, ConditionSyncTargetReady)

	// Phase 6: Notify placement system
	if err := m.notifyPlacementSystem(ctx, cluster); err != nil {
		logger.Error(err, "Failed to notify placement system")
		// This is not a hard failure, log and continue
	}

	// Mark cluster as ready
	conditions.MarkTrue(cluster, ConditionReady)
	m.updatePhase(cluster, ClusterRegistrationPhaseReady)

	logger.V(2).Info("Cluster reconciliation completed successfully", "cluster", cluster.Name)
	return nil
}

// validateCredentials validates the cluster credentials and kubeconfig.
func (m *ClusterManager) validateCredentials(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Validating cluster credentials", "cluster", cluster.Name)

	// TODO: Get kubeconfig from secret reference in cluster spec
	// For now, this is a placeholder that would be implemented when
	// the full ClusterRegistration API is available
	kubeconfigData := []byte{}
	
	if len(kubeconfigData) == 0 {
		return fmt.Errorf("kubeconfig not found or empty")
	}

	// Parse and validate kubeconfig
	config, err := clientcmd.Load(kubeconfigData)
	if err != nil {
		return fmt.Errorf("invalid kubeconfig: %w", err)
	}

	// Validate certificates if present
	for clusterName, clusterInfo := range config.Clusters {
		if len(clusterInfo.CertificateAuthorityData) > 0 {
			if err := m.certValidator.ValidateCertificate(clusterInfo.CertificateAuthorityData); err != nil {
				return fmt.Errorf("invalid CA certificate for cluster %s: %w", clusterName, err)
			}
		}

		// Validate server URL
		if clusterInfo.Server == "" {
			return fmt.Errorf("server URL not specified for cluster %s", clusterName)
		}
	}

	// Validate contexts and users
	if len(config.Contexts) == 0 {
		return fmt.Errorf("no contexts found in kubeconfig")
	}

	if len(config.AuthInfos) == 0 {
		return fmt.Errorf("no authentication info found in kubeconfig")
	}

	logger.V(3).Info("Credentials validation successful", "cluster", cluster.Name)
	return nil
}

// testConnectivity tests connectivity to the cluster and returns a client.
func (m *ClusterManager) testConnectivity(ctx context.Context, cluster *ClusterRegistration) (kubernetes.Interface, error) {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Testing cluster connectivity", "cluster", cluster.Name)

	// TODO: Get kubeconfig from secret reference in cluster spec
	kubeconfigData := []byte{}
	
	if len(kubeconfigData) == 0 {
		return nil, fmt.Errorf("kubeconfig not available for connectivity test")
	}

	// Build kubernetes client
	client, err := m.kubeClientBuilder.BuildClient(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("failed to build client: %w", err)
	}

	// Test connectivity with a simple API call
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err = client.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to cluster: %w", err)
	}

	// Test basic API access
	_, err = client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	logger.V(3).Info("Connectivity test successful", "cluster", cluster.Name)
	return client, nil
}

// discoverCapabilities discovers the capabilities of the target cluster.
func (m *ClusterManager) discoverCapabilities(ctx context.Context, cluster *ClusterRegistration) (*ClusterCapabilities, error) {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Discovering cluster capabilities", "cluster", cluster.Name)

	// TODO: Get kubeconfig from secret reference in cluster spec
	kubeconfigData := []byte{}
	
	if len(kubeconfigData) == 0 {
		return nil, fmt.Errorf("kubeconfig not available for capability discovery")
	}

	// Build discovery client
	discoveryClient, err := m.kubeClientBuilder.BuildDiscoveryClient(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("failed to build discovery client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Discover API groups
	groups, err := discoveryClient.ServerGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to discover API groups: %w", err)
	}

	// Discover version
	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to discover server version: %w", err)
	}

	capabilities := &ClusterCapabilities{
		KubernetesVersion: version.GitVersion,
		APIGroups:         groups.Groups,
		Features:          m.detectFeatures(groups),
	}

	logger.V(3).Info("Capability discovery successful", 
		"cluster", cluster.Name,
		"version", capabilities.KubernetesVersion,
		"apiGroups", len(capabilities.APIGroups),
		"features", len(capabilities.Features))

	return capabilities, nil
}

// detectFeatures detects cluster features based on available API groups.
func (m *ClusterManager) detectFeatures(groups *metav1.APIGroupList) []string {
	var features []string
	
	apiGroupNames := make(map[string]bool)
	for _, group := range groups.Groups {
		apiGroupNames[group.Name] = true
	}

	// Detect common Kubernetes features
	if apiGroupNames["networking.k8s.io"] {
		features = append(features, "NetworkPolicies")
	}
	if apiGroupNames["storage.k8s.io"] {
		features = append(features, "StorageClasses")
	}
	if apiGroupNames["rbac.authorization.k8s.io"] {
		features = append(features, "RBAC")
	}
	if apiGroupNames["apps"] {
		features = append(features, "Deployments", "StatefulSets", "DaemonSets")
	}
	if apiGroupNames["extensions"] {
		features = append(features, "Ingress")
	}
	if apiGroupNames["autoscaling"] {
		features = append(features, "HorizontalPodAutoscaler")
	}
	if apiGroupNames["policy"] {
		features = append(features, "PodDisruptionBudgets")
	}

	// Detect service mesh features
	if apiGroupNames["istio.io"] {
		features = append(features, "Istio")
	}
	if apiGroupNames["linkerd.io"] {
		features = append(features, "Linkerd")
	}

	return features
}

// setupSyncerRBAC sets up RBAC resources needed for the syncer.
func (m *ClusterManager) setupSyncerRBAC(ctx context.Context, cluster *ClusterRegistration, client kubernetes.Interface) error {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Setting up syncer RBAC", "cluster", cluster.Name)

	if err := m.rbacManager.SetupSyncerRBAC(ctx, cluster, client); err != nil {
		return fmt.Errorf("failed to setup RBAC: %w", err)
	}

	logger.V(3).Info("Syncer RBAC setup successful", "cluster", cluster.Name)
	return nil
}

// ensureSyncTarget creates or updates the associated SyncTarget resource.
func (m *ClusterManager) ensureSyncTarget(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Ensuring SyncTarget", "cluster", cluster.Name)

	// Check if SyncTarget already exists
	if cluster.Status.SyncTargetRef != nil {
		// Update existing SyncTarget
		if err := m.syncTargetManager.UpdateSyncTarget(ctx, cluster); err != nil {
			return fmt.Errorf("failed to update SyncTarget: %w", err)
		}
	} else {
		// Create new SyncTarget
		if err := m.syncTargetManager.CreateSyncTarget(ctx, cluster); err != nil {
			return fmt.Errorf("failed to create SyncTarget: %w", err)
		}
		
		// Update cluster status with SyncTarget reference
		cluster.Status.SyncTargetRef = &ClusterReference{
			Name:      generateSyncTargetName(cluster),
			Namespace: cluster.Namespace,
			Cluster:   logicalcluster.From(cluster).String(),
		}
	}

	logger.V(3).Info("SyncTarget ensured successfully", "cluster", cluster.Name)
	return nil
}

// notifyPlacementSystem notifies the placement system of cluster changes.
func (m *ClusterManager) notifyPlacementSystem(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Notifying placement system", "cluster", cluster.Name)

	// Determine the type of notification based on cluster phase
	switch cluster.Status.Phase {
	case ClusterRegistrationPhaseReady:
		if cluster.Status.SyncTargetRef == nil {
			// New cluster
			return m.placementNotifier.NotifyClusterAdded(ctx, cluster)
		} else {
			// Existing cluster updated
			return m.placementNotifier.NotifyClusterUpdated(ctx, cluster)
		}
	case ClusterRegistrationPhaseFailed:
		// Notify removal if cluster failed
		return m.placementNotifier.NotifyClusterRemoved(ctx, cluster)
	}

	return nil
}

// updatePhase updates the cluster registration phase.
func (m *ClusterManager) updatePhase(cluster *ClusterRegistration, phase ClusterRegistrationPhase) {
	cluster.Status.Phase = phase
}

// generateSyncTargetName generates a SyncTarget name for the cluster.
func generateSyncTargetName(cluster *ClusterRegistration) string {
	return fmt.Sprintf("cluster-%s", cluster.Name)
}

// convertToMap converts ClusterCapabilities to a map for storage in status.
func convertToMap(capabilities *ClusterCapabilities) map[string]string {
	result := make(map[string]string)
	
	result["kubernetes.version"] = capabilities.KubernetesVersion
	result["api.groups.count"] = fmt.Sprintf("%d", len(capabilities.APIGroups))
	result["features"] = strings.Join(capabilities.Features, ",")
	
	// Add resource capacity information
	for resourceName, quantity := range capabilities.ResourceCapacity {
		key := fmt.Sprintf("capacity.%s", resourceName)
		result[key] = quantity.String()
	}
	
	return result
}