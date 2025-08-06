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

	// Observability
	metricsServer *http.Server
	healthServer  *http.Server

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

	manager := &Manager{
		kcpClusterClient: kcpClusterClient,
		informerFactory:  informerFactory,
		config:           config,
		workspace:        workspace,
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
	for informerType, synced := range cacheSynced {
		if !synced {
			return fmt.Errorf("failed to sync cache for %v", informerType)
		}
	}

	// Block until context is cancelled
	<-ctx.Done()
	klog.InfoS("TMC controller manager shutting down")

	return nil
}

// Shutdown gracefully shuts down the manager and all controllers.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if !m.started || m.stopping {
		m.mu.Unlock()
		return nil
	}
	m.stopping = true
	m.mu.Unlock()

	klog.InfoS("Shutting down TMC controller manager")

	var shutdownErrors []error

	// Shutdown observability servers
	if m.metricsServer != nil {
		if err := m.metricsServer.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("metrics server shutdown: %w", err))
		}
	}

	if m.healthServer != nil {
		if err := m.healthServer.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("health server shutdown: %w", err))
		}
	}

	if len(shutdownErrors) > 0 {
		return fmt.Errorf("shutdown errors: %v", shutdownErrors)
	}

	klog.InfoS("TMC controller manager shutdown complete")
	return nil
}

// validateConfig validates the manager configuration.
func validateConfig(config *Config) error {
	if config.KCPConfig == nil {
		return fmt.Errorf("KCPConfig is required")
	}
	if config.Workspace == "" {
		return fmt.Errorf("Workspace is required")
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
		mux.HandleFunc("/readyz", m.readinessHandler)
		
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
			if err := m.metricsServer.ListenAndServe(); err != http.ErrServerClosed {
				klog.ErrorS(err, "Metrics server error")
			}
		}()
	}

	if m.healthServer != nil {
		go func() {
			klog.InfoS("Starting health server", "port", m.config.HealthPort)
			if err := m.healthServer.ListenAndServe(); err != http.ErrServerClosed {
				klog.ErrorS(err, "Health server error")
			}
		}()
	}

	return nil
}

// healthHandler provides health check endpoint.
func (m *Manager) healthHandler(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.started || m.stopping {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// readinessHandler provides readiness check endpoint.
func (m *Manager) readinessHandler(w http.ResponseWriter, r *http.Request) {
	m.healthHandler(w, r) // Same logic for now
}
