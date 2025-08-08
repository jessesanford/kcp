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

package physical

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	tmcv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/tmc/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// PhysicalSyncerFactory implements SyncerFactory for physical clusters
type PhysicalSyncerFactory struct {
	// Cluster configurations
	clusterConfigs map[string]*rest.Config
	clusterKubeconfigs map[string]string
	
	// Active syncers
	syncers map[string]WorkloadSyncer
	mu      sync.RWMutex
	
	// Factory configuration
	options *FactoryOptions
}

// FactoryOptions contains configuration for the syncer factory
type FactoryOptions struct {
	// Default syncer options to use for new syncers
	DefaultSyncerOptions *SyncerOptions
	
	// Event handler for factory events
	EventHandler SyncEventHandler
	
	// Health check interval for managed syncers
	HealthCheckInterval time.Duration
}

// NewPhysicalSyncerFactory creates a new factory for physical cluster syncers
func NewPhysicalSyncerFactory(
	clusterKubeconfigs map[string]string,
	options *FactoryOptions,
) (*PhysicalSyncerFactory, error) {
	
	if clusterKubeconfigs == nil {
		return nil, fmt.Errorf("cluster kubeconfigs cannot be nil")
	}
	
	if options == nil {
		options = DefaultFactoryOptions()
	}
	
	// Build cluster configurations from kubeconfigs
	clusterConfigs := make(map[string]*rest.Config)
	for clusterName, kubeconfigPath := range clusterKubeconfigs {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, tmc.NewTMCError(tmc.TMCErrorTypeClusterConfig, "syncer-factory", "init").
				WithMessage(fmt.Sprintf("Failed to build config for cluster %s", clusterName)).
				WithCause(err).
				WithCluster(clusterName, "").
				Build()
		}
		
		// Configure reasonable defaults for cluster connections
		config.QPS = 50
		config.Burst = 100
		
		clusterConfigs[clusterName] = config
	}
	
	factory := &PhysicalSyncerFactory{
		clusterConfigs:     clusterConfigs,
		clusterKubeconfigs: clusterKubeconfigs,
		syncers:           make(map[string]WorkloadSyncer),
		options:           options,
	}
	
	return factory, nil
}

// CreateSyncer implements SyncerFactory.CreateSyncer
func (f *PhysicalSyncerFactory) CreateSyncer(ctx context.Context,
	cluster *tmcv1alpha1.ClusterRegistration,
) (WorkloadSyncer, error) {
	
	f.mu.Lock()
	defer f.mu.Unlock()
	
	logger := klog.FromContext(ctx).WithValues("cluster", cluster.Name, "factory", "physical")
	
	// Check if syncer already exists
	if syncer, exists := f.syncers[cluster.Name]; exists {
		logger.V(2).Info("Syncer already exists for cluster")
		return syncer, nil
	}
	
	// Get cluster configuration
	clusterConfig, exists := f.clusterConfigs[cluster.Name]
	if !exists {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeClusterConfig, "syncer-factory", "create").
			WithMessage(fmt.Sprintf("No configuration found for cluster %s", cluster.Name)).
			WithCluster(cluster.Name, "").
			WithRecoveryHint("Ensure cluster kubeconfig is provided in factory configuration").
			Build()
	}
	
	// Create syncer options
	syncerOptions := f.options.DefaultSyncerOptions
	if syncerOptions == nil {
		syncerOptions = DefaultSyncerOptions()
	}
	
	// Use factory event handler if syncer doesn't have one
	if syncerOptions.EventHandler == nil {
		syncerOptions.EventHandler = f.options.EventHandler
	}
	
	// Create the physical syncer
	syncer, err := NewPhysicalSyncer(cluster, clusterConfig, syncerOptions)
	if err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "syncer-factory", "create").
			WithMessage(fmt.Sprintf("Failed to create syncer for cluster %s", cluster.Name)).
			WithCause(err).
			WithCluster(cluster.Name, string(syncerOptions.LogicalCluster)).
			Build()
	}
	
	// Perform initial health check
	if err := syncer.HealthCheck(ctx, cluster); err != nil {
		logger.Error(err, "Initial health check failed for new syncer")
		// Don't fail creation due to health check, just log the issue
	}
	
	// Store syncer
	f.syncers[cluster.Name] = syncer
	
	logger.Info("Successfully created syncer for cluster")
	
	return syncer, nil
}

// GetSyncer implements SyncerFactory.GetSyncer
func (f *PhysicalSyncerFactory) GetSyncer(clusterName string) (WorkloadSyncer, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	syncer, exists := f.syncers[clusterName]
	return syncer, exists
}

// RemoveSyncer implements SyncerFactory.RemoveSyncer
func (f *PhysicalSyncerFactory) RemoveSyncer(clusterName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	syncer, exists := f.syncers[clusterName]
	if !exists {
		return nil // Already removed
	}
	
	// Clean up resources if the syncer supports it
	if cleanupSyncer, ok := syncer.(interface{ Cleanup() error }); ok {
		if err := cleanupSyncer.Cleanup(); err != nil {
			return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "syncer-factory", "remove").
				WithMessage(fmt.Sprintf("Failed to cleanup syncer for cluster %s", clusterName)).
				WithCause(err).
				WithCluster(clusterName, "").
				Build()
		}
	}
	
	delete(f.syncers, clusterName)
	
	klog.V(2).InfoS("Removed syncer for cluster", "cluster", clusterName)
	
	return nil
}

// ListSyncers returns all active syncers
func (f *PhysicalSyncerFactory) ListSyncers() map[string]WorkloadSyncer {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	// Return copy to avoid concurrent modification
	result := make(map[string]WorkloadSyncer)
	for name, syncer := range f.syncers {
		result[name] = syncer
	}
	
	return result
}

// HealthCheckAll performs health checks on all managed syncers
func (f *PhysicalSyncerFactory) HealthCheckAll(ctx context.Context) map[string]error {
	syncers := f.ListSyncers()
	results := make(map[string]error)
	
	for clusterName, syncer := range syncers {
		// Create a minimal cluster registration for health check
		cluster := &tmcv1alpha1.ClusterRegistration{}
		cluster.Name = clusterName
		
		err := syncer.HealthCheck(ctx, cluster)
		results[clusterName] = err
		
		if err != nil {
			klog.FromContext(ctx).Error(err, "Health check failed", "cluster", clusterName)
		}
	}
	
	return results
}

// RefreshClusterConfigs updates cluster configurations from kubeconfigs
func (f *PhysicalSyncerFactory) RefreshClusterConfigs() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	
	for clusterName, kubeconfigPath := range f.clusterKubeconfigs {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return tmc.NewTMCError(tmc.TMCErrorTypeClusterConfig, "syncer-factory", "refresh").
				WithMessage(fmt.Sprintf("Failed to refresh config for cluster %s", clusterName)).
				WithCause(err).
				WithCluster(clusterName, "").
				Build()
		}
		
		// Configure defaults
		config.QPS = 50
		config.Burst = 100
		
		f.clusterConfigs[clusterName] = config
	}
	
	return nil
}

// GetClusterNames returns the names of all configured clusters
func (f *PhysicalSyncerFactory) GetClusterNames() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	names := make([]string, 0, len(f.clusterConfigs))
	for name := range f.clusterConfigs {
		names = append(names, name)
	}
	
	return names
}

// IsClusterConfigured returns true if the factory has configuration for the cluster
func (f *PhysicalSyncerFactory) IsClusterConfigured(clusterName string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	_, exists := f.clusterConfigs[clusterName]
	return exists
}

// GetFactoryMetrics returns metrics about the factory state
func (f *PhysicalSyncerFactory) GetFactoryMetrics() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	metrics := make(map[string]interface{})
	metrics["configured_clusters"] = len(f.clusterConfigs)
	metrics["active_syncers"] = len(f.syncers)
	
	// Calculate syncer coverage
	if len(f.clusterConfigs) > 0 {
		coverage := float64(len(f.syncers)) / float64(len(f.clusterConfigs)) * 100
		metrics["syncer_coverage_percent"] = coverage
	}
	
	return metrics
}

// DefaultFactoryOptions returns default options for the syncer factory
func DefaultFactoryOptions() *FactoryOptions {
	return &FactoryOptions{
		DefaultSyncerOptions:  DefaultSyncerOptions(),
		HealthCheckInterval:   time.Minute,
	}
}