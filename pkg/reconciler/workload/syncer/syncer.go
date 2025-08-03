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

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/cmd/workload-syncer/options"
	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// Syncer is the main syncer component that orchestrates workload synchronization
// between KCP logical clusters and physical Kubernetes clusters
type Syncer struct {
	options SyncerOptions

	// Core components
	engine       *Engine
	metrics      *MetricsServer
	healthServer *HealthServer

	// TMC integration
	tmcMetrics *tmc.MetricsCollector
	tmcHealth  *tmc.HealthMonitor

	// State management
	started bool
	stopCh  chan struct{}
	mu      sync.RWMutex
}

// SyncerOptions contains the configuration for the syncer
type SyncerOptions struct {
	KCPConfig     *rest.Config
	ClusterConfig *rest.Config
	SyncerOpts    *options.SyncerOptions
}

// NewSyncer creates a new syncer instance
func NewSyncer(ctx context.Context, opts SyncerOptions) (*Syncer, error) {
	syncer := &Syncer{
		options: opts,
		stopCh:  make(chan struct{}),
	}

	// Initialize TMC integration if enabled
	if opts.SyncerOpts.EnableTMCMetrics {
		syncer.tmcMetrics = tmc.NewMetricsCollector()
	}

	if opts.SyncerOpts.EnableTMCHealth {
		syncer.tmcHealth = tmc.NewHealthMonitor()
	}

	// Create the syncer engine
	engine, err := NewEngine(ctx, opts.KCPConfig, opts.ClusterConfig, opts.SyncerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create syncer engine: %w", err)
	}
	syncer.engine = engine

	// Create metrics server
	metricsServer, err := NewMetricsServer(opts.SyncerOpts.MetricsPort, syncer.tmcMetrics)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics server: %w", err)
	}
	syncer.metrics = metricsServer

	// Create health server
	healthServer, err := NewHealthServer(opts.SyncerOpts.HealthPort, syncer.tmcHealth)
	if err != nil {
		return nil, fmt.Errorf("failed to create health server: %w", err)
	}
	syncer.healthServer = healthServer

	return syncer, nil
}

// Start starts the syncer and all its components
func (s *Syncer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("syncer is already started")
	}

	klog.Info("Starting KCP Workload Syncer...")

	// Start health server
	if err := s.healthServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start health server: %w", err)
	}

	// Start metrics server
	if err := s.metrics.Start(ctx); err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}

	// Start TMC health monitoring
	if s.tmcHealth != nil {
		go s.tmcHealth.Start(ctx)
	}

	// Start the syncer engine
	if err := s.engine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start syncer engine: %w", err)
	}

	s.started = true
	
	klog.Info("KCP Workload Syncer started successfully")
	return nil
}

// Stop stops the syncer and all its components gracefully
func (s *Syncer) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	klog.Info("Stopping KCP Workload Syncer...")

	// Signal all components to stop
	close(s.stopCh)

	// Stop components in reverse order
	var errors []error

	if s.engine != nil {
		if err := s.engine.Stop(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop syncer engine: %w", err))
		}
	}

	// TMC health monitoring stops automatically with context cancellation

	if s.metrics != nil {
		if err := s.metrics.Stop(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop metrics server: %w", err))
		}
	}

	if s.healthServer != nil {
		if err := s.healthServer.Stop(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop health server: %w", err))
		}
	}

	s.started = false

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	klog.Info("KCP Workload Syncer stopped successfully")
	return nil
}

// IsHealthy returns true if the syncer and all its components are healthy
func (s *Syncer) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.started {
		return false
	}

	// Check engine health
	if s.engine != nil && !s.engine.IsHealthy() {
		return false
	}

	// Check TMC health (placeholder for future implementation)
	// TODO: Add proper TMC health check when interface is available

	return true
}

// GetMetrics returns current syncer metrics
func (s *Syncer) GetMetrics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := make(map[string]interface{})
	
	metrics["started"] = s.started
	metrics["healthy"] = s.IsHealthy()
	
	if s.engine != nil {
		engineMetrics := s.engine.GetMetrics()
		for k, v := range engineMetrics {
			metrics["engine."+k] = v
		}
	}

	if s.tmcMetrics != nil {
		// TODO: Add proper TMC metrics collection when interface is available
		metrics["tmc.enabled"] = true
	}

	return metrics
}

// handlePanic recovers from panics in goroutines and logs them
func handlePanic(component string) {
	if r := recover(); r != nil {
		runtime.HandleError(fmt.Errorf("panic in %s: %v", component, r))
	}
}