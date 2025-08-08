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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
)

// APIDiscoveryClient abstracts the Kubernetes discovery client for testing
type APIDiscoveryClient interface {
	ServerVersion() (*version.Info, error)
	ServerGroups() (*metav1.APIGroupList, error)
	ServerPreferredResources() ([]*metav1.APIResourceList, error)
}

// DiscoveryResult contains the results of API discovery
type DiscoveryResult struct {
	KubernetesVersion    string
	SupportedAPIVersions []string
	AvailableResources   []string
	DetectedFeatures     []string
}

// PerformAPIDiscovery discovers cluster capabilities using the Kubernetes discovery API
func PerformAPIDiscovery(ctx context.Context, client APIDiscoveryClient) (*DiscoveryResult, error) {
	result := &DiscoveryResult{}

	// Discover Kubernetes version
	version, err := discoverKubernetesVersion(client)
	if err != nil {
		return nil, fmt.Errorf("failed to discover Kubernetes version: %w", err)
	}
	result.KubernetesVersion = version

	// Discover supported API versions
	apiVersions, err := discoverSupportedAPIVersions(client)
	if err != nil {
		return nil, fmt.Errorf("failed to discover API versions: %w", err)
	}
	result.SupportedAPIVersions = apiVersions

	// Discover available resources
	resources, err := discoverAvailableResources(client)
	if err != nil {
		return nil, fmt.Errorf("failed to discover resources: %w", err)
	}
	result.AvailableResources = resources

	// Detect cluster features based on available resources
	result.DetectedFeatures = detectClusterFeatures(resources, apiVersions)

	return result, nil
}

// discoverKubernetesVersion retrieves the Kubernetes version from the cluster
func discoverKubernetesVersion(client APIDiscoveryClient) (string, error) {
	versionInfo, err := client.ServerVersion()
	if err != nil {
		return "", err
	}
	return versionInfo.String(), nil
}

// discoverSupportedAPIVersions retrieves the supported API versions
func discoverSupportedAPIVersions(client APIDiscoveryClient) ([]string, error) {
	groups, err := client.ServerGroups()
	if err != nil {
		return nil, err
	}
	
	var versions []string
	// Add core API version
	versions = append(versions, "v1")
	
	// Add group versions
	for _, group := range groups.Groups {
		for _, version := range group.Versions {
			versions = append(versions, version.GroupVersion)
		}
	}
	
	return versions, nil
}

// discoverAvailableResources retrieves the available resource types
func discoverAvailableResources(client APIDiscoveryClient) ([]string, error) {
	resourceLists, err := client.ServerPreferredResources()
	if err != nil {
		// Partial failures are acceptable for discovery
		if discovery.IsGroupDiscoveryFailedError(err) {
			// When there are partial failures, we still get the successful groups
			if len(resourceLists) == 0 {
				return nil, err
			}
			// Continue with partial results
		} else {
			return nil, err
		}
	}

	var resources []string
	for _, list := range resourceLists {
		for _, resource := range list.APIResources {
			// Include the full resource name with API group
			resourceName := resource.Name
			if list.GroupVersion != "v1" {
				resourceName = fmt.Sprintf("%s.%s", resource.Name, list.GroupVersion)
			}
			resources = append(resources, resourceName)
		}
	}

	return resources, nil
}

// detectClusterFeatures analyzes available resources to detect cluster features
func detectClusterFeatures(resources []string, apiVersions []string) []string {
	var features []string
	resourceSet := make(map[string]bool)
	apiSet := make(map[string]bool)

	// Create lookup sets
	for _, resource := range resources {
		resourceSet[resource] = true
	}
	for _, api := range apiVersions {
		apiSet[api] = true
	}

	// Detect common cluster features
	if hasResource(resourceSet, "deployments.apps/v1", "deployments") {
		features = append(features, "workload-deployment")
	}
	if hasResource(resourceSet, "services", "services.v1") {
		features = append(features, "networking-services")
	}
	if hasResource(resourceSet, "ingresses.networking.k8s.io/v1", "ingresses.extensions/v1beta1") {
		features = append(features, "ingress-support")
	}
	if hasResource(resourceSet, "persistentvolumes", "persistentvolumeclaims") {
		features = append(features, "persistent-storage")
	}
	if hasResource(resourceSet, "networkpolicies.networking.k8s.io/v1") {
		features = append(features, "network-policies")
	}

	return features
}

// hasResource checks if any of the given resource names exist in the resource set
func hasResource(resourceSet map[string]bool, resourceNames ...string) bool {
	for _, name := range resourceNames {
		if resourceSet[name] {
			return true
		}
	}
	return false
}