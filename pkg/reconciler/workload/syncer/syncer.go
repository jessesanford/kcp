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

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"

	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// Syncer is the main syncer implementation that coordinates all syncer components
type Syncer struct {
	// Configuration
	options SyncerOptions
	
	// Core components
	engine       *Engine
	metrics      *MetricsServer
	
	// TMC Integration
	tmcMetrics *tmc.MetricsCollector
	tmcHealth  *tmc.HealthMonitor
	
	// State
	started bool
	stopCh  chan struct{}
	mu      sync.RWMutex
}

// SyncerOptions configures the syncer
type SyncerOptions struct {
	SyncTargetName   string
	SyncTargetUID    string
	WorkspaceCluster logicalcluster.Name
	KCPConfig        *rest.Config
	ClusterConfig    *rest.Config
	ResyncPeriod     time.Duration
	Workers          int
	HeartbeatPeriod  time.Duration
}

// NewSyncer creates a new syncer instance
func NewSyncer(options SyncerOptions) (*Syncer, error) {
	logger := klog.Background().WithValues("component", "Syncer", "syncTarget", options.SyncTargetName)
	logger.Info("Creating syncer")

	// Validate options
	if err := validateSyncerOptions(options); err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeConfiguration, "syncer", "validation").
			WithMessage("Invalid syncer options").
			WithCause(err).
			Build()
	}

	// Initialize TMC components
	tmcMetrics := tmc.NewMetricsCollector()
	tmcHealth := tmc.NewHealthMonitor()

	syncer := &Syncer{
		options:    options,
		tmcMetrics: tmcMetrics,
		tmcHealth:  tmcHealth,
		stopCh:     make(chan struct{}),
	}

	// Create engine
	engine, err := NewEngine(&EngineOptions{
		SyncTargetName:   options.SyncTargetName,
		SyncTargetUID:    options.SyncTargetUID,
		WorkspaceCluster: options.WorkspaceCluster,
		KCPConfig:        options.KCPConfig,
		ClusterConfig:    options.ClusterConfig,
		ResyncPeriod:     options.ResyncPeriod,
		Workers:          options.Workers,
	})
	if err != nil {
		return nil, tmc.NewTMCError(tmc.TMCErrorTypeConfiguration, "syncer", "create-engine").
			WithMessage("Failed to create syncer engine").
			WithCause(err).
			Build()
	}
	syncer.engine = engine

	// Create metrics server
	metrics := NewMetricsServer(MetricsServerOptions{
		SyncTargetName:   options.SyncTargetName,
		WorkspaceCluster: options.WorkspaceCluster.String(),
		TMCMetrics:       tmcMetrics,
	})
	syncer.metrics = metrics

	logger.Info("Successfully created syncer")
	return syncer, nil
}

// Start starts the syncer and all its components
func (s *Syncer) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues("component", "Syncer", "syncTarget", s.options.SyncTargetName)
	logger.Info("Starting syncer")

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("syncer already started")
	}
	s.started = true
	s.mu.Unlock()

	// Start TMC health monitoring
	go s.tmcHealth.Start(ctx)

	// Start metrics server
	if err := s.metrics.Start(ctx); err != nil {
		return tmc.NewTMCError(tmc.TMCErrorTypeInternal, "syncer", "start-metrics").
			WithMessage("Failed to start metrics server").
			WithCause(err).
			Build()
	}

	// Start syncer engine (this will start all other components)
	if err := s.engine.Start(ctx); err != nil {
		return tmc.NewTMCError(tmc.TMCErrorTypeInternal, "syncer", "start-engine").
			WithMessage("Failed to start syncer engine").
			WithCause(err).
			Build()
	}

	// Record startup metrics
	s.tmcMetrics.RecordComponentOperation("syncer", s.options.SyncTargetName, "start", "success")

	logger.Info("Syncer started successfully")
	return nil
}

// Stop stops the syncer and all its components
func (s *Syncer) Stop() {
	logger := klog.Background().WithValues("component", "Syncer", "syncTarget", s.options.SyncTargetName)
	logger.Info("Stopping syncer")

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	// Signal all components to stop
	close(s.stopCh)

	// Stop engine (this will stop all other syncer components)
	if s.engine != nil {
		s.engine.Stop()
	}

	// Stop metrics server
	if s.metrics != nil {
		s.metrics.Stop()
	}

	// Stop TMC health monitoring
	s.tmcHealth.Stop()

	s.started = false

	// Record shutdown metrics
	s.tmcMetrics.RecordComponentOperation("syncer", s.options.SyncTargetName, "stop", "success")

	logger.Info("Syncer stopped")
}

// GetStatus returns the current status of the syncer
func (s *Syncer) GetStatus() *SyncerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &SyncerStatus{
		SyncTargetName:   s.options.SyncTargetName,
		WorkspaceCluster: s.options.WorkspaceCluster,
		Started:          s.started,
	}

	// Get engine status
	if s.engine != nil {
		engineStatus := s.engine.GetStatus()
		status.Engine = engineStatus
	}

	// Get TMC health status
	if s.tmcHealth != nil {
		overallHealth := s.tmcHealth.GetOverallHealth()
		status.Health = &SyncerHealthStatus{
			Status:     overallHealth.Status,
			Message:    overallHealth.Message,
			Components: overallHealth.Details,
		}
	}

	// Get metrics snapshot
	if s.metrics != nil {
		status.Metrics = s.metrics.GetMetricsSnapshot()
	}

	return status
}

// GetHealth returns the current health status
func (s *Syncer) GetHealth(ctx context.Context) *tmc.HealthCheck {
	if s.engine != nil && s.engine.healthMonitor != nil {
		return s.engine.healthMonitor.GetHealth(ctx)
	}

	return &tmc.HealthCheck{
		ComponentType: tmc.ComponentTypeSyncTargetController,
		ComponentID:   s.options.SyncTargetName,
		Status:        tmc.HealthStatusUnhealthy,
		Message:       "Syncer not properly initialized",
		Timestamp:     time.Now(),
	}
}

// SyncerStatus represents the overall status of the syncer
type SyncerStatus struct {
	SyncTargetName   string
	WorkspaceCluster logicalcluster.Name
	Started          bool
	Engine           *EngineStatus
	Health           *SyncerHealthStatus
	Metrics          map[string]interface{}
}

// SyncerHealthStatus represents the health status of the syncer
type SyncerHealthStatus struct {
	Status     tmc.HealthStatus
	Message    string
	Components map[string]interface{}
}

// validateSyncerOptions validates the syncer configuration
func validateSyncerOptions(options SyncerOptions) error {
	if options.SyncTargetName == "" {
		return fmt.Errorf("SyncTargetName is required")
	}
	if options.SyncTargetUID == "" {
		return fmt.Errorf("SyncTargetUID is required")
	}
	if options.WorkspaceCluster.Empty() {
		return fmt.Errorf("WorkspaceCluster is required")
	}
	if options.KCPConfig == nil {
		return fmt.Errorf("KCPConfig is required")
	}
	if options.ClusterConfig == nil {
		return fmt.Errorf("ClusterConfig is required")
	}

	// Set defaults
	if options.ResyncPeriod == 0 {
		options.ResyncPeriod = 30 * time.Second
	}
	if options.Workers == 0 {
		options.Workers = 2
	}
	if options.HeartbeatPeriod == 0 {
		options.HeartbeatPeriod = 30 * time.Second
	}

	return nil
}

// SyncerManager manages multiple syncer instances
type SyncerManager struct {
	syncers map[string]*Syncer
	mu      sync.RWMutex
}

// NewSyncerManager creates a new syncer manager
func NewSyncerManager() *SyncerManager {
	return &SyncerManager{
		syncers: make(map[string]*Syncer),
	}
}

// AddSyncer adds a syncer to the manager
func (sm *SyncerManager) AddSyncer(syncer *Syncer) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.syncers[syncer.options.SyncTargetName] = syncer
}

// RemoveSyncer removes a syncer from the manager
func (sm *SyncerManager) RemoveSyncer(syncTargetName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if syncer, exists := sm.syncers[syncTargetName]; exists {
		syncer.Stop()
		delete(sm.syncers, syncTargetName)
	}
}

// GetSyncer returns a syncer by name
func (sm *SyncerManager) GetSyncer(syncTargetName string) (*Syncer, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	syncer, exists := sm.syncers[syncTargetName]
	return syncer, exists
}

// ListSyncers returns all syncers
func (sm *SyncerManager) ListSyncers() map[string]*Syncer {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]*Syncer)
	for name, syncer := range sm.syncers {
		result[name] = syncer
	}
	return result
}

// StopAll stops all syncers
func (sm *SyncerManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, syncer := range sm.syncers {
		syncer.Stop()
	}
	sm.syncers = make(map[string]*Syncer)
}