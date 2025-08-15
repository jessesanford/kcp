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
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	kcpclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
	"github.com/kcp-dev/kcp/pkg/virtual/contracts"
)

// KCPDiscoveryProvider implements ResourceDiscoveryInterface for KCP environments
type KCPDiscoveryProvider struct {
	// kcpClient provides access to KCP APIs
	kcpClient kcpclient.ClusterInterface

	// informerFactory provides shared informers for KCP resources
	informerFactory kcpinformers.SharedInformerFactory

	// apiExportInformer monitors APIExport changes
	apiExportInformer cache.SharedIndexInformer

	// cache stores discovered resources per workspace
	cache interfaces.DiscoveryCache

	// converter converts APIExport data to ResourceInfo
	converter *APIExportConverter

	// watcher handles resource change watching
	watcher *ResourceWatcher

	// workspace is the logical cluster this provider serves
	workspace string

	// mutex protects concurrent access
	mutex sync.RWMutex

	// started indicates if the provider has been started
	started bool

	// stopCh signals shutdown
	stopCh chan struct{}
}

// NewKCPDiscoveryProvider creates a new KCP discovery provider
func NewKCPDiscoveryProvider(
	kcpClient kcpclient.ClusterInterface,
	informerFactory kcpinformers.SharedInformerFactory,
	workspace string,
) (*KCPDiscoveryProvider, error) {
	if kcpClient == nil {
		return nil, fmt.Errorf("kcpClient cannot be nil")
	}
	if informerFactory == nil {
		return nil, fmt.Errorf("informerFactory cannot be nil")
	}
	if workspace == "" {
		return nil, fmt.Errorf("workspace cannot be empty")
	}

	// Create discovery cache
	discoveryCache := NewMemoryDiscoveryCache(
		time.Duration(contracts.DefaultCacheTTLSeconds)*time.Second,
		time.Duration(contracts.DefaultCacheCleanupIntervalSeconds)*time.Second,
	)

	// Get APIExport informer
	apiExportInformer := informerFactory.Apis().V1alpha1().APIExports().Informer()

	// Create converter
	converter := NewAPIExportConverter(workspace)

	provider := &KCPDiscoveryProvider{
		kcpClient:         kcpClient,
		informerFactory:   informerFactory,
		apiExportInformer: apiExportInformer,
		cache:             discoveryCache,
		converter:         converter,
		workspace:         workspace,
		stopCh:            make(chan struct{}),
	}

	// Create watcher
	provider.watcher = NewResourceWatcher(provider, provider.stopCh)

	return provider, nil
}

// Start initializes the discovery provider and begins monitoring
func (p *KCPDiscoveryProvider) Start(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.started {
		return fmt.Errorf("discovery provider already started")
	}

	klog.V(2).InfoS("Starting KCP discovery provider", "workspace", p.workspace)

	// Start cache cleanup
	if memCache, ok := p.cache.(*MemoryDiscoveryCache); ok {
		memCache.Start()
	}

	// Start the watcher
	if err := p.watcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start resource watcher: %w", err)
	}

	p.started = true
	klog.V(2).InfoS("KCP discovery provider started successfully", "workspace", p.workspace)
	return nil
}

// Discover returns available resources in the specified workspace
func (p *KCPDiscoveryProvider) Discover(ctx context.Context, workspace string) ([]interfaces.ResourceInfo, error) {
	start := time.Now()
	defer func() {
		RecordDiscoveryRequest(workspace, "discover", time.Since(start), nil)
	}()

	// Check cache first
	if cached, found := p.cache.GetResources(workspace); found {
		RecordCacheHit(workspace, true)
		klog.V(4).InfoS("Discovery cache hit", "workspace", workspace, "resources", len(cached))
		return cached, nil
	}

	RecordCacheHit(workspace, false)

	// Get APIExports for the workspace
	apiExports := p.apiExportInformer.GetStore().List()
	var workspaceResources []interfaces.ResourceInfo

	for _, obj := range apiExports {
		apiExport, ok := obj.(*apisv1alpha1.APIExport)
		if !ok {
			continue
		}

		// Convert APIExport to ResourceInfo
		resources, err := p.converter.ConvertAPIExport(apiExport)
		if err != nil {
			klog.ErrorS(err, "Failed to convert APIExport", "name", apiExport.Name, "workspace", workspace)
			continue
		}

		workspaceResources = append(workspaceResources, resources...)
	}

	// Cache the results
	p.cache.SetResources(workspace, workspaceResources, contracts.DefaultCacheTTLSeconds)

	klog.V(3).InfoS("Discovery completed", "workspace", workspace, "resources", len(workspaceResources))
	return workspaceResources, nil
}

// GetOpenAPISchema returns the OpenAPI schema for workspace resources
func (p *KCPDiscoveryProvider) GetOpenAPISchema(ctx context.Context, workspace string) ([]byte, error) {
	resources, err := p.Discover(ctx, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to discover resources: %w", err)
	}

	// Aggregate OpenAPI schemas from all resources
	// This is a simplified implementation - in practice, you'd merge schemas properly
	var aggregatedSchema []byte
	for _, resource := range resources {
		if len(resource.OpenAPISchema) > 0 {
			aggregatedSchema = append(aggregatedSchema, resource.OpenAPISchema...)
		}
	}

	return aggregatedSchema, nil
}

// Watch monitors for resource changes in the workspace
func (p *KCPDiscoveryProvider) Watch(ctx context.Context, workspace string) (<-chan interfaces.DiscoveryEvent, error) {
	if !p.started {
		return nil, fmt.Errorf("discovery provider not started")
	}

	return p.watcher.Subscribe(workspace), nil
}

// IsResourceAvailable checks if a specific resource is available
func (p *KCPDiscoveryProvider) IsResourceAvailable(ctx context.Context, workspace string, gvr schema.GroupVersionResource) (bool, error) {
	resources, err := p.Discover(ctx, workspace)
	if err != nil {
		return false, fmt.Errorf("failed to discover resources: %w", err)
	}

	for _, resource := range resources {
		if resource.GroupVersionResource == gvr {
			return true, nil
		}
	}

	return false, nil
}

// Stop shuts down the discovery provider
func (p *KCPDiscoveryProvider) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.started {
		return
	}

	klog.V(2).InfoS("Stopping KCP discovery provider", "workspace", p.workspace)

	close(p.stopCh)

	// Stop cache
	if memCache, ok := p.cache.(*MemoryDiscoveryCache); ok {
		memCache.Stop()
	}

	p.started = false
	klog.V(2).InfoS("KCP discovery provider stopped", "workspace", p.workspace)
}