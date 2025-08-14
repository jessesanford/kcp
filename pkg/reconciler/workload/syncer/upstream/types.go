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
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
)

// StatusAggregator combines status from multiple clusters
type StatusAggregator interface {
	AggregateStatus(ctx context.Context, resources []ResourceStatus) (*AggregatedStatus, error)
	ResolveConflicts(ctx context.Context, conflicts []Conflict) (*Resolution, error)
	SetStrategy(strategy workloadv1alpha1.ConflictStrategy)
	GetLastAggregation() *AggregatedStatus
}

// ConflictResolver resolves status conflicts
type ConflictResolver interface {
	Resolve(ctx context.Context, conflict Conflict) (*Resolution, error)
	SetStrategy(strategy workloadv1alpha1.ConflictStrategy)
	CanAutoResolve(conflict Conflict) bool
}

// UpdateApplier applies updates to KCP
type UpdateApplier interface {
	Apply(ctx context.Context, update *Update) error
	ApplyBatch(ctx context.Context, updates []*Update) error
	SetDryRun(enabled bool)
	GetAppliedCount() int64
}

// ResourceStatus represents status from a single cluster
type ResourceStatus struct {
	ClusterName   string
	Resource      *unstructured.Unstructured
	LastUpdated   time.Time
	Health        HealthStatus
}

// AggregatedStatus represents combined status
type AggregatedStatus struct {
	ResourceKey       string
	CombinedStatus    *unstructured.Unstructured
	SourceStatuses    []ResourceStatus
	AggregationTime   time.Time
	ConflictsResolved int
}

// Conflict represents a status conflict
type Conflict struct {
	ResourceKey string
	Statuses    []ResourceStatus
	Type        ConflictType
	Severity    ConflictSeverity
}

// Resolution represents a conflict resolution
type Resolution struct {
	Conflict       Conflict
	ResolvedStatus *ResourceStatus
	Strategy       string
	Timestamp      time.Time
}

// Update represents a change to apply to KCP
type Update struct {
	Type     UpdateType
	Resource *unstructured.Unstructured
	Strategy ApplyStrategy
}

// HealthStatus represents resource health
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "Healthy"
	HealthStatusDegraded  HealthStatus = "Degraded"
	HealthStatusUnhealthy HealthStatus = "Unhealthy"
	HealthStatusUnknown   HealthStatus = "Unknown"
)

// ConflictType represents the type of conflict
type ConflictType string

const (
	ConflictTypeGeneration ConflictType = "Generation"
	ConflictTypeStatus     ConflictType = "Status"
	ConflictTypeMetadata   ConflictType = "Metadata"
)

// ConflictSeverity represents conflict severity
type ConflictSeverity string

const (
	ConflictSeverityLow    ConflictSeverity = "Low"
	ConflictSeverityMedium ConflictSeverity = "Medium"
	ConflictSeverityHigh   ConflictSeverity = "High"
)

// UpdateType represents the type of update
type UpdateType string

const (
	UpdateTypeCreate UpdateType = "Create"
	UpdateTypeUpdate UpdateType = "Update"
	UpdateTypeDelete UpdateType = "Delete"
	UpdateTypeStatus UpdateType = "Status"
)

// ApplyStrategy represents how to apply updates
type ApplyStrategy string

const (
	ApplyStrategyClientSide ApplyStrategy = "ClientSide"
	ApplyStrategyServerSide ApplyStrategy = "ServerSide"
)