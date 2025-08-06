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

// Package controller provides the TMC controller manager and foundation components.
// The manager coordinates multiple controllers and handles their lifecycle, ensuring
// proper startup, shutdown, and observability for the TMC system.
package controller

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	"github.com/kcp-dev/logicalcluster/v3"
)

// Manager manages the lifecycle of all TMC controllers.
// It provides centralized coordination, metrics, health checking,
// and graceful shutdown capabilities for the TMC controller system.
type Manager struct {
	// Core components
	kcpClusterClient kcpclientset.ClusterInterface
	informerFactory  kcpinformers.SharedInformerFactory

	// Configuration
	config    *Config
	workspace logicalcluster.Name

	// Controller components
	baseController                BaseController
	clusterRegistrationController *ClusterRegistrationController

	// Observability
	metricsServer *http.Server
	healthServer  *http.Server
	metrics       *ManagerMetrics

	// Lifecycle management
	mu       sync.RWMutex
	started  bool
	stopping bool
}

// Config contains configuration for the TMC controller manager.
// This configuration is provided by the main function and contains
// all necessary settings for controller operation.
type Config struct {
	// KCP connection configuration
	KCPConfig *rest.Config
	Workspace string

	// Physical cluster configurations
	ClusterConfigs map[string]*rest.Config

	// Operational parameters
	ResyncPeriod time.Duration
	WorkerCount  int

	// Observability configuration
	MetricsPort int
	HealthPort  int
}

// ManagerMetrics provides basic metrics for the controller manager.
type ManagerMetrics struct {
	controllersTotal   prometheus.Gauge
	controllersHealthy prometheus.Gauge
	reconcileTotal     *prometheus.CounterVec
}

// NewManager creates a new TMC controller manager.
// It initializes all necessary components and prepares them for startup,
// following KCP patterns for external controller management.
func NewManager(ctx context.Context, config *Config) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	workspace := logicalcluster.Name(config.Workspace)

	// Create KCP cluster client
	kcpClusterClient, err := kcpclientset.NewForConfig(config.KCPConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create KCP cluster client: %w", err)
	}

	// Create informer factory with workspace scope
	informerFactory := kcpinformers.NewSharedInformerFactory(
		kcpClusterClient,
		config.ResyncPeriod,
	)

	// Initialize metrics
	metrics := newManagerMetrics()

	// Create base controller
	baseController := NewBaseController(&BaseControllerConfig{
		Name:            "tmc-manager",
		ResyncPeriod:    config.ResyncPeriod,
		WorkerCount:     config.WorkerCount,
		Metrics:         metrics,
		InformerFactory: informerFactory,
	})

	// Create cluster registration controller
	clusterRegistrationController, err := NewClusterRegistrationController(
		kcpClusterClient,
		config.ClusterConfigs,
		workspace,
		config.ResyncPeriod,
		config.WorkerCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster registration controller: %w", err)
	}

	manager := &Manager{
		kcpClusterClient:              kcpClusterClient,
		informerFactory:               informerFactory,
		config:                        config,
		workspace:                     workspace,
		baseController:                baseController,
		clusterRegistrationController: clusterRegistrationController,
		metrics:                       metrics,
	}

	// Initialize observability servers
	if err := manager.initializeObservabilityServers(); err != nil {
		return nil, fmt.Errorf("failed to initialize observability servers: %w", err)
	}

	klog.InfoS("TMC controller manager initialized",
		"workspace", workspace,
		"clusters", len(config.ClusterConfigs),
		"resyncPeriod", config.ResyncPeriod,
		"workers", config.WorkerCount)

	return manager, nil
}

// Start starts all controllers and their dependencies.
// This method blocks until the context is cancelled or an error occurs.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("manager already started")
	}
	m.started = true
	m.mu.Unlock()

	klog.InfoS("Starting TMC controller manager", "workspace", m.workspace)

	// Update metrics
	m.metrics.controllersTotal.Set(2) // Base controller + cluster registration controller

	// Start observability servers
	if err := m.startObservabilityServers(ctx); err != nil {
		return fmt.Errorf("failed to start observability servers: %w", err)
	}

	// Start informer factory
	klog.V(2).InfoS("Starting informer factory")
	m.informerFactory.Start(ctx.Done())

	// Wait for cache sync
	klog.V(2).InfoS("Waiting for informer caches to sync")
	cacheSynced := m.informerFactory.WaitForCacheSync(ctx.Done())
	
	syncSuccess := true
	for informer, synced := range cacheSynced {
		if !synced {
			klog.ErrorS(nil, "Failed to sync cache", "informer", informer)
			syncSuccess = false
		}
	}

	if !syncSuccess {
		return fmt.Errorf("failed to sync informer caches")
	}
	klog.InfoS("All informer caches synced successfully")

	var wg sync.WaitGroup

	// Start base controller (foundation for future controllers)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				klog.ErrorS(nil, "Base controller panicked", "panic", r)
			}
		}()

		if err := m.baseController.Start(ctx); err != nil {
			klog.ErrorS(err, "Base controller failed")
		}
	}()

	// Start cluster registration controller
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				klog.ErrorS(nil, "Cluster registration controller panicked", "panic", r)
			}
		}()

		if err := m.clusterRegistrationController.Start(ctx); err != nil {
			klog.ErrorS(err, "Cluster registration controller failed")
		}
	}()

	// Update healthy controllers count
	m.metrics.controllersHealthy.Set(2)

	klog.InfoS("All TMC controllers started successfully")

	// Block until context is done
	<-ctx.Done()
	klog.InfoS("Received shutdown signal, stopping controllers")

	// Mark as stopping
	m.mu.Lock()
	m.stopping = true
	m.mu.Unlock()

	// Wait for controllers to finish
	wg.Wait()

	klog.InfoS("All controllers stopped")
	return nil
}

// Shutdown gracefully shuts down all controllers and servers.
// It attempts to complete in-flight work within the given context timeout.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if !m.started || m.stopping {
		m.mu.Unlock()
		return nil
	}
	m.stopping = true
	m.mu.Unlock()

	klog.InfoS("Shutting down TMC controller manager")

	var errs []error

	// Shutdown base controller
	if err := m.baseController.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("base controller shutdown failed: %w", err))
	}

	// Shutdown observability servers
	if m.metricsServer != nil {
		if err := m.metricsServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("metrics server shutdown failed: %w", err))
		}
	}

	if m.healthServer != nil {
		if err := m.healthServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("health server shutdown failed: %w", err))
		}
	}

	// Update metrics
	m.metrics.controllersHealthy.Set(0)

	if len(errs) > 0 {
		return fmt.Errorf("shutdown completed with errors: %v", errs)
	}

	klog.InfoS("TMC controller manager shutdown completed successfully")
	return nil
}

// IsHealthy returns true if the manager and all controllers are healthy.
// This is used by the health check endpoint and external monitoring.
func (m *Manager) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.started || m.stopping {
		return false
	}

	// Check base controller health
	baseHealthy := m.baseController.IsHealthy()
	clusterHealthy := m.clusterRegistrationController.IsHealthy()
	
	return baseHealthy && clusterHealthy
}

// validateConfig validates the manager configuration.
func validateConfig(config *Config) error {
	if config.KCPConfig == nil {
		return fmt.Errorf("KCPConfig is required")
	}
	if config.Workspace == "" {
		return fmt.Errorf("Workspace is required")
	}
	if len(config.ClusterConfigs) == 0 {
		return fmt.Errorf("at least one ClusterConfig is required")
	}
	if config.ResyncPeriod <= 0 {
		return fmt.Errorf("ResyncPeriod must be positive")
	}
	if config.WorkerCount <= 0 {
		return fmt.Errorf("WorkerCount must be positive")
	}
	return nil
}

// initializeObservabilityServers sets up metrics and health servers.
func (m *Manager) initializeObservabilityServers() error {
	// Initialize metrics server
	if m.config.MetricsPort > 0 {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		m.metricsServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", m.config.MetricsPort),
			Handler: mux,
		}
	}

	// Initialize health server
	if m.config.HealthPort > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", m.healthHandler)
		mux.HandleFunc("/readyz", m.readyHandler)
		m.healthServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", m.config.HealthPort),
			Handler: mux,
		}
	}

	return nil
}

// startObservabilityServers starts the metrics and health servers.
func (m *Manager) startObservabilityServers(ctx context.Context) error {
	if m.metricsServer != nil {
		go func() {
			klog.InfoS("Starting metrics server", "port", m.config.MetricsPort)
			if err := m.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				klog.ErrorS(err, "Metrics server failed")
			}
		}()
	}

	if m.healthServer != nil {
		go func() {
			klog.InfoS("Starting health server", "port", m.config.HealthPort)
			if err := m.healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				klog.ErrorS(err, "Health server failed")
			}
		}()
	}

	return nil
}

// healthHandler handles health check requests.
func (m *Manager) healthHandler(w http.ResponseWriter, r *http.Request) {
	if m.IsHealthy() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not healthy"))
	}
}

// readyHandler handles readiness check requests.
func (m *Manager) readyHandler(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	ready := m.started && !m.stopping
	m.mu.RUnlock()

	if ready {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not ready"))
	}
}

// newManagerMetrics creates basic metrics for the manager.
func newManagerMetrics() *ManagerMetrics {
	metrics := &ManagerMetrics{
		controllersTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "tmc_controllers_total",
				Help: "Total number of TMC controllers",
			},
		),
		controllersHealthy: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "tmc_controllers_healthy",
				Help: "Number of healthy TMC controllers",
			},
		),
		reconcileTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "tmc_reconcile_total",
				Help: "Total number of reconciliation attempts",
			},
			[]string{"controller", "result"},
		),
	}

	// Register metrics
	prometheus.MustRegister(
		metrics.controllersTotal,
		metrics.controllersHealthy,
		metrics.reconcileTotal,
	)

	return metrics
}