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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ClusterTarget represents a target cluster for workload synchronization.
// This interface abstracts cluster details for upstream communication.
type ClusterTarget interface {
	// GetName returns the cluster name
	GetName() string
	
	// GetNamespace returns the cluster namespace (if applicable)
	GetNamespace() string
	
	// GetLabels returns cluster labels
	GetLabels() map[string]string
	
	// GetAnnotations returns cluster annotations
	GetAnnotations() map[string]string
}

// WorkloadSyncer defines the interface for syncing workloads to clusters.
// This interface handles upstream communication with physical or virtual clusters.
type WorkloadSyncer interface {
	// SyncWorkload synchronizes a workload to the target cluster
	SyncWorkload(ctx context.Context, cluster ClusterTarget, workload runtime.Object) error

	// GetStatus retrieves the current status of a workload from the target cluster
	GetStatus(ctx context.Context, cluster ClusterTarget, workload runtime.Object) (*WorkloadStatus, error)

	// DeleteWorkload removes a workload from the target cluster
	DeleteWorkload(ctx context.Context, cluster ClusterTarget, workload runtime.Object) error

	// HealthCheck verifies connectivity and health of the target cluster
	HealthCheck(ctx context.Context, cluster ClusterTarget) error
}

// SyncEventHandler handles events generated during workload synchronization
type SyncEventHandler interface {
	// HandleEvent processes a synchronization event
	HandleEvent(ctx context.Context, event *SyncEvent) error
}

// WorkloadStatus represents the status of a workload in a cluster
type WorkloadStatus struct {
	// Ready indicates if the workload is ready and healthy
	Ready bool `json:"ready"`

	// Phase represents the current phase of the workload
	Phase WorkloadPhase `json:"phase"`

	// LastUpdated is when this status was last updated
	LastUpdated time.Time `json:"lastUpdated"`

	// ClusterName is the name of the cluster where this workload resides
	ClusterName string `json:"clusterName"`

	// Resources contains status of individual resources that make up this workload
	Resources []ResourceStatus `json:"resources,omitempty"`

	// Conditions provides detailed status conditions
	Conditions []WorkloadCondition `json:"conditions,omitempty"`
}

// ResourceStatus represents the status of a single resource
type ResourceStatus struct {
	// GVK is the GroupVersionKind of the resource
	GVK schema.GroupVersionKind `json:"gvk"`

	// Namespace is the namespace of the resource (if namespaced)
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the resource
	Name string `json:"name"`

	// Ready indicates if this resource is ready
	Ready bool `json:"ready"`

	// Phase represents the current phase of this resource
	Phase string `json:"phase,omitempty"`
}

// WorkloadCondition represents a condition of a workload
type WorkloadCondition struct {
	// Type is the type of the condition
	Type string `json:"type"`

	// Status is the status of the condition (True, False, Unknown)
	Status ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transitioned
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty"`

	// Reason is a unique, one-word, CamelCase reason for the condition's last transition
	Reason string `json:"reason,omitempty"`

	// Message is a human readable message indicating details about the transition
	Message string `json:"message,omitempty"`
}

// SyncEvent represents an event during workload synchronization
type SyncEvent struct {
	// Type is the type of sync event
	Type SyncEventType `json:"type"`

	// Cluster is the name of the target cluster
	Cluster string `json:"cluster"`

	// Workload is a reference to the workload being synced
	Workload WorkloadRef `json:"workload"`

	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Message provides additional context about the event
	Message string `json:"message,omitempty"`

	// Error contains error information if the event represents a failure
	Error error `json:"error,omitempty"`
}

// WorkloadRef provides a reference to a workload
type WorkloadRef struct {
	// GVK is the GroupVersionKind of the workload
	GVK schema.GroupVersionKind `json:"gvk"`

	// Namespace is the namespace of the workload (if namespaced)
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the workload
	Name string `json:"name"`
}

// WorkloadPhase represents the phase of a workload
type WorkloadPhase string

const (
	// WorkloadPhasePending indicates the workload is pending deployment
	WorkloadPhasePending WorkloadPhase = "Pending"

	// WorkloadPhaseDeploying indicates the workload is being deployed
	WorkloadPhaseDeploying WorkloadPhase = "Deploying"

	// WorkloadPhaseReady indicates the workload is ready and healthy
	WorkloadPhaseReady WorkloadPhase = "Ready"

	// WorkloadPhaseDegraded indicates the workload is degraded but functional
	WorkloadPhaseDegraded WorkloadPhase = "Degraded"

	// WorkloadPhaseFailed indicates the workload has failed
	WorkloadPhaseFailed WorkloadPhase = "Failed"

	// WorkloadPhaseTerminating indicates the workload is being terminated
	WorkloadPhaseTerminating WorkloadPhase = "Terminating"

	// WorkloadPhaseUnknown indicates the workload phase is unknown
	WorkloadPhaseUnknown WorkloadPhase = "Unknown"
)

// SyncEventType represents the type of synchronization event
type SyncEventType string

const (
	// SyncEventStarted indicates synchronization has started
	SyncEventStarted SyncEventType = "SyncStarted"

	// SyncEventCompleted indicates synchronization completed successfully
	SyncEventCompleted SyncEventType = "SyncCompleted"

	// SyncEventFailed indicates synchronization failed
	SyncEventFailed SyncEventType = "SyncFailed"

	// SyncEventSkipped indicates synchronization was skipped
	SyncEventSkipped SyncEventType = "SyncSkipped"
)

// ConditionStatus represents the status of a condition
type ConditionStatus string

const (
	// ConditionTrue means the condition is true
	ConditionTrue ConditionStatus = "True"

	// ConditionFalse means the condition is false
	ConditionFalse ConditionStatus = "False"

	// ConditionUnknown means the condition status is unknown
	ConditionUnknown ConditionStatus = "Unknown"
)

// SimpleClusterTarget provides a basic implementation of ClusterTarget
type SimpleClusterTarget struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GetName returns the cluster name
func (c *SimpleClusterTarget) GetName() string {
	return c.Name
}

// GetNamespace returns the cluster namespace
func (c *SimpleClusterTarget) GetNamespace() string {
	return c.Namespace
}

// GetLabels returns cluster labels
func (c *SimpleClusterTarget) GetLabels() map[string]string {
	if c.Labels == nil {
		return make(map[string]string)
	}
	return c.Labels
}

// GetAnnotations returns cluster annotations
func (c *SimpleClusterTarget) GetAnnotations() map[string]string {
	if c.Annotations == nil {
		return make(map[string]string)
	}
	return c.Annotations
}