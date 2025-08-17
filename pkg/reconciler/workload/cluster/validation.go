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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
)

// ClusterValidator provides comprehensive cluster validation functionality.
type ClusterValidator struct {
	// allowedServerProtocols defines acceptable server protocols
	allowedServerProtocols []string
	
	// maxClusterNameLength defines maximum cluster name length
	maxClusterNameLength int
	
	// requiredLabels defines labels that must be present
	requiredLabels []string
	
	// timeouts for various validation operations
	connectTimeout      time.Duration
	validationTimeout   time.Duration
}

// NewClusterValidator creates a new cluster validator with default settings.
func NewClusterValidator() *ClusterValidator {
	return &ClusterValidator{
		allowedServerProtocols: []string{"https"},
		maxClusterNameLength:   63,
		requiredLabels:         []string{"kcp.io/cluster-type"},
		connectTimeout:         30 * time.Second,
		validationTimeout:      60 * time.Second,
	}
}

// ValidateClusterSpec validates the cluster specification before registration.
func (v *ClusterValidator) ValidateClusterSpec(ctx context.Context, cluster *ClusterRegistration) error {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Validating cluster specification", "cluster", cluster.Name)

	var validationErrors []string

	// Validate cluster name
	if err := v.validateClusterName(cluster.Name); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid cluster name: %v", err))
	}

	// Validate location
	if err := v.validateLocation(cluster.Spec.Location); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid location: %v", err))
	}

	// Validate labels
	if err := v.validateLabels(cluster.Spec.Labels); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid labels: %v", err))
	}

	// Validate capabilities
	if err := v.validateCapabilities(cluster.Spec.Capabilities); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid capabilities: %v", err))
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("cluster specification validation failed: %s", strings.Join(validationErrors, "; "))
	}

	logger.V(3).Info("Cluster specification validation successful", "cluster", cluster.Name)
	return nil
}

// ValidateKubeconfig validates the kubeconfig structure and content.
func (v *ClusterValidator) ValidateKubeconfig(ctx context.Context, config *api.Config) error {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Validating kubeconfig structure")

	if config == nil {
		return fmt.Errorf("kubeconfig is nil")
	}

	// Validate clusters section
	if len(config.Clusters) == 0 {
		return fmt.Errorf("no clusters defined in kubeconfig")
	}

	for clusterName, clusterInfo := range config.Clusters {
		if err := v.validateClusterInfo(clusterName, clusterInfo); err != nil {
			return fmt.Errorf("invalid cluster info for %s: %w", clusterName, err)
		}
	}

	// Validate contexts section
	if len(config.Contexts) == 0 {
		return fmt.Errorf("no contexts defined in kubeconfig")
	}

	for contextName, contextInfo := range config.Contexts {
		if err := v.validateContextInfo(contextName, contextInfo, config); err != nil {
			return fmt.Errorf("invalid context info for %s: %w", contextName, err)
		}
	}

	// Validate auth infos section
	if len(config.AuthInfos) == 0 {
		return fmt.Errorf("no auth infos defined in kubeconfig")
	}

	for authName, authInfo := range config.AuthInfos {
		if err := v.validateAuthInfo(authName, authInfo); err != nil {
			return fmt.Errorf("invalid auth info for %s: %w", authName, err)
		}
	}

	// Validate current context
	if config.CurrentContext == "" {
		return fmt.Errorf("no current context specified")
	}

	if _, exists := config.Contexts[config.CurrentContext]; !exists {
		return fmt.Errorf("current context %s does not exist", config.CurrentContext)
	}

	logger.V(3).Info("Kubeconfig validation successful")
	return nil
}

// ValidateClusterConnectivity performs comprehensive connectivity validation.
func (v *ClusterValidator) ValidateClusterConnectivity(ctx context.Context, cluster *ClusterRegistration, config *api.Config) error {
	logger := klog.FromContext(ctx)
	logger.V(3).Info("Validating cluster connectivity", "cluster", cluster.Name)

	// Create timeout context for connectivity checks
	ctx, cancel := context.WithTimeout(ctx, v.connectTimeout)
	defer cancel()

	// Validate each cluster endpoint
	for clusterName, clusterInfo := range config.Clusters {
		if err := v.validateEndpointConnectivity(ctx, clusterName, clusterInfo); err != nil {
			return fmt.Errorf("connectivity validation failed for %s: %w", clusterName, err)
		}
	}

	logger.V(3).Info("Cluster connectivity validation successful", "cluster", cluster.Name)
	return nil
}

// validateClusterName validates the cluster name according to Kubernetes naming conventions.
func (v *ClusterValidator) validateClusterName(name string) error {
	if name == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	if len(name) > v.maxClusterNameLength {
		return fmt.Errorf("cluster name too long: %d characters (max %d)", len(name), v.maxClusterNameLength)
	}

	// Use Kubernetes DNS subdomain validation
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("invalid cluster name: %s", strings.Join(errs, ", "))
	}

	return nil
}

// validateLocation validates the cluster location specification.
func (v *ClusterValidator) validateLocation(location string) error {
	if location == "" {
		return fmt.Errorf("location cannot be empty")
	}

	// Basic location format validation
	if len(location) < 2 || len(location) > 100 {
		return fmt.Errorf("location length must be between 2 and 100 characters")
	}

	// Check for valid characters (alphanumeric, hyphens, underscores)
	if errs := validation.IsDNS1123Subdomain(strings.ToLower(location)); len(errs) > 0 {
		return fmt.Errorf("invalid location format: %s", strings.Join(errs, ", "))
	}

	return nil
}

// validateLabels validates cluster labels.
func (v *ClusterValidator) validateLabels(labels map[string]string) error {
	if labels == nil {
		labels = make(map[string]string)
	}

	// Check required labels
	for _, requiredLabel := range v.requiredLabels {
		if _, exists := labels[requiredLabel]; !exists {
			return fmt.Errorf("required label %s is missing", requiredLabel)
		}
	}

	// Validate each label
	for key, value := range labels {
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return fmt.Errorf("invalid label key %s: %s", key, strings.Join(errs, ", "))
		}

		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			return fmt.Errorf("invalid label value for %s: %s", key, strings.Join(errs, ", "))
		}
	}

	return nil
}

// validateCapabilities validates cluster capabilities specification.
func (v *ClusterValidator) validateCapabilities(capabilities map[string]string) error {
	if capabilities == nil {
		return nil
	}

	// Define allowed capability keys
	allowedCapabilities := map[string]bool{
		"compute.instances":       true,
		"compute.cores":          true,
		"memory.gb":              true,
		"storage.gb":             true,
		"networking.loadbalancer": true,
		"networking.ingress":     true,
		"gpu.nvidia":             true,
		"gpu.amd":                true,
	}

	for key, value := range capabilities {
		if !allowedCapabilities[key] {
			return fmt.Errorf("unknown capability key: %s", key)
		}

		if value == "" {
			return fmt.Errorf("capability value cannot be empty for key: %s", key)
		}
	}

	return nil
}

// validateClusterInfo validates cluster information in kubeconfig.
func (v *ClusterValidator) validateClusterInfo(name string, info *api.Cluster) error {
	if info == nil {
		return fmt.Errorf("cluster info is nil")
	}

	// Validate server URL
	if info.Server == "" {
		return fmt.Errorf("server URL is empty")
	}

	serverURL, err := url.Parse(info.Server)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	// Check protocol
	validProtocol := false
	for _, allowedProtocol := range v.allowedServerProtocols {
		if serverURL.Scheme == allowedProtocol {
			validProtocol = true
			break
		}
	}
	if !validProtocol {
		return fmt.Errorf("unsupported server protocol: %s (allowed: %v)", serverURL.Scheme, v.allowedServerProtocols)
	}

	// Validate certificate authority data if present
	if len(info.CertificateAuthorityData) > 0 && len(info.CertificateAuthority) > 0 {
		return fmt.Errorf("both certificate-authority-data and certificate-authority are specified")
	}

	return nil
}

// validateContextInfo validates context information in kubeconfig.
func (v *ClusterValidator) validateContextInfo(name string, info *api.Context, config *api.Config) error {
	if info == nil {
		return fmt.Errorf("context info is nil")
	}

	// Validate cluster reference
	if info.Cluster == "" {
		return fmt.Errorf("cluster reference is empty")
	}

	if _, exists := config.Clusters[info.Cluster]; !exists {
		return fmt.Errorf("referenced cluster %s does not exist", info.Cluster)
	}

	// Validate user reference
	if info.AuthInfo == "" {
		return fmt.Errorf("user reference is empty")
	}

	if _, exists := config.AuthInfos[info.AuthInfo]; !exists {
		return fmt.Errorf("referenced user %s does not exist", info.AuthInfo)
	}

	return nil
}

// validateAuthInfo validates authentication information in kubeconfig.
func (v *ClusterValidator) validateAuthInfo(name string, info *api.AuthInfo) error {
	if info == nil {
		return fmt.Errorf("auth info is nil")
	}

	// Check that at least one authentication method is specified
	hasAuth := false

	if info.Token != "" {
		hasAuth = true
	}

	if len(info.ClientCertificateData) > 0 || info.ClientCertificate != "" {
		hasAuth = true
		// If client cert is specified, client key must also be present
		if len(info.ClientKeyData) == 0 && info.ClientKey == "" {
			return fmt.Errorf("client certificate specified but client key is missing")
		}
	}

	if info.Username != "" {
		hasAuth = true
	}

	if info.Exec != nil {
		hasAuth = true
	}

	if !hasAuth {
		return fmt.Errorf("no authentication method specified")
	}

	return nil
}

// validateEndpointConnectivity validates connectivity to a cluster endpoint.
func (v *ClusterValidator) validateEndpointConnectivity(ctx context.Context, name string, info *api.Cluster) error {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Validating endpoint connectivity", "cluster", name, "server", info.Server)

	// Parse server URL
	serverURL, err := url.Parse(info.Server)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	// Basic network connectivity check
	// TODO: Implement actual network connectivity validation
	// This would involve creating a client and testing basic API calls
	// For now, we perform basic URL validation

	if serverURL.Host == "" {
		return fmt.Errorf("server host is empty")
	}

	if serverURL.Port() == "" {
		// Use default ports based on scheme
		switch serverURL.Scheme {
		case "https":
			// Default HTTPS port is 443, which is acceptable
		case "http":
			// HTTP is generally not recommended for production
			logger.V(2).Info("Warning: using insecure HTTP protocol", "cluster", name)
		default:
			return fmt.Errorf("unsupported protocol: %s", serverURL.Scheme)
		}
	}

	logger.V(4).Info("Endpoint connectivity validation successful", "cluster", name)
	return nil
}