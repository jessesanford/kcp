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

package syncer

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// Engine is the core syncer component that orchestrates resource synchronization
// between KCP logical clusters and physical Kubernetes clusters
type Engine struct {
	// Configuration
	syncTargetName   string
	syncTargetUID    string
	workspaceCluster logicalcluster.Name
	
	// Clients
	kcpClient       dynamic.Interface
	clusterClient   dynamic.Interface
	kcpDiscovery    discovery.DiscoveryInterface
	clusterDiscovery discovery.DiscoveryInterface
	
	// Controllers
	resourceControllers map[string]*ResourceController
	statusReporter      *StatusReporter
	healthMonitor       *HealthMonitor
	
	// TMC Integration
	tmcMetrics       *tmc.MetricsCollector
	tmcHealthMonitor *tmc.HealthMonitor
	errorHandler     *tmc.TMCError
	
	// State
	started        bool
	mu             sync.RWMutex
	stopCh         chan struct{}
	workersStopCh  chan struct{}
	informerStopCh chan struct{}
	
	// Metrics
	syncCount      int64
	errorCount     int64
	lastSyncTime   time.Time
	startTime      time.Time
}

// EngineOptions configures the syncer engine
type EngineOptions struct {
	SyncTargetName   string
	SyncTargetUID    string
	WorkspaceCluster logicalcluster.Name
	KCPConfig        *rest.Config
	ClusterConfig    *rest.Config
	ResyncPeriod     time.Duration
	Workers          int
}

// NewEngine creates a new syncer engine
func NewEngine(options *EngineOptions) (*Engine, error) {
	logger := klog.Background().WithValues("component", "SyncerEngine", "syncTarget", options.SyncTargetName)
	logger.Info("Creating syncer engine")

	// Create KCP clients
	kcpClient, err := dynamic.NewForConfig(options.KCPConfig)
	if err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeConfiguration, "syncer-engine", "create-kcp-client").
			WithMessage("Failed to create KCP dynamic client").
			WithCause(err).
			Build()
	}

	kcpDiscovery, err := discovery.NewDiscoveryClientForConfig(options.KCPConfig)
	if err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeConfiguration, "syncer-engine", "create-kcp-discovery").
			WithMessage("Failed to create KCP discovery client").
			WithCause(err).
			Build()
	}

	// Create cluster clients
	clusterClient, err := dynamic.NewForConfig(options.ClusterConfig)
	if err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeClusterUnreachable, "syncer-engine", "create-cluster-client").
			WithMessage("Failed to create cluster dynamic client").
			WithCause(err).
			Build()
	}

	clusterDiscovery, err := discovery.NewDiscoveryClientForConfig(options.ClusterConfig)
	if err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeClusterUnreachable, "syncer-engine", "create-cluster-discovery").
			WithMessage("Failed to create cluster discovery client").
			WithCause(err).
			Build()
	}

	// Initialize TMC components
	tmcMetrics := tmc.NewMetricsCollector()
	tmcHealthMonitor := tmc.NewHealthMonitor()

	engine := &Engine{
		syncTargetName:      options.SyncTargetName,
		syncTargetUID:       options.SyncTargetUID,
		workspaceCluster:    options.WorkspaceCluster,
		kcpClient:           kcpClient,
		clusterClient:       clusterClient,
		kcpDiscovery:        kcpDiscovery,
		clusterDiscovery:    clusterDiscovery,
		resourceControllers: make(map[string]*ResourceController),
		tmcMetrics:          tmcMetrics,
		tmcHealthMonitor:    tmcHealthMonitor,
		stopCh:              make(chan struct{}),
		workersStopCh:       make(chan struct{}),
		informerStopCh:      make(chan struct{}),
		startTime:           time.Now(),
	}

	// Initialize status reporter
	statusReporter, err := NewStatusReporter(StatusReporterOptions{
		SyncTargetName:   options.SyncTargetName,
		SyncTargetUID:    options.SyncTargetUID,
		WorkspaceCluster: options.WorkspaceCluster,
		KCPClient:        kcpClient,
		TMCMetrics:       tmcMetrics,
	})
	if err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeConfiguration, "syncer-engine", "create-status-reporter").
			WithMessage("Failed to create status reporter").
			WithCause(err).
			Build()
	}
	engine.statusReporter = statusReporter

	// Initialize health monitor
	healthMonitor, err := NewHealthMonitor(HealthMonitorOptions{
		SyncTargetName: options.SyncTargetName,
		Engine:         engine,
		TMCHealth:      tmcHealthMonitor,
	})
	if err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeConfiguration, "syncer-engine", "create-health-monitor").
			WithMessage("Failed to create health monitor").
			WithCause(err).
			Build()
	}
	engine.healthMonitor = healthMonitor

	logger.Info("Successfully created syncer engine")
	return engine, nil
}

// Start starts the syncer engine and all its components
func (e *Engine) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues("component", "SyncerEngine", "syncTarget", e.syncTargetName)
	logger.Info("Starting syncer engine")

	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return fmt.Errorf("syncer engine already started")
	}
	e.started = true
	e.mu.Unlock()

	// Start TMC health monitoring
	go e.tmcHealthMonitor.Start(ctx)

	// Start status reporter
	if err := e.statusReporter.Start(ctx); err != nil {
		return tmc.NewTMCError(tmc.TMCErrorTypeInternal, "syncer-engine", "start-status-reporter").
			WithMessage("Failed to start status reporter").
			WithCause(err).
			Build()
	}

	// Start health monitor
	if err := e.healthMonitor.Start(ctx); err != nil {
		return tmc.NewTMCError(tmc.TMCErrorTypeInternal, "syncer-engine", "start-health-monitor").
			WithMessage("Failed to start health monitor").
			WithCause(err).
			Build()
	}

	// Discover and start resource controllers
	if err := e.discoverAndStartControllers(ctx); err != nil {
		return tmc.NewTMCError(tmc.TMCErrorTypeSyncFailure, "syncer-engine", "start-controllers").
			WithMessage("Failed to discover and start resource controllers").
			WithCause(err).
			Build()
	}

	// Start main reconciliation loop
	go e.reconcileLoop(ctx)

	logger.Info("Syncer engine started successfully")
	return nil
}

// Stop stops the syncer engine and all its components
func (e *Engine) Stop() {
	logger := klog.Background().WithValues("component", "SyncerEngine", "syncTarget", e.syncTargetName)
	logger.Info("Stopping syncer engine")

	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started {
		return
	}

	// Signal all components to stop
	close(e.stopCh)
	close(e.workersStopCh)
	close(e.informerStopCh)

	// Stop resource controllers
	for name, controller := range e.resourceControllers {
		logger.V(2).Info("Stopping resource controller", "controller", name)
		controller.Stop()
	}

	// Stop health monitor
	if e.healthMonitor != nil {
		e.healthMonitor.Stop()
	}

	// Stop status reporter
	if e.statusReporter != nil {
		e.statusReporter.Stop()
	}

	// Stop TMC health monitoring
	e.tmcHealthMonitor.Stop()

	e.started = false
	logger.Info("Syncer engine stopped")
}

// discoverAndStartControllers discovers available resource types and starts controllers for them
func (e *Engine) discoverAndStartControllers(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues("component", "SyncerEngine", "operation", "discover-controllers")

	// Discover available resource types from KCP
	kcpResources, err := e.kcpDiscovery.ServerResourcesForGroupVersion("v1")
	if err != nil {
		return fmt.Errorf("failed to discover KCP resources: %w", err)
	}

	// For now, just create a simple list - in a real implementation you'd discover all groups
	kcpResourceList := []*metav1.APIResourceList{kcpResources}

	// Discover available resource types from cluster
	clusterResources, err := e.clusterDiscovery.ServerResourcesForGroupVersion("v1")
	if err != nil {
		return fmt.Errorf("failed to discover cluster resources: %w", err)
	}

	clusterResourceList := []*metav1.APIResourceList{clusterResources}

	// Find common resource types that should be synchronized
	commonResources := e.findSyncableResources(kcpResourceList, clusterResourceList)
	logger.Info("Discovered syncable resources", "count", len(commonResources))

	// Start controllers for each resource type
	for _, resource := range commonResources {
		if err := e.startResourceController(ctx, resource); err != nil {
			logger.Error(err, "Failed to start resource controller", "resource", resource.String())
			e.tmcMetrics.RecordComponentError("syncer-engine", e.syncTargetName, 
				tmc.TMCErrorTypeSyncFailure, tmc.TMCErrorSeverityMedium)
			continue
		}
		logger.V(2).Info("Started resource controller", "resource", resource.String())
	}

	if len(e.resourceControllers) == 0 {
		return fmt.Errorf("no resource controllers started")
	}

	logger.Info("Successfully started resource controllers", "count", len(e.resourceControllers))
	return nil
}

// reconcileLoop runs the main reconciliation loop
func (e *Engine) reconcileLoop(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "SyncerEngine", "operation", "reconcile-loop")
	logger.Info("Starting reconcile loop")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Reconcile loop stopping due to context cancellation")
			return
		case <-e.stopCh:
			logger.Info("Reconcile loop stopping due to stop signal")
			return
		case <-ticker.C:
			e.performHealthCheck(ctx)
			e.updateMetrics()
		}
	}
}

// performHealthCheck performs periodic health checks
func (e *Engine) performHealthCheck(ctx context.Context) {
	logger := klog.FromContext(ctx).WithValues("component", "SyncerEngine", "operation", "health-check")

	// Check KCP connectivity
	_, err := e.kcpDiscovery.ServerVersion()
	if err != nil {
		logger.Error(err, "KCP connectivity check failed")
		e.tmcMetrics.RecordClusterConnectivity(e.syncTargetName, e.workspaceCluster.String(), false)
		e.errorCount++
		return
	}

	// Check cluster connectivity
	_, err = e.clusterDiscovery.ServerVersion()
	if err != nil {
		logger.Error(err, "Cluster connectivity check failed")
		e.tmcMetrics.RecordClusterConnectivity(e.syncTargetName, e.workspaceCluster.String(), false)
		e.errorCount++
		return
	}

	// Record successful connectivity
	e.tmcMetrics.RecordClusterConnectivity(e.syncTargetName, e.workspaceCluster.String(), true)
	e.tmcMetrics.RecordClusterHealth(e.syncTargetName, e.workspaceCluster.String(), tmc.HealthStatusHealthy)
}

// updateMetrics updates engine metrics
func (e *Engine) updateMetrics() {
	e.mu.RLock()
	defer e.mu.RUnlock()

	uptime := time.Since(e.startTime)
	e.tmcMetrics.RecordComponentUptime("syncer-engine", e.syncTargetName, e.syncTargetName, uptime)

	// Record sync count
	e.tmcMetrics.RecordSyncResourceCount(e.syncTargetName, "", "", "total", int(e.syncCount))

	// Record error rate
	if e.syncCount > 0 {
		errorRate := float64(e.errorCount) / float64(e.syncCount)
		if errorRate > 0.1 {
			e.tmcMetrics.RecordComponentHealth("syncer-engine", e.syncTargetName, e.syncTargetName, tmc.HealthStatusDegraded)
		} else {
			e.tmcMetrics.RecordComponentHealth("syncer-engine", e.syncTargetName, e.syncTargetName, tmc.HealthStatusHealthy)
		}
	}
}

// findSyncableResources finds resource types that should be synchronized between KCP and cluster
func (e *Engine) findSyncableResources(kcpResources, clusterResources []*metav1.APIResourceList) []schema.GroupVersionResource {
	syncableResources := make([]schema.GroupVersionResource, 0)
	
	// Create a map of cluster resources for quick lookup
	clusterResourceMap := make(map[string]*metav1.APIResource)
	for _, list := range clusterResources {
		for _, resource := range list.APIResources {
			key := fmt.Sprintf("%s/%s", list.GroupVersion, resource.Name)
			clusterResourceMap[key] = &resource
		}
	}
	
	// Find common resources that exist in both KCP and cluster
	for _, list := range kcpResources {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		
		for _, resource := range list.APIResources {
			if resource.Namespaced {
				// Only sync namespaced resources for now
				key := fmt.Sprintf("%s/%s", list.GroupVersion, resource.Name)
				if _, exists := clusterResourceMap[key]; exists {
					// Skip subresources
					if len(resource.Name) > 0 && !contains(resource.Name, "/") {
						gvr := gv.WithResource(resource.Name)
						syncableResources = append(syncableResources, gvr)
					}
				}
			}
		}
	}
	
	return syncableResources
}

// startResourceController starts a resource controller for a specific GVR
func (e *Engine) startResourceController(ctx context.Context, gvr schema.GroupVersionResource) error {
	logger := klog.FromContext(ctx).WithValues("gvr", gvr.String())
	logger.V(2).Info("Starting resource controller")
	
	// Convert GVR to GVK (simplified - in practice you'd need proper discovery)
	gvk := schema.GroupVersionKind{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    gvr.Resource, // Simplified assumption
	}
	
	controller, err := NewResourceController(ResourceControllerOptions{
		GVR:              gvr,
		GVK:              gvk,
		Namespaced:       true, // Simplified - should be discovered
		SyncTargetName:   e.syncTargetName,
		WorkspaceCluster: e.workspaceCluster,
		KCPClient:        e.kcpClient,
		ClusterClient:    e.clusterClient,
		ResyncPeriod:     30 * time.Second,
		Workers:          2,
		TMCMetrics:       e.tmcMetrics,
	})
	if err != nil {
		return err
	}
	
	if err := controller.Start(ctx); err != nil {
		return err
	}
	
	e.mu.Lock()
	e.resourceControllers[gvr.String()] = controller
	e.mu.Unlock()
	
	return nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetStatus returns the current status of the engine
func (e *Engine) GetStatus() *EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return &EngineStatus{
		Started:             e.started,
		SyncTargetName:      e.syncTargetName,
		WorkspaceCluster:    e.workspaceCluster,
		ResourceControllers: len(e.resourceControllers),
		SyncCount:           e.syncCount,
		ErrorCount:          e.errorCount,
		LastSyncTime:        e.lastSyncTime,
		Uptime:              time.Since(e.startTime),
	}
}

// EngineStatus represents the current status of the syncer engine
type EngineStatus struct {
	Started             bool
	SyncTargetName      string
	WorkspaceCluster    logicalcluster.Name
	ResourceControllers int
	SyncCount           int64
	ErrorCount          int64
	LastSyncTime        time.Time
	Uptime              time.Duration
}