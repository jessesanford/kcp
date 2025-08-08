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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

const (
	// CapabilityDetectionTimeout is the timeout for capability detection operations
	CapabilityDetectionTimeout = 30 * time.Second

	// CapabilityDetectionInterval is how often to refresh capabilities
	CapabilityDetectionInterval = 5 * time.Minute
)

// ClusterCapabilityDetector handles detection of cluster capabilities
type ClusterCapabilityDetector struct {
	// clientFactory creates Kubernetes clients for target clusters
	clientFactory ClientFactory
}

// ClientFactory abstracts the creation of Kubernetes clients for testing
type ClientFactory interface {
	CreateClient(endpoint tmcv1alpha1.ClusterEndpoint) (kubernetes.Interface, error)
	CreateDiscoveryClient(endpoint tmcv1alpha1.ClusterEndpoint) (APIDiscoveryClient, error)
}

// DefaultClientFactory implements ClientFactory using standard Kubernetes client-go
type DefaultClientFactory struct{}

// CreateClient creates a Kubernetes client for the given cluster endpoint
func (f *DefaultClientFactory) CreateClient(endpoint tmcv1alpha1.ClusterEndpoint) (kubernetes.Interface, error) {
	config, err := f.buildRestConfig(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to build REST config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return clientset, nil
}

// CreateDiscoveryClient creates a discovery client for the given cluster endpoint
func (f *DefaultClientFactory) CreateDiscoveryClient(endpoint tmcv1alpha1.ClusterEndpoint) (APIDiscoveryClient, error) {
	config, err := f.buildRestConfig(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to build REST config: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	return &discoveryAdapter{discoveryClient}, nil
}

// buildRestConfig creates a REST config from cluster endpoint information
func (f *DefaultClientFactory) buildRestConfig(endpoint tmcv1alpha1.ClusterEndpoint) (*rest.Config, error) {
	config := &rest.Config{
		Host:    endpoint.ServerURL,
		Timeout: CapabilityDetectionTimeout,
	}

	// Configure TLS settings
	if endpoint.CABundle != nil {
		config.TLSClientConfig.CAData = endpoint.CABundle
	}

	if endpoint.TLSConfig != nil && endpoint.TLSConfig.InsecureSkipVerify {
		config.TLSClientConfig.Insecure = true
	}

	// TODO: In a full implementation, this would also handle:
	// - Authentication credentials (service account tokens, certificates)
	// - Custom transport configurations
	// - Connection pooling and retry logic

	return config, nil
}

// discoveryAdapter wraps the discovery client to implement APIDiscoveryClient
type discoveryAdapter struct {
	client *discovery.DiscoveryClient
}

func (a *discoveryAdapter) ServerVersion() (*version.Info, error) {
	return a.client.ServerVersion()
}

func (a *discoveryAdapter) ServerGroups() (*metav1.APIGroupList, error) {
	return a.client.ServerGroups()
}

func (a *discoveryAdapter) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return a.client.ServerPreferredResources()
}

// NewClusterCapabilityDetector creates a new capability detector
func NewClusterCapabilityDetector(clientFactory ClientFactory) *ClusterCapabilityDetector {
	if clientFactory == nil {
		clientFactory = &DefaultClientFactory{}
	}
	return &ClusterCapabilityDetector{
		clientFactory: clientFactory,
	}
}

// DetectCapabilities performs comprehensive capability detection for a cluster
func (d *ClusterCapabilityDetector) DetectCapabilities(ctx context.Context, cluster *tmcv1alpha1.ClusterRegistration) (*tmcv1alpha1.ClusterCapabilities, error) {
	logger := klog.FromContext(ctx)
	logger.V(2).Info("starting capability detection", "cluster", cluster.Name)

	// Create clients for the target cluster
	kubeClient, err := d.clientFactory.CreateClient(cluster.Spec.ClusterEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	discoveryClient, err := d.clientFactory.CreateDiscoveryClient(cluster.Spec.ClusterEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Set up context with timeout
	detectCtx, cancel := context.WithTimeout(ctx, CapabilityDetectionTimeout)
	defer cancel()

	capabilities := &tmcv1alpha1.ClusterCapabilities{}

	// Detect API capabilities
	if err := d.detectAPICapabilities(detectCtx, discoveryClient, capabilities); err != nil {
		logger.Error(err, "failed to detect API capabilities")
		// Don't fail completely on API detection errors
	}

	// Detect resource capacity
	if err := d.detectResourceCapacity(detectCtx, kubeClient, capabilities); err != nil {
		logger.Error(err, "failed to detect resource capacity")
		// Don't fail completely on capacity detection errors
	}

	// Set detection timestamp
	now := metav1.NewTime(time.Now())
	capabilities.LastDetected = &now

	logger.V(2).Info("capability detection completed",
		"cluster", cluster.Name,
		"kubernetesVersion", capabilities.KubernetesVersion,
		"nodeCount", capabilities.NodeCount,
		"featuresCount", len(capabilities.Features),
	)

	return capabilities, nil
}

// detectAPICapabilities discovers API-related capabilities
func (d *ClusterCapabilityDetector) detectAPICapabilities(ctx context.Context, client APIDiscoveryClient, capabilities *tmcv1alpha1.ClusterCapabilities) error {
	logger := klog.FromContext(ctx)

	// Perform API discovery
	discoveryResult, err := PerformAPIDiscovery(ctx, client)
	if err != nil {
		return fmt.Errorf("API discovery failed: %w", err)
	}

	// Populate capabilities from discovery results
	capabilities.KubernetesVersion = discoveryResult.KubernetesVersion
	capabilities.SupportedAPIVersions = discoveryResult.SupportedAPIVersions
	capabilities.AvailableResources = discoveryResult.AvailableResources
	capabilities.Features = discoveryResult.DetectedFeatures

	logger.V(4).Info("API capabilities detected",
		"version", capabilities.KubernetesVersion,
		"apiVersionsCount", len(capabilities.SupportedAPIVersions),
		"resourcesCount", len(capabilities.AvailableResources),
		"featuresCount", len(capabilities.Features),
	)

	return nil
}

// detectResourceCapacity detects cluster resource capacity and node information
func (d *ClusterCapabilityDetector) detectResourceCapacity(ctx context.Context, client kubernetes.Interface, capabilities *tmcv1alpha1.ClusterCapabilities) error {
	logger := klog.FromContext(ctx)

	// Get node information to determine capacity
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	// Count nodes
	nodeCount := int32(len(nodes.Items))
	capabilities.NodeCount = &nodeCount

	logger.V(4).Info("resource capacity detected",
		"nodeCount", nodeCount,
	)

	return nil
}

// ShouldRefreshCapabilities determines if capabilities should be refreshed
func ShouldRefreshCapabilities(cluster *tmcv1alpha1.ClusterRegistration) bool {
	// Always detect if capabilities are missing
	if cluster.Status.Capabilities == nil {
		return true
	}

	// Refresh if LastDetected is missing
	if cluster.Status.Capabilities.LastDetected == nil {
		return true
	}

	// Refresh if capabilities are stale
	lastDetected := cluster.Status.Capabilities.LastDetected.Time
	return time.Since(lastDetected) > CapabilityDetectionInterval
}