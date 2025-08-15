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

// Package workspace provides abstractions for virtual workspace management
// within the KCP ecosystem. It defines core interfaces and types for workspace
// lifecycle management, caching, and monitoring.
package workspace

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Manager defines the interface for managing virtual workspace lifecycle.
// It provides operations to create, update, delete, and monitor virtual workspaces
// following KCP architectural patterns.
type Manager interface {
	// Create creates a new virtual workspace based on the provided configuration.
	// The workspace will be initialized but not necessarily ready for use.
	Create(ctx context.Context, config *VirtualWorkspaceConfig) (*Workspace, error)

	// Update modifies an existing virtual workspace configuration.
	// Changes will be applied asynchronously and may trigger workspace restart.
	Update(ctx context.Context, config *VirtualWorkspaceConfig) (*Workspace, error)

	// Delete removes a virtual workspace and cleans up associated resources.
	// The operation is asynchronous and may take time to complete.
	Delete(ctx context.Context, name string) error

	// Get retrieves a specific workspace by name.
	// Returns error if workspace doesn't exist or is inaccessible.
	Get(ctx context.Context, name string) (*Workspace, error)

	// List returns all managed workspaces with optional filtering.
	// Results are sorted by creation time, newest first.
	List(ctx context.Context, opts ListOptions) ([]*Workspace, error)

	// Watch monitors workspace changes and returns an event channel.
	// The channel will be closed when the context is cancelled.
	Watch(ctx context.Context) (<-chan WorkspaceEvent, error)

	// GetStatus returns the current status of a workspace.
	// Includes readiness, health, and resource usage information.
	GetStatus(ctx context.Context, name string) (*WorkspaceStatus, error)
}

// Workspace represents a managed virtual workspace instance.
// It encapsulates the workspace configuration, current state, and metadata.
type Workspace struct {
	// Name is the unique identifier for the workspace
	Name string

	// UID provides a unique identifier across the cluster
	UID types.UID

	// Config contains the workspace configuration
	Config *VirtualWorkspaceConfig

	// State represents the current operational state
	State WorkspaceState

	// Resources lists the available resources in this workspace
	Resources []ResourceInfo

	// Metadata contains additional workspace information
	Metadata WorkspaceMetadata

	// Conditions reflect the current condition of the workspace
	Conditions []metav1.Condition
}

// VirtualWorkspaceConfig defines the configuration for a virtual workspace.
// This struct will be replaced with the actual KCP virtual workspace type
// once vw-02 branch provides the core types.
type VirtualWorkspaceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the virtual workspace
	Spec VirtualWorkspaceSpec `json:"spec,omitempty"`

	// Status reflects the observed state of the virtual workspace  
	Status VirtualWorkspaceStatus `json:"status,omitempty"`
}

// VirtualWorkspaceSpec defines the desired state of a virtual workspace.
type VirtualWorkspaceSpec struct {
	// Description provides human-readable workspace description
	Description string `json:"description,omitempty"`

	// Replicas specifies the desired number of workspace replicas
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources defines resource limits and requests
	Resources ResourceRequirements `json:"resources,omitempty"`

	// ReadyReplicas indicates minimum ready replicas for workspace availability
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// VirtualWorkspaceStatus defines the observed state of a virtual workspace.
type VirtualWorkspaceStatus struct {
	// Phase indicates the current phase of the workspace
	Phase VirtualWorkspacePhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// VirtualWorkspacePhase represents the phase of a virtual workspace.
type VirtualWorkspacePhase string

const (
	// VirtualWorkspacePhasePending indicates the workspace is being created
	VirtualWorkspacePhasePending VirtualWorkspacePhase = "Pending"
	
	// VirtualWorkspacePhaseInitializing indicates the workspace is initializing
	VirtualWorkspacePhaseInitializing VirtualWorkspacePhase = "Initializing"
	
	// VirtualWorkspacePhaseReady indicates the workspace is ready for use
	VirtualWorkspacePhaseReady VirtualWorkspacePhase = "Ready"
	
	// VirtualWorkspacePhaseTerminating indicates the workspace is being deleted
	VirtualWorkspacePhaseTerminating VirtualWorkspacePhase = "Terminating"
	
	// VirtualWorkspacePhaseFailed indicates the workspace has failed
	VirtualWorkspacePhaseFailed VirtualWorkspacePhase = "Failed"
)

// WorkspaceState encapsulates the current state of a workspace.
type WorkspaceState struct {
	// Phase indicates the current lifecycle phase
	Phase VirtualWorkspacePhase

	// Ready indicates if the workspace is ready for use
	Ready bool

	// Message provides human-readable status information
	Message string

	// LastTransition records when the state last changed
	LastTransition time.Time

	// Reason provides a brief reason for the current state
	Reason string
}

// ResourceInfo describes an available resource in the workspace.
type ResourceInfo struct {
	// GroupVersionResource identifies the resource type
	GroupVersionResource schema.GroupVersionResource

	// Namespaced indicates if the resource is namespaced
	Namespaced bool

	// Verbs lists the supported operations on this resource
	Verbs []string

	// ShortNames provides aliases for the resource
	ShortNames []string

	// Categories groups related resources
	Categories []string
}

// WorkspaceMetadata contains additional workspace information.
type WorkspaceMetadata struct {
	// CreatedAt records when the workspace was created
	CreatedAt time.Time

	// UpdatedAt records the last update time
	UpdatedAt time.Time

	// Labels are key-value pairs attached to the workspace
	Labels map[string]string

	// Annotations are key-value pairs for additional metadata
	Annotations map[string]string

	// Generation reflects the current generation of the workspace
	Generation int64
}

// WorkspaceEvent represents a change in workspace state.
type WorkspaceEvent struct {
	// Type specifies the kind of event
	Type EventType

	// Workspace is the workspace affected by the event
	Workspace *Workspace

	// OldWorkspace provides the previous state for update events
	OldWorkspace *Workspace

	// Error contains error information if the event represents a failure
	Error error

	// Timestamp records when the event occurred
	Timestamp time.Time
}

// EventType represents the type of workspace event.
type EventType string

const (
	// EventTypeCreated indicates a workspace was created
	EventTypeCreated EventType = "Created"
	
	// EventTypeUpdated indicates a workspace was updated
	EventTypeUpdated EventType = "Updated"
	
	// EventTypeDeleted indicates a workspace was deleted
	EventTypeDeleted EventType = "Deleted"
	
	// EventTypeError indicates an error occurred
	EventTypeError EventType = "Error"
	
	// EventTypeReady indicates a workspace became ready
	EventTypeReady EventType = "Ready"
	
	// EventTypeNotReady indicates a workspace became not ready
	EventTypeNotReady EventType = "NotReady"
)

// ListOptions provides filtering and pagination for workspace listing.
type ListOptions struct {
	// LabelSelector filters workspaces by labels
	LabelSelector string

	// FieldSelector filters workspaces by fields
	FieldSelector string

	// Limit restricts the number of results
	Limit int

	// Continue token for pagination
	Continue string
}

// WorkspaceStatus represents the comprehensive status of a workspace.
type WorkspaceStatus struct {
	// Phase indicates the current workspace phase
	Phase VirtualWorkspacePhase

	// Ready indicates overall workspace readiness
	Ready bool

	// Conditions reflect current workspace conditions
	Conditions []metav1.Condition

	// ResourceUsage shows current resource consumption
	ResourceUsage ResourceUsage

	// LastUpdate records when status was last updated
	LastUpdate time.Time
}

// ResourceRequirements specify resource limits and requests.
type ResourceRequirements struct {
	// Limits specify maximum resource consumption
	Limits map[string]string

	// Requests specify minimum resource requirements
	Requests map[string]string
}

// ResourceUsage tracks current resource consumption.
type ResourceUsage struct {
	// CPU usage in cores
	CPU string

	// Memory usage in bytes
	Memory string

	// Storage usage in bytes
	Storage string

	// RequestCount tracks API request volume
	RequestCount int64
}