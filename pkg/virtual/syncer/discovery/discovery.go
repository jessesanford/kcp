/*
Copyright 2023 The KCP Authors.

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

package discovery

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/endpoints/discovery"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/controlplane/apiserver/miniaggregator"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	corelogicalcluster "github.com/kcp-dev/logicalcluster/v3"
)

var (
	errorScheme = runtime.NewScheme()
	errorCodecs = serializer.NewCodecFactory(errorScheme)
)

func init() {
	errorScheme.AddUnversionedTypes(metav1.Unversioned,
		&metav1.Status{},
	)
}

// DiscoveryService provides API discovery for virtual workspaces.
// It aggregates available APIs from multiple clusters and presents them
// as a unified discovery interface following Kubernetes conventions.
//
// The service handles:
// - API group and version discovery
// - Resource type enumeration  
// - Version negotiation across clusters
// - OpenAPI schema aggregation
// - Performance optimized caching
type DiscoveryService struct {
	// logger provides structured logging
	logger logr.Logger

	// kcpClient provides cluster-aware API access
	kcpClient kcpclientset.ClusterInterface

	// dynamicClient enables dynamic resource access across clusters
	dynamicClient dynamic.Interface

	// informerFactory provides shared informers for efficient watching
	informerFactory kcpinformers.SharedInformerFactory

	// cache stores aggregated discovery data
	cache *DiscoveryCache

	// mu protects concurrent access to discovery data
	mu sync.RWMutex

	// aggregatedAPIGroups holds discovered API groups across all clusters
	aggregatedAPIGroups map[string]*metav1.APIGroup

	// clusterAPIs tracks API availability per cluster
	clusterAPIs map[corelogicalcluster.Name]map[schema.GroupVersion][]metav1.APIResource

	// versionPreference defines preferred API versions
	versionPreference VersionNegotiator

	// schemaManager handles OpenAPI schema aggregation
	schemaManager *OpenAPISchemaManager
}

// NewDiscoveryService creates a new discovery service for virtual workspaces.
//
// The service aggregates API discovery information from multiple clusters
// and provides a unified view through standard Kubernetes discovery endpoints.
//
// Parameters:
//   - logger: Structured logger for the service
//   - kcpClient: Cluster-aware KCP client for API access
//   - dynamicClient: Dynamic client for cross-cluster resource operations
//   - informerFactory: Shared informer factory for efficient watching
//
// Returns:
//   - *DiscoveryService: Configured discovery service
//   - error: Configuration or initialization error
func NewDiscoveryService(
	logger logr.Logger,
	kcpClient kcpclientset.ClusterInterface,
	dynamicClient dynamic.Interface,
	informerFactory kcpinformers.SharedInformerFactory,
) (*DiscoveryService, error) {
	if kcpClient == nil {
		return nil, fmt.Errorf("kcpClient is required")
	}
	if dynamicClient == nil {
		return nil, fmt.Errorf("dynamicClient is required")
	}
	if informerFactory == nil {
		return nil, fmt.Errorf("informerFactory is required")
	}

	cache, err := NewDiscoveryCache(logger.WithName("cache"), 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery cache: %w", err)
	}

	schemaManager, err := NewOpenAPISchemaManager(logger.WithName("schemas"))
	if err != nil {
		return nil, fmt.Errorf("failed to create schema manager: %w", err)
	}

	return &DiscoveryService{
		logger:              logger,
		kcpClient:           kcpClient,
		dynamicClient:       dynamicClient,
		informerFactory:     informerFactory,
		cache:               cache,
		aggregatedAPIGroups: make(map[string]*metav1.APIGroup),
		clusterAPIs:         make(map[corelogicalcluster.Name]map[schema.GroupVersion][]metav1.APIResource),
		versionPreference:   NewVersionNegotiator(),
		schemaManager:       schemaManager,
	}, nil
}

// DiscoverAPIs refreshes the aggregated API discovery data from all clusters.
//
// This method queries each cluster for available APIs, aggregates the results,
// and updates the internal cache. It handles version negotiation to determine
// the best available versions across clusters.
//
// Parameters:
//   - ctx: Context for request lifecycle and cancellation
//
// Returns:
//   - error: Discovery or aggregation error
func (d *DiscoveryService) DiscoverAPIs(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	logger := d.logger.WithName("discover-apis")
	logger.Info("Starting API discovery across clusters")

	// Clear existing aggregated data
	d.aggregatedAPIGroups = make(map[string]*metav1.APIGroup)
	newClusterAPIs := make(map[corelogicalcluster.Name]map[schema.GroupVersion][]metav1.APIResource)

	// Get list of available clusters
	clusters, err := d.getAvailableClusters(ctx)
	if err != nil {
		logger.Error(err, "Failed to get available clusters")
		return fmt.Errorf("failed to get clusters: %w", err)
	}

	// Discover APIs from each cluster
	for _, cluster := range clusters {
		clusterAPIs, err := d.discoverClusterAPIs(ctx, cluster)
		if err != nil {
			logger.Error(err, "Failed to discover APIs for cluster", "cluster", cluster)
			// Continue with other clusters
			continue
		}
		newClusterAPIs[cluster] = clusterAPIs
	}

	// Update cluster APIs
	d.clusterAPIs = newClusterAPIs

	// Aggregate APIs across clusters
	if err := d.aggregateAPIs(); err != nil {
		logger.Error(err, "Failed to aggregate APIs")
		return fmt.Errorf("failed to aggregate APIs: %w", err)
	}

	// Update cache
	d.cache.UpdateAPIGroups(d.aggregatedAPIGroups)

	logger.Info("API discovery completed", "groups", len(d.aggregatedAPIGroups), "clusters", len(clusters))
	return nil
}

// getAvailableClusters returns the list of clusters available for discovery.
func (d *DiscoveryService) getAvailableClusters(ctx context.Context) ([]corelogicalcluster.Name, error) {
	// In a real implementation, this would query the cluster registry
	// For now, return a placeholder
	return []corelogicalcluster.Name{
		corelogicalcluster.Name("cluster-1"),
		corelogicalcluster.Name("cluster-2"),
	}, nil
}

// discoverClusterAPIs discovers available APIs for a specific cluster.
func (d *DiscoveryService) discoverClusterAPIs(ctx context.Context, cluster corelogicalcluster.Name) (map[schema.GroupVersion][]metav1.APIResource, error) {
	logger := d.logger.WithName("cluster-discovery").WithValues("cluster", cluster)
	logger.V(2).Info("Discovering APIs for cluster")

	// Get cluster-specific discovery client
	clusterClient := d.kcpClient.Cluster(cluster.Path())
	discoveryClient := clusterClient.Discovery()

	// Discover API groups
	apiGroups, err := discoveryClient.ServerGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to discover API groups: %w", err)
	}

	clusterAPIs := make(map[schema.GroupVersion][]metav1.APIResource)

	// Discover resources for each group/version
	for _, group := range apiGroups.Groups {
		for _, version := range group.Versions {
			gv := schema.GroupVersion{
				Group:   group.Name,
				Version: version.Version,
			}

			resources, err := discoveryClient.ServerResourcesForGroupVersion(version.GroupVersion)
			if err != nil {
				logger.V(1).Error(err, "Failed to discover resources for group/version", "groupVersion", version.GroupVersion)
				continue
			}

			clusterAPIs[gv] = resources.APIResources
		}
	}

	logger.V(2).Info("Completed API discovery for cluster", "groupVersions", len(clusterAPIs))
	return clusterAPIs, nil
}

// aggregateAPIs combines APIs from all clusters into aggregated groups.
func (d *DiscoveryService) aggregateAPIs() error {
	groupVersions := make(map[string]sets.Set[string])
	groupResources := make(map[schema.GroupVersion][]metav1.APIResource)

	// Collect all group/versions and resources across clusters
	for _, clusterAPIs := range d.clusterAPIs {
		for gv, resources := range clusterAPIs {
			groupName := gv.Group
			if _, exists := groupVersions[groupName]; !exists {
				groupVersions[groupName] = sets.New[string]()
			}
			groupVersions[groupName].Insert(gv.Version)

			// Merge resources (simple approach - could be more sophisticated)
			existing := groupResources[gv]
			merged := d.mergeAPIResources(existing, resources)
			groupResources[gv] = merged
		}
	}

	// Build aggregated API groups
	for groupName, versions := range groupVersions {
		versionList := versions.UnsortedList()
		d.versionPreference.SortVersions(versionList)

		groupVersionsForDiscovery := make([]metav1.GroupVersionForDiscovery, 0, len(versionList))
		for _, version := range versionList {
			groupVersionsForDiscovery = append(groupVersionsForDiscovery, metav1.GroupVersionForDiscovery{
				GroupVersion: schema.GroupVersion{Group: groupName, Version: version}.String(),
				Version:      version,
			})
		}

		apiGroup := &metav1.APIGroup{
			Name:             groupName,
			Versions:         groupVersionsForDiscovery,
			PreferredVersion: groupVersionsForDiscovery[0], // First is preferred after sorting
		}

		d.aggregatedAPIGroups[groupName] = apiGroup
	}

	return nil
}

// mergeAPIResources combines API resources from multiple sources.
func (d *DiscoveryService) mergeAPIResources(existing, new []metav1.APIResource) []metav1.APIResource {
	resourceMap := make(map[string]metav1.APIResource)

	// Add existing resources
	for _, resource := range existing {
		resourceMap[resource.Name] = resource
	}

	// Merge new resources (simple override for now)
	for _, resource := range new {
		resourceMap[resource.Name] = resource
	}

	// Convert back to slice
	merged := make([]metav1.APIResource, 0, len(resourceMap))
	for _, resource := range resourceMap {
		merged = append(merged, resource)
	}

	// Sort for consistent ordering
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Name < merged[j].Name
	})

	return merged
}

// GetAPIGroups returns the aggregated list of API groups.
func (d *DiscoveryService) GetAPIGroups() []*metav1.APIGroup {
	d.mu.RLock()
	defer d.mu.RUnlock()

	groups := make([]*metav1.APIGroup, 0, len(d.aggregatedAPIGroups))
	for _, group := range d.aggregatedAPIGroups {
		groups = append(groups, group)
	}

	// Sort groups by name for consistent ordering
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups
}

// GetAPIGroup returns a specific API group by name.
func (d *DiscoveryService) GetAPIGroup(groupName string) (*metav1.APIGroup, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	group, exists := d.aggregatedAPIGroups[groupName]
	return group, exists
}

// CreateDiscoveryHandlers creates HTTP handlers for discovery endpoints.
//
// This method creates handlers that implement the standard Kubernetes
// discovery API endpoints (/api, /apis, /apis/<group>, /apis/<group>/<version>).
//
// Parameters:
//   - delegate: Fallback handler for non-virtual requests
//
// Returns:
//   - map[string]http.Handler: Map of URL patterns to handlers
func (d *DiscoveryService) CreateDiscoveryHandlers(delegate http.Handler) map[string]http.Handler {
	return map[string]http.Handler{
		"/apis":                    d.newRootDiscoveryHandler(delegate),
		"/apis/{group}":            d.newGroupDiscoveryHandler(delegate),
		"/apis/{group}/{version}":  d.newVersionDiscoveryHandler(delegate),
	}
}

// Start begins the discovery service background operations.
func (d *DiscoveryService) Start(ctx context.Context) error {
	logger := d.logger.WithName("start")
	logger.Info("Starting discovery service")

	// Start schema manager
	if err := d.schemaManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start schema manager: %w", err)
	}

	// Start cache
	d.cache.Start(ctx)

	// Initial discovery
	if err := d.DiscoverAPIs(ctx); err != nil {
		logger.Error(err, "Initial API discovery failed")
		return fmt.Errorf("initial discovery failed: %w", err)
	}

	// Start periodic refresh
	go d.periodicRefresh(ctx)

	logger.Info("Discovery service started successfully")
	return nil
}

// periodicRefresh runs periodic API discovery updates.
func (d *DiscoveryService) periodicRefresh(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.DiscoverAPIs(ctx); err != nil {
				d.logger.Error(err, "Periodic API discovery failed")
			}
		}
	}
}