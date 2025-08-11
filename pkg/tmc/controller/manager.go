// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package controller implements the TMC external controller foundation.
// This provides basic controller lifecycle management, workqueue processing,
// and coordination infrastructure for TMC controllers.
package controller

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions/cluster"
)

// Config contains configuration for the TMC controller manager
type Config struct {
	// KCP client configuration
	KCPConfig      *rest.Config
	ClusterConfigs map[string]*rest.Config
	
	// Controller settings
	Workspace    logicalcluster.Name
	ResyncPeriod time.Duration
	WorkerCount  int
	
	// Observability
	MetricsPort int
	HealthPort  int
}

// Manager manages the lifecycle of TMC controllers
type Manager struct {
	// Core clients
	kcpClient        kcpclientset.ClusterInterface
	clusterClients   map[string]kubernetes.Interface
	informerFactory  kcpinformers.SharedInformerFactory
	
	// Configuration
	config *Config
	
	// Controller lifecycle
	controllers []Controller
	stopCh      chan struct{}
	wg          sync.WaitGroup
	
	// HTTP servers
	metricsServer *http.Server
	healthServer  *http.Server
}

// Controller defines the interface for TMC controllers
type Controller interface {
	// Start runs the controller until context is cancelled
	Start(ctx context.Context) error
	
	// Name returns the controller name for logging
	Name() string
}

// NewManager creates a new TMC controller manager
func NewManager(ctx context.Context, config *Config) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	// Create KCP client
	kcpClient, err := kcpclientset.NewForConfig(config.KCPConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP client: %w", err)
	}
	
	// Create cluster clients
	clusterClients := make(map[string]kubernetes.Interface)
	for name, clientConfig := range config.ClusterConfigs {
		client, err := kubernetes.NewForConfig(clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for cluster %s: %w", name, err)
		}
		clusterClients[name] = client
		
		klog.V(2).InfoS("Created cluster client", "cluster", name)
	}
	
	// Create shared informer factory
	informerFactory := kcpinformers.NewSharedInformerFactoryWithOptions(
		kcpClient,
		config.ResyncPeriod,
		kcpinformers.WithNamespace(""),
	)
	
	mgr := &Manager{
		kcpClient:       kcpClient,
		clusterClients:  clusterClients,
		informerFactory: informerFactory,
		config:          config,
		controllers:     make([]Controller, 0),
		stopCh:          make(chan struct{}),
	}
	
	// Initialize controllers
	if err := mgr.initializeControllers(); err != nil {
		return nil, fmt.Errorf("failed to initialize controllers: %w", err)
	}
	
	// Setup HTTP servers
	if err := mgr.setupHTTPServers(); err != nil {
		return nil, fmt.Errorf("failed to setup HTTP servers: %w", err)
	}
	
	klog.InfoS("TMC controller manager created",
		"workspace", config.Workspace,
		"clusters", len(config.ClusterConfigs),
		"controllers", len(mgr.controllers))
	
	return mgr, nil
}

// Start starts the controller manager and all controllers
func (m *Manager) Start(ctx context.Context) error {
	klog.InfoS("Starting TMC controller manager", 
		"controllers", len(m.controllers),
		"workspace", m.config.Workspace)
	
	// Start shared informers
	klog.V(2).InfoS("Starting shared informers")
	m.informerFactory.Start(m.stopCh)
	
	// Wait for informer caches to sync
	klog.V(2).InfoS("Waiting for informer caches to sync")
	for gvr, ok := range m.informerFactory.WaitForCacheSync(m.stopCh) {
		if !ok {
			return fmt.Errorf("failed to sync cache for %v", gvr)
		}
	}
	klog.V(2).InfoS("All informer caches synced")
	
	// Start HTTP servers
	if err := m.startHTTPServers(); err != nil {
		return fmt.Errorf("failed to start HTTP servers: %w", err)
	}
	
	// Start all controllers
	for _, controller := range m.controllers {
		c := controller // capture loop variable
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			klog.V(2).InfoS("Starting controller", "controller", c.Name())
			
			if err := c.Start(ctx); err != nil {
				klog.ErrorS(err, "Controller failed", "controller", c.Name())
			}
			klog.V(2).InfoS("Controller stopped", "controller", c.Name())
		}()
	}
	
	klog.InfoS("All controllers started, waiting for completion")
	
	// Wait for context cancellation or all controllers to finish
	<-ctx.Done()
	klog.InfoS("Context cancelled, initiating shutdown")
	
	return nil
}

// Shutdown gracefully shuts down the controller manager
func (m *Manager) Shutdown(ctx context.Context) error {
	klog.InfoS("Shutting down TMC controller manager")
	
	// Stop informers first
	close(m.stopCh)
	
	// Shutdown HTTP servers with timeout
	shutdownDone := make(chan error, 2)
	
	if m.metricsServer != nil {
		go func() {
			shutdownDone <- m.metricsServer.Shutdown(ctx)
		}()
	}
	
	if m.healthServer != nil {
		go func() {
			shutdownDone <- m.healthServer.Shutdown(ctx)
		}()
	}
	
	// Wait for HTTP servers to shutdown
	for i := 0; i < 2; i++ {
		select {
		case err := <-shutdownDone:
			if err != nil {
				klog.ErrorS(err, "Error shutting down HTTP server")
			}
		case <-ctx.Done():
			klog.ErrorS(ctx.Err(), "Timeout waiting for HTTP servers to shutdown")
			break
		}
	}
	
	// Wait for all controllers to finish with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		klog.InfoS("All controllers shut down successfully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for controllers to shutdown: %w", ctx.Err())
	}
}

// initializeControllers creates and configures all TMC controllers
func (m *Manager) initializeControllers() error {
	// For now, just create a basic registry controller
	// This will be expanded as more controllers are added
	
	registryController, err := NewBasicController(
		m.kcpClient,
		m.clusterClients,
		m.config.Workspace,
		m.config.ResyncPeriod,
		m.config.WorkerCount,
	)
	if err != nil {
		return fmt.Errorf("failed to create registry controller: %w", err)
	}
	
	m.controllers = append(m.controllers, registryController)
	
	klog.V(2).InfoS("Initialized controllers", "count", len(m.controllers))
	return nil
}

// setupHTTPServers configures metrics and health HTTP servers
func (m *Manager) setupHTTPServers() error {
	if m.config.MetricsPort > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/metrics", m.metricsHandler)
		m.metricsServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", m.config.MetricsPort),
			Handler: mux,
		}
		klog.V(2).InfoS("Configured metrics server", "port", m.config.MetricsPort)
	}
	
	if m.config.HealthPort > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", m.healthHandler)
		mux.HandleFunc("/readyz", m.readinessHandler)
		m.healthServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", m.config.HealthPort),
			Handler: mux,
		}
		klog.V(2).InfoS("Configured health server", "port", m.config.HealthPort)
	}
	
	return nil
}

// startHTTPServers starts the configured HTTP servers
func (m *Manager) startHTTPServers() error {
	if m.metricsServer != nil {
		go func() {
			klog.InfoS("Starting metrics server", "addr", m.metricsServer.Addr)
			if err := m.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				klog.ErrorS(err, "Metrics server failed")
			}
		}()
	}
	
	if m.healthServer != nil {
		go func() {
			klog.InfoS("Starting health server", "addr", m.healthServer.Addr)
			if err := m.healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				klog.ErrorS(err, "Health server failed")
			}
		}()
	}
	
	return nil
}

// HTTP handlers

func (m *Manager) metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Basic metrics endpoint - will be expanded later
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "# TMC Controller Metrics\n")
	fmt.Fprintf(w, "tmc_controllers_total %d\n", len(m.controllers))
	fmt.Fprintf(w, "tmc_clusters_total %d\n", len(m.clusterClients))
}

func (m *Manager) healthHandler(w http.ResponseWriter, r *http.Request) {
	// Basic health check - will be expanded with actual health monitoring
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}

func (m *Manager) readinessHandler(w http.ResponseWriter, r *http.Request) {
	// Basic readiness check
	// TODO: Add proper readiness checks for controllers
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ready")
}

// validateConfig validates the manager configuration
func validateConfig(config *Config) error {
	if config.KCPConfig == nil {
		return fmt.Errorf("KCPConfig is required")
	}
	
	if len(config.ClusterConfigs) == 0 {
		return fmt.Errorf("at least one cluster config is required")
	}
	
	if config.WorkerCount <= 0 {
		return fmt.Errorf("WorkerCount must be positive")
	}
	
	if config.ResyncPeriod <= 0 {
		return fmt.Errorf("ResyncPeriod must be positive")
	}
	
	return nil
}