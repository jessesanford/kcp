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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/logicalcluster/v3"
)

// ClusterRegistration represents a registered physical cluster for workload placement
// This is a simplified version for the syncer implementation.
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// ClusterRegistrationSpec defines the desired state of a cluster registration
type ClusterRegistrationSpec struct {
	// Location describes the location of the cluster
	Location string `json:"location,omitempty"`

	// Provider describes the cloud provider of the cluster
	Provider string `json:"provider,omitempty"`
}

// ClusterRegistrationStatus defines the observed state of a cluster registration
type ClusterRegistrationStatus struct {
	// Ready indicates if the cluster is ready to accept workloads
	Ready bool `json:"ready"`

	// Conditions contains the status conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// WorkloadSyncer defines the interface for syncing workloads to physical clusters
// with workspace isolation and logical cluster awareness.
type WorkloadSyncer interface {
	// SyncWorkload synchronizes a workload to the target cluster within the specified workspace context
	SyncWorkload(ctx context.Context, cluster *ClusterRegistration, workload runtime.Object) error

	// GetStatus retrieves the current status of a workload from the target cluster
	GetStatus(ctx context.Context, cluster *ClusterRegistration, workload runtime.Object) (*WorkloadStatus, error)

	// DeleteWorkload removes a workload from the target cluster
	DeleteWorkload(ctx context.Context, cluster *ClusterRegistration, workload runtime.Object) error

	// HealthCheck verifies the syncer can communicate with the target cluster
	HealthCheck(ctx context.Context, cluster *ClusterRegistration) error
}

// WorkloadStatus represents the current state of a workload in a physical cluster
type WorkloadStatus struct {
	// Ready indicates if the workload is ready and functioning
	Ready bool `json:"ready"`

	// Phase represents the current lifecycle phase
	Phase WorkloadPhase `json:"phase"`

	// LastUpdated is the timestamp of the last status update
	LastUpdated time.Time `json:"lastUpdated"`

	// ClusterName identifies the cluster where this status was observed
	ClusterName string `json:"clusterName"`

	// LogicalCluster identifies the workspace context for this workload
	LogicalCluster logicalcluster.Name `json:"logicalCluster"`

	// Resources contains status for individual Kubernetes resources
	Resources []ResourceStatus `json:"resources,omitempty"`

	// Conditions contains detailed status conditions
	Conditions []WorkloadCondition `json:"conditions,omitempty"`

	// Message provides human-readable status information
	Message string `json:"message,omitempty"`
}

// WorkloadPhase represents the lifecycle phase of a workload
type WorkloadPhase string

const (
	// WorkloadPhasePending indicates the workload is waiting to be deployed
	WorkloadPhasePending WorkloadPhase = "Pending"

	// WorkloadPhaseDeploying indicates the workload is being deployed
	WorkloadPhaseDeploying WorkloadPhase = "Deploying"

	// WorkloadPhaseReady indicates the workload is deployed and ready
	WorkloadPhaseReady WorkloadPhase = "Ready"

	// WorkloadPhaseDegraded indicates the workload is deployed but degraded
	WorkloadPhaseDegraded WorkloadPhase = "Degraded"

	// WorkloadPhaseFailed indicates the workload deployment has failed
	WorkloadPhaseFailed WorkloadPhase = "Failed"

	// WorkloadPhaseTerminating indicates the workload is being terminated
	WorkloadPhaseTerminating WorkloadPhase = "Terminating"

	// WorkloadPhaseUnknown indicates the workload status is unknown
	WorkloadPhaseUnknown WorkloadPhase = "Unknown"
)

// ResourceStatus represents the status of an individual Kubernetes resource
type ResourceStatus struct {
	// GVK identifies the resource type
	GVK schema.GroupVersionKind `json:"gvk"`

	// Namespace of the resource (empty for cluster-scoped resources)
	Namespace string `json:"namespace,omitempty"`

	// Name of the resource
	Name string `json:"name"`

	// Ready indicates if this resource is ready
	Ready bool `json:"ready"`

	// Phase represents the resource phase
	Phase string `json:"phase,omitempty"`

	// Message provides resource-specific status information
	Message string `json:"message,omitempty"`

	// WorkspacePrefix contains the workspace-aware resource naming prefix
	WorkspacePrefix string `json:"workspacePrefix,omitempty"`
}

// WorkloadCondition represents a condition of a workload
type WorkloadCondition struct {
	// Type is the type of condition
	Type string `json:"type"`

	// Status is the status of the condition (True, False, Unknown)
	Status ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition changed
	LastTransitionTime time.Time `json:"lastTransitionTime"`

	// Reason is a brief reason for the condition's status
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable explanation of the condition
	Message string `json:"message,omitempty"`
}

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

// WorkloadRef provides a reference to a specific workload resource
type WorkloadRef struct {
	// GVK identifies the resource type
	GVK schema.GroupVersionKind `json:"gvk"`

	// Namespace of the resource (empty for cluster-scoped resources)
	Namespace string `json:"namespace,omitempty"`

	// Name of the resource
	Name string `json:"name"`

	// LogicalCluster provides workspace context for the workload
	LogicalCluster logicalcluster.Name `json:"logicalCluster"`

	// WorkspaceQualifiedName is the workspace-aware name used in the physical cluster
	WorkspaceQualifiedName string `json:"workspaceQualifiedName,omitempty"`
}

// SyncEvent represents an event that occurs during workload synchronization
type SyncEvent struct {
	// Type identifies the kind of sync event
	Type SyncEventType `json:"type"`

	// Cluster is the name of the target cluster
	Cluster string `json:"cluster"`

	// Workload references the workload being synchronized
	Workload WorkloadRef `json:"workload"`

	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Message provides human-readable event information
	Message string `json:"message"`

	// Error contains error details if the event represents a failure
	Error error `json:"error,omitempty"`

	// LogicalCluster provides workspace context for the event
	LogicalCluster logicalcluster.Name `json:"logicalCluster"`
}

// SyncEventType represents the type of synchronization event
type SyncEventType string

const (
	// SyncEventStarted indicates synchronization has begun
	SyncEventStarted SyncEventType = "SyncStarted"

	// SyncEventCompleted indicates synchronization completed successfully
	SyncEventCompleted SyncEventType = "SyncCompleted"

	// SyncEventFailed indicates synchronization failed
	SyncEventFailed SyncEventType = "SyncFailed"

	// SyncEventSkipped indicates synchronization was skipped
	SyncEventSkipped SyncEventType = "SyncSkipped"
)

// SyncEventHandler defines the interface for handling synchronization events
// with workspace-aware event processing.
type SyncEventHandler interface {
	// HandleEvent processes a synchronization event within the appropriate workspace context
	HandleEvent(ctx context.Context, event *SyncEvent) error
}

// WorkspaceAwareNaming provides utilities for workspace-scoped resource naming
// to ensure proper isolation between workspaces in physical clusters.
type WorkspaceAwareNaming struct {
	// LogicalCluster identifies the workspace context
	LogicalCluster logicalcluster.Name

	// Separator used in workspace-qualified names
	Separator string
}

// NewWorkspaceAwareNaming creates a new workspace naming utility
func NewWorkspaceAwareNaming(logicalCluster logicalcluster.Name) *WorkspaceAwareNaming {
	return &WorkspaceAwareNaming{
		LogicalCluster: logicalCluster,
		Separator:      "--",
	}
}

// QualifyName creates a workspace-qualified name for a resource to ensure
// uniqueness and prevent cross-workspace conflicts in physical clusters.
func (wan *WorkspaceAwareNaming) QualifyName(name string) string {
	if wan.LogicalCluster.Empty() {
		return name
	}

	// Create workspace-qualified name: workspace--original-name
	workspaceStr := string(wan.LogicalCluster)
	// Replace any invalid characters for Kubernetes names
	workspaceStr = sanitizeForKubernetes(workspaceStr)
	
	return workspaceStr + wan.Separator + name
}

// ExtractOriginalName extracts the original resource name from a workspace-qualified name
func (wan *WorkspaceAwareNaming) ExtractOriginalName(qualifiedName string) string {
	if wan.LogicalCluster.Empty() {
		return qualifiedName
	}

	workspaceStr := sanitizeForKubernetes(string(wan.LogicalCluster))
	prefix := workspaceStr + wan.Separator

	if len(qualifiedName) > len(prefix) && qualifiedName[:len(prefix)] == prefix {
		return qualifiedName[len(prefix):]
	}

	return qualifiedName
}

// IsWorkspaceResource checks if a resource belongs to this workspace based on its name
func (wan *WorkspaceAwareNaming) IsWorkspaceResource(name string) bool {
	if wan.LogicalCluster.Empty() {
		return true
	}

	workspaceStr := sanitizeForKubernetes(string(wan.LogicalCluster))
	prefix := workspaceStr + wan.Separator

	return len(name) > len(prefix) && name[:len(prefix)] == prefix
}

// sanitizeForKubernetes ensures a string is valid for use in Kubernetes resource names
func sanitizeForKubernetes(s string) string {
	// Replace invalid characters with hyphens
	result := ""
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result += string(r)
		} else if r >= 'A' && r <= 'Z' {
			result += string(r - 'A' + 'a') // Convert to lowercase
		} else if r == ':' || r == '/' {
			result += "-"
		}
	}
	return result
}

// RetryStrategy defines how to retry failed operations
type RetryStrategy struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// InitialDelay is the initial delay between retries
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// BackoffFactor is the factor by which delays are multiplied
	BackoffFactor float64
}

// DefaultRetryStrategy returns a sensible default retry strategy
func DefaultRetryStrategy() *RetryStrategy {
	return &RetryStrategy{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	}
}

// ExecuteWithRetry executes an operation with retries according to the strategy
func ExecuteWithRetry(operation func() error, strategy *RetryStrategy) error {
	if strategy == nil {
		return operation()
	}

	var lastErr error
	delay := strategy.InitialDelay

	for attempt := 0; attempt <= strategy.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * strategy.BackoffFactor)
			if delay > strategy.MaxDelay {
				delay = strategy.MaxDelay
			}
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return lastErr
}