/*
Copyright 2023 The KCP Authors.

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

package scheduler

import (
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/kcp-dev/logicalcluster/v3"
)

// Priority defines placement request priorities.
type Priority int

const (
	// PriorityLow for low priority placements
	PriorityLow Priority = 1
	// PriorityNormal for normal priority placements
	PriorityNormal Priority = 50
	// PriorityHigh for high priority placements
	PriorityHigh Priority = 100
	// PriorityCritical for critical priority placements
	PriorityCritical Priority = 1000
)

// PlacementRequest represents a request to schedule a workload across workspaces.
type PlacementRequest struct {
	// Name is a unique identifier for this placement request
	Name string
	
	// Namespace is the namespace for this placement request
	Namespace string
	
	// Workspace is the source workspace for this placement request
	Workspace logicalcluster.Name
	
	// Priority determines the scheduling priority (higher values = higher priority)
	Priority Priority
	
	// ResourceRequirements specifies the resources required for this placement
	ResourceRequirements ResourceRequirements
	
	// MaxPlacements limits the number of workspaces for placement (0 = no limit)
	MaxPlacements int
	
	// User is the user context for authorization checks
	User user.Info
	
	// CreatedAt is when this placement request was created
	CreatedAt time.Time
}

// SchedulingDecision represents the result of placement scheduling.
type SchedulingDecision struct {
	// PlacementRequest is the original request
	PlacementRequest *PlacementRequest
	
	// SelectedWorkspaces are the workspaces chosen for placement
	SelectedWorkspaces []*WorkspacePlacement
	
	// SchedulingTime is when this decision was made
	SchedulingTime time.Time
	
	// SchedulingDuration is how long scheduling took
	SchedulingDuration time.Duration
	
	// ReasonForDecision explains why this decision was made
	ReasonForDecision string
	
	// Error contains any error that occurred during scheduling
	Error error
}

// WorkspacePlacement represents a workspace selected for placement.
type WorkspacePlacement struct {
	// Workspace is the selected workspace
	Workspace logicalcluster.Name
	
	// Score is the scheduling score for this workspace
	Score float64
	
	// AllocatedResources are the resources reserved in this workspace
	AllocatedResources ResourceAllocation
	
	// Reason explains why this workspace was selected
	Reason string
}

// WorkspaceCandidate represents a potential workspace for placement.
type WorkspaceCandidate struct {
	// Workspace is the logical cluster name
	Workspace logicalcluster.Name
	
	// AvailableCapacity represents the available resources
	AvailableCapacity ResourceCapacity
	
	// CurrentLoad represents the current resource utilization
	CurrentLoad ResourceUtilization
	
	// Labels are the workspace labels for affinity matching
	Labels labels.Set
	
	// Ready indicates if the workspace is ready to accept placements
	Ready bool
	
	// LastHeartbeat is when this workspace last reported in
	LastHeartbeat time.Time
}

// ScoredCandidate represents a workspace candidate with its scheduling score.
type ScoredCandidate struct {
	// Candidate is the workspace candidate
	Candidate *WorkspaceCandidate
	
	// Score is the overall scheduling score (0-100, higher is better)
	Score float64
}

// ResourceRequirements specifies the resource requirements for a placement.
type ResourceRequirements struct {
	// CPU is the CPU requirement
	CPU resource.Quantity
	
	// Memory is the memory requirement
	Memory resource.Quantity
	
	// Storage is the storage requirement
	Storage resource.Quantity
	
	// CustomResources contains requirements for custom resources
	CustomResources map[string]resource.Quantity
}

// ResourceCapacity represents the resource capacity of a workspace.
type ResourceCapacity struct {
	// CPU is the CPU capacity
	CPU resource.Quantity
	
	// Memory is the memory capacity
	Memory resource.Quantity
	
	// Storage is the storage capacity
	Storage resource.Quantity
	
	// CustomResources contains capacity for custom resources
	CustomResources map[string]resource.Quantity
	
	// LastUpdated is when this capacity was last updated
	LastUpdated time.Time
}

// ResourceUtilization represents the current resource utilization of a workspace.
type ResourceUtilization struct {
	// CPU is the current CPU usage
	CPU resource.Quantity
	
	// Memory is the current memory usage
	Memory resource.Quantity
	
	// Storage is the current storage usage
	Storage resource.Quantity
	
	// CustomResources contains usage for custom resources
	CustomResources map[string]resource.Quantity
}

// ResourceAllocation represents resources allocated to a placement.
type ResourceAllocation struct {
	// CPU is the allocated CPU
	CPU resource.Quantity
	
	// Memory is the allocated memory
	Memory resource.Quantity
	
	// Storage is the allocated storage
	Storage resource.Quantity
	
	// CustomResources contains allocated custom resources
	CustomResources map[string]resource.Quantity
	
	// ReservationID is the unique identifier for this reservation
	ReservationID string
	
	// ExpiresAt is when this allocation expires if not confirmed
	ExpiresAt time.Time
}

// CapacityTracker tracks resource capacity and utilization across workspaces.
type CapacityTracker interface {
	// GetCapacity returns the current capacity for a workspace
	GetCapacity(workspace logicalcluster.Name) (*ResourceCapacity, error)
	
	// GetAvailableCapacity returns available capacity (total - utilized - reserved)
	GetAvailableCapacity(workspace logicalcluster.Name) (*ResourceCapacity, error)
}