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

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
)

// Manager manages all TMC controllers and coordinates their lifecycle.
// It demonstrates the external controller pattern for KCP by running outside
// KCP and consuming TMC APIs via standard Kubernetes client patterns.
type Manager struct {
	// KCP client and informer infrastructure
	kcpClusterClient kcpclientset.ClusterInterface
	informerFactory  kcpinformers.SharedInformerFactory

	// TMC Controllers
	clusterRegistrationController *ClusterRegistrationController

	// Configuration
	config    *Config
	workspace logicalcluster.Name

	// Lifecycle management
	started bool
	mu      sync.RWMutex
}

// Config contains configuration for the TMC controller manager
type Config struct {
	// Connection configuration
	KCPConfig      *rest.Config
	ClusterConfigs map[string]*rest.Config
	Workspace      string

	// Controller behavior configuration
	ResyncPeriod    time.Duration
	WorkerCount     int

	// TMC-specific configuration
	ClusterHealthCheckInterval time.Duration
	PlacementDecisionTimeout   time.Duration
	MaxConcurrentPlacements    int

	// Feature gates
	EnablePlacementController     bool
	EnableClusterHealthChecking   bool
	EnableWorkloadSynchronization bool
}

// NewManager creates a new TMC controller manager with the specified configuration.
// The manager will create KCP clients and set up informers for the specified workspace.
func NewManager(ctx context.Context, config *Config) (*Manager, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	workspace := logicalcluster.Name(config.Workspace)

	// Create KCP cluster client for accessing TMC APIs
	kcpClusterClient, err := kcpclientset.NewForConfig(config.KCPConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP client: %w", err)
	}

	// Create shared informer factory scoped to our workspace
	// This ensures we only watch TMC resources in our assigned workspace
	informerFactory := kcpinformers.NewSharedInformerFactoryWithOptions(
		kcpClusterClient,
		config.ResyncPeriod,
		kcpinformers.WithCluster(workspace),
	)

	klog.InfoS("Created KCP client and informer factory", 
		"workspace", workspace,
		"resyncPeriod", config.ResyncPeriod)

	manager := &Manager{
		kcpClusterClient: kcpClusterClient,
		informerFactory:  informerFactory,
		config:           config,
		workspace:        workspace,
	}

	// Create controllers based on feature gates
	if err := manager.createControllers(); err != nil {
		return nil, fmt.Errorf("failed to create controllers: %w", err)
	}

	return manager, nil
}

// createControllers creates and configures all enabled TMC controllers
func (m *Manager) createControllers() error {
	var err error

	// Always create cluster registration controller as it's foundational
	m.clusterRegistrationController, err = NewClusterRegistrationController(
		m.kcpClusterClient,
		m.informerFactory.Tmc().V1alpha1().ClusterRegistrations(),
		m.config.ClusterConfigs,
		m.workspace,
		m.config.ClusterHealthCheckInterval,
		m.config.EnableClusterHealthChecking,
	)
	if err != nil {
		return fmt.Errorf("failed to create cluster registration controller: %w", err)
	}

	klog.InfoS("Created ClusterRegistration controller", 
		"healthCheckEnabled", m.config.EnableClusterHealthChecking,
		"healthCheckInterval", m.config.ClusterHealthCheckInterval)

	// Additional controllers will be created here in subsequent phases:
	// - WorkloadPlacement controller (Phase 2 PR 4)
	// - Workload synchronization controller (Phase 3)
	// - Advanced placement engine (Phase 4)

	return nil
}

// Start starts all controllers and begins processing TMC resources.
// This method blocks until the context is cancelled.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("manager is already started")
	}

	klog.InfoS("Starting TMC controller manager", 
		"workspace", m.workspace,
		"clusters", len(m.config.ClusterConfigs),
		"features", m.getEnabledFeatures())

	// Start informer factory to begin caching TMC resources
	m.informerFactory.Start(ctx.Done())

	// Wait for informer caches to sync before starting controllers
	klog.InfoS("Waiting for informer caches to sync")
	
	syncFuncs := m.informerFactory.WaitForCacheSync(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), syncFuncs...) {
		return fmt.Errorf("failed to wait for informer caches to sync")
	}

	klog.InfoS("Informer caches synced, starting controllers")

	var wg sync.WaitGroup

	// Start cluster registration controller
	if m.clusterRegistrationController != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.clusterRegistrationController.Start(ctx, m.config.WorkerCount)
		}()
	}

	// Additional controllers will be started here in subsequent phases

	m.started = true

	// Wait for all controllers to finish
	wg.Wait()
	return nil
}

// GetClusterRegistrationController returns the cluster registration controller
// This allows other components to access the controller if needed
func (m *Manager) GetClusterRegistrationController() *ClusterRegistrationController {
	return m.clusterRegistrationController
}

// GetWorkspace returns the workspace this manager is operating in
func (m *Manager) GetWorkspace() logicalcluster.Name {
	return m.workspace
}

// getEnabledFeatures returns a list of enabled features for logging
func (m *Manager) getEnabledFeatures() []string {
	var features []string
	
	if m.config.EnableClusterHealthChecking {
		features = append(features, "ClusterHealthChecking")
	}
	if m.config.EnablePlacementController {
		features = append(features, "PlacementController")
	}
	if m.config.EnableWorkloadSynchronization {
		features = append(features, "WorkloadSynchronization")
	}
	
	if len(features) == 0 {
		features = append(features, "None")
	}
	
	return features
}

// validateConfig validates the manager configuration
func validateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if config.KCPConfig == nil {
		return fmt.Errorf("KCPConfig cannot be nil")
	}
	if config.Workspace == "" {
		return fmt.Errorf("Workspace cannot be empty")
	}
	if len(config.ClusterConfigs) == 0 {
		return fmt.Errorf("at least one cluster config is required")
	}
	if config.ResyncPeriod <= 0 {
		return fmt.Errorf("ResyncPeriod must be positive")
	}
	if config.WorkerCount <= 0 {
		return fmt.Errorf("WorkerCount must be positive")
	}
	if config.ClusterHealthCheckInterval <= 0 {
		return fmt.Errorf("ClusterHealthCheckInterval must be positive")
	}
	if config.PlacementDecisionTimeout <= 0 {
		return fmt.Errorf("PlacementDecisionTimeout must be positive")
	}
	if config.MaxConcurrentPlacements <= 0 {
		return fmt.Errorf("MaxConcurrentPlacements must be positive")
	}

	return nil
}