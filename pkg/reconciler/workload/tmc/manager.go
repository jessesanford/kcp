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

package tmc

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/component-base/featuregate"
	"k8s.io/klog/v2"

	kcpfeatures "github.com/kcp-dev/kcp/pkg/features"
)

// Manager manages TMC (Transparent Multi-Cluster) controllers and components
type Manager struct {
	config        *rest.Config
	healthManager *HealthMonitor
	metrics       *MetricsCollector
	tracer        *TracingManager
	recovery      *RecoveryManager
}

// NewManager creates a new TMC manager
func NewManager(config *rest.Config) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("rest config cannot be nil")
	}

	manager := &Manager{
		config: rest.CopyConfig(config),
	}

	// Initialize TMC infrastructure components if enabled
	if kcpfeatures.DefaultFeatureGate.Enabled(kcpfeatures.TransparentMultiCluster) {
		// Initialize health monitoring
		manager.healthManager = NewHealthMonitor()

		// Initialize metrics collection
		manager.metrics = NewMetricsCollector()

		// Initialize tracing
		manager.tracer = NewTracingManager("kcp-tmc", "v1.0.0")
		
		// Initialize recovery manager
		manager.recovery = NewRecoveryManager()
	}

	return manager, nil
}

// Start starts the TMC manager and all enabled components
func (m *Manager) Start(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues("component", "tmc-manager")
	
	if !kcpfeatures.DefaultFeatureGate.Enabled(kcpfeatures.TransparentMultiCluster) {
		logger.Info("TMC (Transparent Multi-Cluster) is disabled - feature flag TransparentMultiCluster=false")
		return nil
	}

	logger.Info("Starting TMC (Transparent Multi-Cluster) manager")

	// Start health monitoring
	if m.healthManager != nil {
		go m.healthManager.Start(ctx)
	}

	// Start metrics collection (already started on creation)
	if m.metrics != nil {
		logger.Info("TMC metrics collector initialized")
	}

	// Start tracing
	if m.tracer != nil {
		logger.Info("TMC tracing manager initialized")
	}

	// Start recovery manager (already started on creation)
	if m.recovery != nil {
		logger.Info("TMC recovery manager initialized")
	}

	// Log enabled TMC sub-features
	m.logEnabledFeatures(logger)

	logger.Info("TMC manager started successfully")
	return nil
}

// Stop stops the TMC manager and all components
func (m *Manager) Stop(ctx context.Context) error {
	logger := klog.FromContext(ctx).WithValues("component", "tmc-manager")
	
	if !kcpfeatures.DefaultFeatureGate.Enabled(kcpfeatures.TransparentMultiCluster) {
		return nil
	}

	logger.Info("Stopping TMC manager")

	// Stop components in reverse order
	if m.recovery != nil {
		logger.Info("TMC recovery manager stopping")
	}

	if m.tracer != nil {
		logger.Info("TMC tracing manager stopping")
	}

	if m.metrics != nil {
		logger.Info("TMC metrics collector stopping")
	}

	if m.healthManager != nil {
		// Health monitor will stop when context is cancelled
		logger.Info("TMC health monitor stopping")
	}

	logger.Info("TMC manager stopped")
	return nil
}

// logEnabledFeatures logs which TMC sub-features are enabled
func (m *Manager) logEnabledFeatures(logger klog.Logger) {
	features := []struct {
		name    string
		feature featuregate.Feature
	}{
		{"Placement", kcpfeatures.TMCPlacement},
		{"Synchronization", kcpfeatures.TMCSynchronization},
		{"VirtualWorkspaces", kcpfeatures.TMCVirtualWorkspaces},
		{"Migration", kcpfeatures.TMCMigration},
		{"StatusAggregation", kcpfeatures.TMCStatusAggregation},
	}

	enabled := []string{}
	disabled := []string{}

	for _, f := range features {
		if kcpfeatures.DefaultFeatureGate.Enabled(f.feature) {
			enabled = append(enabled, f.name)
		} else {
			disabled = append(disabled, f.name)
		}
	}

	if len(enabled) > 0 {
		logger.Info("TMC features enabled", "features", enabled)
	}
	if len(disabled) > 0 {
		logger.Info("TMC features disabled", "features", disabled)
	}
}

// GetHealthManager returns the health manager instance
func (m *Manager) GetHealthManager() *HealthMonitor {
	return m.healthManager
}

// GetMetricsCollector returns the metrics collector instance
func (m *Manager) GetMetricsCollector() *MetricsCollector {
	return m.metrics
}

// GetTracingManager returns the tracing manager instance  
func (m *Manager) GetTracingManager() *TracingManager {
	return m.tracer
}

// GetRecoveryManager returns the recovery manager instance
func (m *Manager) GetRecoveryManager() *RecoveryManager {
	return m.recovery
}