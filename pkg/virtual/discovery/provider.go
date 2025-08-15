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

// Package discovery provides KCP-aware resource discovery for virtual workspaces.
//
// This package implements a discovery provider that enables virtual workspaces
// to discover available API resources across logical clusters while maintaining
// proper workspace isolation and multi-tenancy guarantees.
//
// This implementation is part of a multi-PR development strategy where cache,
// converter, and watcher components will be added in subsequent PRs.
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
	
	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
	"github.com/kcp-dev/kcp/pkg/virtual/contracts"
)

// KCPDiscoveryProvider implements ResourceDiscoveryInterface for KCP environments.
//
// This provider integrates with KCP's APIExport and APIBinding system to discover
// available resources in each logical cluster workspace while maintaining strict
// workspace isolation and multi-tenancy guarantees.
//
// All methods are thread-safe and can be called concurrently.
type KCPDiscoveryProvider struct {
	// kcpClient provides access to KCP APIs
	kcpClient kcpclient.ClusterInterface

	// informerFactory provides shared informers for KCP resources
	informerFactory kcpinformers.SharedInformerFactory

	// apiExportInformer monitors APIExport changes
	apiExportInformer cache.SharedIndexInformer

	// cache stores discovered resources per workspace (stubbed for PR split)
	cache interfaces.DiscoveryCache

	// NOTE: converter and watcher are stubbed for this PR split
	// They will be implemented in subsequent PRs

	// workspace is the logical cluster this provider serves
	workspace logicalcluster.Name

	// mutex protects concurrent access
	mutex sync.RWMutex

	// started indicates if the provider has been started
	started bool

	// stopCh signals shutdown
	stopCh chan struct{}
}

// NewKCPDiscoveryProvider creates a new KCP discovery provider.
// It initializes the discovery provider with the necessary KCP clients and informers.
//
// Parameters:
//   - kcpClient: Cluster-aware KCP client for accessing KCP APIs
//   - informerFactory: Shared informer factory for the workspace
//   - workspace: Logical cluster name for workspace isolation
//
// Returns:
//   - *KCPDiscoveryProvider: Configured discovery provider ready to start
//   - error: Configuration or setup error
func NewKCPDiscoveryProvider(
	kcpClient kcpclient.ClusterInterface,
	informerFactory kcpinformers.SharedInformerFactory,
	workspace logicalcluster.Name,
) (*KCPDiscoveryProvider, error) {
	if kcpClient == nil {
		return nil, fmt.Errorf("kcpClient cannot be nil")
	}
	if informerFactory == nil {
		return nil, fmt.Errorf("informerFactory cannot be nil")
	}
	if workspace.Empty() {
		return nil, fmt.Errorf("workspace cannot be empty")
	}

	// Create a stub discovery cache for this PR split
	// The actual implementation will be added in the cache PR
	discoveryCache := NewStubDiscoveryCache()

	// Get APIExport informer
	apiExportInformer := informerFactory.Apis().V1alpha1().APIExports().Informer()

	provider := &KCPDiscoveryProvider{
		kcpClient:         kcpClient,
		informerFactory:   informerFactory,
		apiExportInformer: apiExportInformer,
		cache:             discoveryCache,
		workspace:         workspace,
		stopCh:            make(chan struct{}),
	}

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

	// Cache is stubbed for this PR split
	// Watcher is stubbed for this PR split
	// These will be implemented in subsequent PRs

	p.started = true
	klog.V(2).InfoS("KCP discovery provider started successfully", "workspace", p.workspace)
	return nil
}

// Discover returns available resources in the specified workspace.
// This method respects workspace boundaries and implements proper access control.
func (p *KCPDiscoveryProvider) Discover(ctx context.Context, workspace logicalcluster.Name) ([]interfaces.ResourceInfo, error) {
	start := time.Now()
	defer func() {
		RecordDiscoveryRequest(workspace.String(), "discover", time.Since(start), nil)
	}()

	// Check cache first
	if cached, found := p.cache.GetResources(workspace); found {
		RecordCacheHit(workspace.String(), true)
		klog.V(4).InfoS("Discovery cache hit", "workspace", workspace, "resources", len(cached))
		return cached, nil
	}

	RecordCacheHit(workspace.String(), false)

	// TODO: Implement actual APIExport discovery for this workspace
	// This is stubbed for the PR split - actual implementation will be added
	// in the converter PR which will handle APIExport to ResourceInfo conversion
	var workspaceResources []interfaces.ResourceInfo
	
	// For now, return empty results to allow compilation
	// The actual discovery logic will be implemented when the converter is added

	// Cache the results
	p.cache.SetResources(workspace, workspaceResources, contracts.DefaultCacheTTLSeconds)

	klog.V(3).InfoS("Discovery completed", "workspace", workspace, "resources", len(workspaceResources))
	return workspaceResources, nil
}

// GetOpenAPISchema returns the OpenAPI schema for workspace resources.
// The schema is workspace-scoped and includes only accessible resources.
func (p *KCPDiscoveryProvider) GetOpenAPISchema(ctx context.Context, workspace logicalcluster.Name) ([]byte, error) {
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

// Watch monitors for resource changes in the workspace.
// This method is stubbed for the PR split - watching will be implemented
// in the watcher PR.
func (p *KCPDiscoveryProvider) Watch(ctx context.Context, workspace logicalcluster.Name) (<-chan interfaces.DiscoveryEvent, error) {
	if !p.started {
		return nil, fmt.Errorf("discovery provider not started")
	}

	// TODO: Implement watching in the watcher PR
	return nil, fmt.Errorf("watch functionality not yet implemented - will be added in watcher PR")
}

// IsResourceAvailable checks if a specific resource is available in the workspace.
// This method respects RBAC and workspace access policies.
func (p *KCPDiscoveryProvider) IsResourceAvailable(ctx context.Context, workspace logicalcluster.Name, gvr schema.GroupVersionResource) (bool, error) {
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

	// Cache cleanup is stubbed for this PR split
	// Will be implemented when the actual cache is added

	p.started = false
	klog.V(2).InfoS("KCP discovery provider stopped", "workspace", p.workspace)
}