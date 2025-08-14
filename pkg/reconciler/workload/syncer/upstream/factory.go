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

package upstream

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
	tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
)

// Factory creates upstream sync components
type Factory interface {
	// NewSyncer creates a new syncer instance
	NewSyncer(config *Config) (Syncer, error)

	// NewWatcher creates a resource watcher for a cluster
	NewWatcher(client dynamic.Interface, clusterName string) ResourceWatcher

	// NewProcessor creates an event processor
	NewProcessor(applier UpdateApplier) EventProcessor

	// NewAggregator creates a status aggregator
	NewAggregator(strategy tmcv1alpha1.ConflictStrategy) StatusAggregator

	// NewCacheManager creates a cache manager
	NewCacheManager(maxSize int) CacheManager

	// NewUpdateApplier creates an update applier
	NewUpdateApplier(client dynamic.Interface) UpdateApplier

	// NewPhysicalClient creates a physical cluster client
	NewPhysicalClient(config *rest.Config, clusterID string) PhysicalClusterClient

	// NewConflictResolver creates a conflict resolver
	NewConflictResolver(strategy tmcv1alpha1.ConflictStrategy) ConflictResolver
}

// Config holds configuration for creating sync components
type Config struct {
	// ClusterName identifies the KCP workspace
	ClusterName string

	// Namespace to operate in
	Namespace string

	// SyncInterval for periodic sync
	SyncInterval time.Duration

	// MaxRetries for failed operations
	MaxRetries int

	// CacheSize for local cache
	CacheSize int

	// EnableMetrics enables metrics collection
	EnableMetrics bool
}

// defaultFactory is the default implementation of Factory
type defaultFactory struct{}

// NewFactory creates a new factory instance
func NewFactory() Factory {
	if !utilfeature.DefaultMutableFeatureGate.Enabled(kcpfeatures.UpstreamSync) {
		klog.V(2).Info("UpstreamSync feature gate is disabled")
		return &noopFactory{}
	}
	return &defaultFactory{}
}

// Implementation stubs for defaultFactory - actual implementation in PR2/PR3
func (f *defaultFactory) NewSyncer(config *Config) (Syncer, error) {
	runtime.HandleError(fmt.Errorf("syncer implementation pending in wave2c-02"))
	return nil, fmt.Errorf("not implemented: awaiting wave2c-02-core-sync")
}

func (f *defaultFactory) NewWatcher(client dynamic.Interface, clusterName string) ResourceWatcher {
	runtime.HandleError(fmt.Errorf("watcher implementation pending in wave2c-02"))
	return nil
}

func (f *defaultFactory) NewProcessor(applier UpdateApplier) EventProcessor {
	runtime.HandleError(fmt.Errorf("processor implementation pending in wave2c-02"))
	return nil
}

func (f *defaultFactory) NewAggregator(strategy tmcv1alpha1.ConflictStrategy) StatusAggregator {
	runtime.HandleError(fmt.Errorf("aggregator implementation pending in wave2c-03"))
	return nil
}

func (f *defaultFactory) NewCacheManager(maxSize int) CacheManager {
	runtime.HandleError(fmt.Errorf("cache manager implementation pending in wave2c-02"))
	return nil
}

func (f *defaultFactory) NewUpdateApplier(client dynamic.Interface) UpdateApplier {
	runtime.HandleError(fmt.Errorf("update applier implementation pending in wave2c-03"))
	return nil
}

func (f *defaultFactory) NewPhysicalClient(config *rest.Config, clusterID string) PhysicalClusterClient {
	runtime.HandleError(fmt.Errorf("physical client implementation pending in wave2c-02"))
	return nil
}

func (f *defaultFactory) NewConflictResolver(strategy tmcv1alpha1.ConflictStrategy) ConflictResolver {
	runtime.HandleError(fmt.Errorf("conflict resolver implementation pending in wave2c-03"))
	return nil
}

// noopFactory returns when feature gate is disabled
type noopFactory struct{}

func (f *noopFactory) NewSyncer(config *Config) (Syncer, error) {
	return &noopSyncer{}, nil
}

func (f *noopFactory) NewWatcher(client dynamic.Interface, clusterName string) ResourceWatcher {
	return nil
}

func (f *noopFactory) NewProcessor(applier UpdateApplier) EventProcessor {
	return nil
}

func (f *noopFactory) NewAggregator(strategy tmcv1alpha1.ConflictStrategy) StatusAggregator {
	return nil
}

func (f *noopFactory) NewCacheManager(maxSize int) CacheManager {
	return nil
}

func (f *noopFactory) NewUpdateApplier(client dynamic.Interface) UpdateApplier {
	return nil
}

func (f *noopFactory) NewPhysicalClient(config *rest.Config, clusterID string) PhysicalClusterClient {
	return nil
}

func (f *noopFactory) NewConflictResolver(strategy tmcv1alpha1.ConflictStrategy) ConflictResolver {
	return nil
}

// noopSyncer is a no-op implementation when feature is disabled
type noopSyncer struct{}

func (s *noopSyncer) Start(ctx context.Context) error                                                      { return nil }
func (s *noopSyncer) Stop()                                                                                {}
func (s *noopSyncer) ReconcileSyncTarget(ctx context.Context, target interface{}) error { return nil }
func (s *noopSyncer) GetMetrics() Metrics                                                                  { return Metrics{} }
func (s *noopSyncer) IsReady() bool                                                                        { return false }