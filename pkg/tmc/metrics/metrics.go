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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kcp-dev/kcp/pkg/features"
)

const (
	// TMCSubsystem is the metrics subsystem for TMC
	TMCSubsystem = "tmc"
	
	// Common label names
	WorkspaceLabel         = "workspace"
	ClusterLabel          = "cluster"
	OperationLabel        = "operation"
	StatusLabel           = "status"
	ResourceTypeLabel     = "resource_type"
	SyncerLabel           = "syncer"
)

var (
	// TMC cluster registration metrics
	clusterRegistrations = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: TMCSubsystem,
			Name:      "cluster_registrations_total",
			Help:      "Total number of TMC cluster registrations by status",
		},
		[]string{WorkspaceLabel, StatusLabel},
	)

	// TMC workload placement metrics
	workloadPlacements = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: TMCSubsystem,
			Name:      "workload_placements_total",
			Help:      "Total number of TMC workload placements by status",
		},
		[]string{WorkspaceLabel, ClusterLabel, StatusLabel},
	)

	// TMC syncer operation metrics
	syncerOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: TMCSubsystem,
			Name:      "syncer_operations_total",
			Help:      "Total number of TMC syncer operations",
		},
		[]string{WorkspaceLabel, SyncerLabel, OperationLabel, StatusLabel},
	)

	// TMC syncer operation duration
	syncerOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: TMCSubsystem,
			Name:      "syncer_operation_duration_seconds",
			Help:      "Duration of TMC syncer operations in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{WorkspaceLabel, SyncerLabel, OperationLabel},
	)

	// TMC resource sync metrics
	resourceSyncs = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: TMCSubsystem,
			Name:      "resource_syncs_total", 
			Help:      "Total number of TMC resource sync operations",
		},
		[]string{WorkspaceLabel, ClusterLabel, ResourceTypeLabel, StatusLabel},
	)

	// TMC workload sync metrics
	workloadSyncs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: TMCSubsystem,
			Name:      "workload_syncs_active",
			Help:      "Number of active TMC workload sync operations",
		},
		[]string{WorkspaceLabel, ClusterLabel},
	)

	// TMC controller metrics
	controllerReconciliations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: TMCSubsystem,
			Name:      "controller_reconciliations_total",
			Help:      "Total number of TMC controller reconciliations",
		},
		[]string{WorkspaceLabel, "controller", StatusLabel},
	)

	// TMC controller reconciliation duration
	controllerReconciliationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: TMCSubsystem,
			Name:      "controller_reconciliation_duration_seconds",
			Help:      "Duration of TMC controller reconciliations in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{WorkspaceLabel, "controller"},
	)
)

// TMCMetrics provides methods for recording TMC metrics.
type TMCMetrics struct {
	enabled bool
}

// NewTMCMetrics creates a new TMC metrics recorder.
func NewTMCMetrics() *TMCMetrics {
	return &TMCMetrics{
		enabled: features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled),
	}
}

// RegisterMetrics registers TMC metrics with the controller runtime metrics registry.
func RegisterMetrics() {
	if !features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled) {
		return
	}

	metrics.Registry.MustRegister(
		clusterRegistrations,
		workloadPlacements,
		syncerOperations,
		syncerOperationDuration,
		resourceSyncs,
		workloadSyncs,
		controllerReconciliations,
		controllerReconciliationDuration,
	)
}

// RecordClusterRegistration records cluster registration metrics.
func (m *TMCMetrics) RecordClusterRegistration(workspace, status string) {
	if !m.enabled {
		return
	}
	clusterRegistrations.WithLabelValues(workspace, status).Inc()
}

// RecordWorkloadPlacement records workload placement metrics.
func (m *TMCMetrics) RecordWorkloadPlacement(workspace, cluster, status string) {
	if !m.enabled {
		return
	}
	workloadPlacements.WithLabelValues(workspace, cluster, status).Inc()
}

// RecordSyncerOperation records syncer operation metrics.
func (m *TMCMetrics) RecordSyncerOperation(workspace, syncer, operation, status string) {
	if !m.enabled {
		return
	}
	syncerOperations.WithLabelValues(workspace, syncer, operation, status).Inc()
}

// ObserveSyncerOperationDuration records syncer operation duration.
func (m *TMCMetrics) ObserveSyncerOperationDuration(workspace, syncer, operation string, duration float64) {
	if !m.enabled {
		return
	}
	syncerOperationDuration.WithLabelValues(workspace, syncer, operation).Observe(duration)
}

// RecordResourceSync records resource sync metrics.
func (m *TMCMetrics) RecordResourceSync(workspace, cluster, resourceType, status string) {
	if !m.enabled {
		return
	}
	resourceSyncs.WithLabelValues(workspace, cluster, resourceType, status).Inc()
}

// SetWorkloadSyncs sets the active workload syncs gauge.
func (m *TMCMetrics) SetWorkloadSyncs(workspace, cluster string, count float64) {
	if !m.enabled {
		return
	}
	workloadSyncs.WithLabelValues(workspace, cluster).Set(count)
}

// RecordControllerReconciliation records controller reconciliation metrics.
func (m *TMCMetrics) RecordControllerReconciliation(workspace, controller, status string) {
	if !m.enabled {
		return
	}
	controllerReconciliations.WithLabelValues(workspace, controller, status).Inc()
}

// ObserveControllerReconciliationDuration records controller reconciliation duration.
func (m *TMCMetrics) ObserveControllerReconciliationDuration(workspace, controller string, duration float64) {
	if !m.enabled {
		return
	}
	controllerReconciliationDuration.WithLabelValues(workspace, controller).Observe(duration)
}

// Common metric status values
const (
	StatusSuccess = "success"
	StatusFailure = "failure"
	StatusPending = "pending"
	StatusRunning = "running"
	StatusReady   = "ready"
	StatusError   = "error"
)

// Common metric operation values  
const (
	OperationCreate = "create"
	OperationUpdate = "update"
	OperationDelete = "delete"
	OperationSync   = "sync"
	OperationList   = "list"
	OperationWatch  = "watch"
)