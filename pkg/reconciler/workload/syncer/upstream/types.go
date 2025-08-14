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

package upstream

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/logicalcluster/v3"
)

// EventType represents the type of resource event
type EventType string

const (
	EventTypeCreate EventType = "Create"
	EventTypeUpdate EventType = "Update"
	EventTypeDelete EventType = "Delete"
)

// Event represents a resource change event from a physical cluster
type Event struct {
	Type        EventType
	Resource    *unstructured.Unstructured
	OldResource *unstructured.Unstructured // For updates
	Timestamp   time.Time
	Source      ClusterSource
}

// ClusterSource identifies the source cluster
type ClusterSource struct {
	Name      string
	Workspace logicalcluster.Name
	Region    string
}

// ResourceStatus represents resource status from a single cluster
type ResourceStatus struct {
	ClusterName string
	Resource    *unstructured.Unstructured
	Conditions  []metav1.Condition
	LastUpdated time.Time
	Health      HealthStatus
}

// HealthStatus represents resource health
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "Healthy"
	HealthStatusDegraded  HealthStatus = "Degraded"
	HealthStatusUnhealthy HealthStatus = "Unhealthy"
	HealthStatusUnknown   HealthStatus = "Unknown"
)

// Conflict represents a status conflict between clusters
type Conflict struct {
	ResourceKey string
	Statuses    []ResourceStatus
	Type        ConflictType
	Severity    ConflictSeverity
}

// ConflictType categorizes the conflict
type ConflictType string

const (
	ConflictTypeStatus     ConflictType = "Status"
	ConflictTypeGeneration ConflictType = "Generation"
	ConflictTypeContent    ConflictType = "Content"
)

// ConflictSeverity indicates conflict severity
type ConflictSeverity string

const (
	ConflictSeverityLow    ConflictSeverity = "Low"
	ConflictSeverityMedium ConflictSeverity = "Medium"
	ConflictSeverityHigh   ConflictSeverity = "High"
)

// AggregatedStatus represents combined status from all clusters
type AggregatedStatus struct {
	ResourceKey       string
	CombinedStatus    *unstructured.Unstructured
	SourceStatuses    []ResourceStatus
	AggregationTime   time.Time
	ConflictsResolved int
}

// Resolution represents a conflict resolution
type Resolution struct {
	Conflict       Conflict
	ResolvedStatus *ResourceStatus
	Strategy       string
	Timestamp      time.Time
}

// Update represents an update to apply to KCP
type Update struct {
	Type      UpdateType
	Resource  *unstructured.Unstructured
	Workspace logicalcluster.Name
	Strategy  ApplyStrategy
}

// UpdateType categorizes the update
type UpdateType string

const (
	UpdateTypeCreate UpdateType = "Create"
	UpdateTypeUpdate UpdateType = "Update"
	UpdateTypeDelete UpdateType = "Delete"
	UpdateTypeStatus UpdateType = "Status"
)

// ApplyStrategy defines how updates are applied
type ApplyStrategy string

const (
	ApplyStrategyReplace    ApplyStrategy = "Replace"
	ApplyStrategyMerge      ApplyStrategy = "Merge"
	ApplyStrategyServerSide ApplyStrategy = "ServerSide"
)

// Metrics tracks synchronization metrics
type Metrics struct {
	SyncTargetsActive int
	ResourcesSynced   int64
	EventsProcessed   int64
	ConflictsResolved int64
	ErrorCount        int64
	LastSyncTime      time.Time
	SyncLatency       time.Duration
	QueueDepth        int
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
	Hits       int64
	Misses     int64
	Evictions  int64
	Size       int
	ErrorCount int64
}