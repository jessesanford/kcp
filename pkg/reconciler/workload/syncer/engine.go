/*
Copyright 2022 The KCP Authors.

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

package syncer

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/cmd/workload-syncer/options"
	kcpclusterclient "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// Engine orchestrates the core synchronization logic between KCP and physical clusters
type Engine struct {
	// Configuration
	options *options.SyncerOptions

	// Client connections
	kcpClusterClient kcpclusterclient.ClusterInterface
	clusterClient dynamic.Interface
	kcpDynamic    dynamic.Interface

	// Informer factories
	kcpInformerFactory     dynamicinformer.DynamicSharedInformerFactory
	clusterInformerFactory dynamicinformer.DynamicSharedInformerFactory

	// Resource controllers
	resourceControllers map[schema.GroupVersionResource]*ResourceController
	controllersMu       sync.RWMutex

	// Status management
	statusReporter *StatusReporter

	// TMC integration
	tmcRecovery *tmc.RecoveryManager

	// State management
	started   bool
	stopCh    chan struct{}
	waitGroup sync.WaitGroup
	mu        sync.RWMutex
}

// NewEngine creates a new syncer engine
func NewEngine(ctx context.Context, kcpConfig, clusterConfig *rest.Config, opts *options.SyncerOptions) (*Engine, error) {
	engine := &Engine{
		options:             opts,
		resourceControllers: make(map[schema.GroupVersionResource]*ResourceController),
		stopCh:              make(chan struct{}),
	}

	// Configure client rate limiting
	kcpConfig.QPS = opts.QPS
	kcpConfig.Burst = opts.Burst
	clusterConfig.QPS = opts.QPS
	clusterConfig.Burst = opts.Burst

	// Create KCP cluster client
	kcpClusterClient, err := kcpclusterclient.NewForConfig(kcpConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP cluster client: %w", err)
	}
	engine.kcpClusterClient = kcpClusterClient

	kcpDynamic, err := dynamic.NewForConfig(kcpConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP dynamic client: %w", err)
	}
	engine.kcpDynamic = kcpDynamic

	// Create cluster dynamic client
	clusterClient, err := dynamic.NewForConfig(clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster dynamic client: %w", err)
	}
	engine.clusterClient = clusterClient

	// Create informer factories
	engine.kcpInformerFactory = dynamicinformer.NewDynamicSharedInformerFactory(kcpDynamic, opts.ResyncPeriod)
	engine.clusterInformerFactory = dynamicinformer.NewDynamicSharedInformerFactory(clusterClient, opts.ResyncPeriod)

	// Initialize TMC integration
	engine.tmcRecovery = tmc.NewRecoveryManager()

	// Create status reporter
	statusReporter, err := NewStatusReporter(ctx, kcpClusterClient, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create status reporter: %w", err)
	}
	engine.statusReporter = statusReporter

	return engine, nil
}

// Start starts the engine and all its components
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return fmt.Errorf("engine is already started")
	}

	klog.Info("Starting syncer engine...")

	// Start informer factories
	e.kcpInformerFactory.Start(e.stopCh)
	e.clusterInformerFactory.Start(e.stopCh)

	// Wait for caches to sync
	klog.Info("Waiting for informer caches to sync...")
	if !cache.WaitForCacheSync(e.stopCh, e.getInformerSyncFuncs()...) {
		return fmt.Errorf("failed to wait for informer caches to sync")
	}

	// Start status reporter
	if err := e.statusReporter.Start(ctx); err != nil {
		return fmt.Errorf("failed to start status reporter: %w", err)
	}

	// Discover and start resource controllers
	if err := e.discoverAndStartResourceControllers(ctx); err != nil {
		return fmt.Errorf("failed to discover and start resource controllers: %w", err)
	}

	// Start TMC recovery manager
	go e.tmcRecovery.Start(ctx)

	e.started = true
	klog.Info("Syncer engine started successfully")
	return nil
}

// Stop stops the engine and all its components
func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started {
		return nil
	}

	klog.Info("Stopping syncer engine...")

	// Signal all components to stop
	close(e.stopCh)

	// Stop resource controllers
	e.controllersMu.Lock()
	for gvr, controller := range e.resourceControllers {
		klog.FromContext(ctx).WithValues("gvr", gvr).Info("Stopping resource controller")
		if err := controller.Stop(ctx); err != nil {
			klog.FromContext(ctx).WithValues("gvr", gvr).Error(err, "Failed to stop resource controller")
		}
	}
	e.controllersMu.Unlock()

	// Stop status reporter
	if e.statusReporter != nil {
		if err := e.statusReporter.Stop(ctx); err != nil {
			klog.FromContext(ctx).Error(err, "Failed to stop status reporter")
		}
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		e.waitGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
		klog.Info("All engine components stopped")
	case <-ctx.Done():
		klog.Warning("Engine shutdown context cancelled before all components stopped")
	}

	e.started = false
	return nil
}

// IsHealthy returns true if the engine is healthy
func (e *Engine) IsHealthy() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.started && e.statusReporter != nil && e.statusReporter.IsHealthy()
}

// GetMetrics returns engine metrics
func (e *Engine) GetMetrics() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	metrics := make(map[string]interface{})
	
	e.controllersMu.RLock()
	metrics["resource_controllers_count"] = len(e.resourceControllers)
	e.controllersMu.RUnlock()
	
	metrics["started"] = e.started
	
	if e.statusReporter != nil {
		statusMetrics := e.statusReporter.GetMetrics()
		for k, v := range statusMetrics {
			metrics["status."+k] = v
		}
	}

	return metrics
}

// discoverAndStartResourceControllers discovers the resources to sync and starts controllers for them
func (e *Engine) discoverAndStartResourceControllers(ctx context.Context) error {
	// TODO: This should discover resources from the SyncTarget spec
	// For now, we'll start with common workload resources
	resourcesToSync := []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "", Version: "v1", Resource: "secrets"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "replicasets"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
	}

	for _, gvr := range resourcesToSync {
		if err := e.startResourceController(ctx, gvr); err != nil {
			// Log error but continue with other resources
			klog.FromContext(ctx).WithValues("gvr", gvr).Error(err, "Failed to start resource controller")
			continue
		}
	}

	return nil
}

// startResourceController starts a resource controller for the given GVR
func (e *Engine) startResourceController(ctx context.Context, gvr schema.GroupVersionResource) error {
	e.controllersMu.Lock()
	defer e.controllersMu.Unlock()

	// Check if controller already exists
	if _, exists := e.resourceControllers[gvr]; exists {
		return nil
	}

	// Create resource controller
	controller, err := NewResourceController(ctx, ResourceControllerOptions{
		GVR:                    gvr,
		KCPInformerFactory:     e.kcpInformerFactory,
		ClusterInformerFactory: e.clusterInformerFactory,
		KCPClient:              e.kcpDynamic,
		ClusterClient:          e.clusterClient,
		SyncerOptions:          e.options,
	})
	if err != nil {
		return fmt.Errorf("failed to create resource controller for %s: %w", gvr, err)
	}

	// Start controller
	e.waitGroup.Add(1)
	go func() {
		defer e.waitGroup.Done()
		defer handlePanic(fmt.Sprintf("resource-controller-%s", gvr))
		
		if err := controller.Start(ctx, e.options.Workers); err != nil {
			klog.FromContext(ctx).WithValues("gvr", gvr).Error(err, "Resource controller failed")
		}
	}()

	e.resourceControllers[gvr] = controller
	klog.FromContext(ctx).WithValues("gvr", gvr).Info("Started resource controller")
	
	return nil
}

// getInformerSyncFuncs returns functions to check if all informers are synced
func (e *Engine) getInformerSyncFuncs() []cache.InformerSynced {
	// For now, return empty slice - will be populated when we add specific informers
	return []cache.InformerSynced{}
}