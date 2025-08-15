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

package workspace

import (
	"context"
	"time"
)

// LifecycleManager handles workspace lifecycle operations including
// initialization, startup, shutdown, and cleanup processes.
// It coordinates with the workspace manager to ensure proper state transitions.
type LifecycleManager interface {
	// Initialize prepares a workspace for first use.
	// This includes setting up initial resources, configurations, and dependencies.
	Initialize(ctx context.Context, workspace *Workspace) error

	// Start activates a workspace and makes it available for use.
	// The workspace must be initialized before starting.
	Start(ctx context.Context, workspace *Workspace) error

	// Stop gracefully deactivates a workspace.
	// Active connections are drained and resources are preserved.
	Stop(ctx context.Context, workspace *Workspace) error

	// Restart performs a stop followed by a start operation.
	// This is useful for applying configuration changes.
	Restart(ctx context.Context, workspace *Workspace) error

	// Destroy permanently removes a workspace and all its resources.
	// This operation cannot be undone.
	Destroy(ctx context.Context, workspace *Workspace) error

	// Health performs a comprehensive health check on the workspace.
	// Returns detailed health status for all workspace components.
	Health(ctx context.Context, workspace *Workspace) (*HealthStatus, error)

	// SetPolicy updates the lifecycle policy for a workspace.
	// Policy changes take effect immediately for future operations.
	SetPolicy(ctx context.Context, workspace *Workspace, policy *LifecyclePolicy) error
}

// LifecycleHook provides extension points for custom logic during
// workspace lifecycle operations. Hooks are called synchronously
// and can abort operations by returning errors.
type LifecycleHook interface {
	// PreCreate is called before workspace creation begins.
	// Can be used to validate prerequisites or modify configuration.
	PreCreate(ctx context.Context, workspace *Workspace) error

	// PostCreate is called after workspace creation completes successfully.
	// Used for post-creation setup like creating default resources.
	PostCreate(ctx context.Context, workspace *Workspace) error

	// PreStart is called before workspace startup begins.
	// Can perform pre-startup validation or resource preparation.
	PreStart(ctx context.Context, workspace *Workspace) error

	// PostStart is called after workspace startup completes successfully.
	// Used for post-startup operations like health checks.
	PostStart(ctx context.Context, workspace *Workspace) error

	// PreStop is called before workspace shutdown begins.
	// Can perform graceful shutdown preparation.
	PreStop(ctx context.Context, workspace *Workspace) error

	// PostStop is called after workspace shutdown completes successfully.
	// Used for post-shutdown cleanup operations.
	PostStop(ctx context.Context, workspace *Workspace) error

	// PreDelete is called before workspace deletion begins.
	// Final chance to backup data or perform cleanup.
	PreDelete(ctx context.Context, workspace *Workspace) error

	// PostDelete is called after workspace deletion completes successfully.
	// Used for final cleanup operations.
	PostDelete(ctx context.Context, workspace *Workspace) error

	// OnError is called when any lifecycle operation encounters an error.
	// Can be used for error logging, alerting, or recovery actions.
	OnError(ctx context.Context, workspace *Workspace, operation string, err error) error
}

// HealthStatus represents the comprehensive health state of a workspace.
// It includes overall health and detailed status for individual components.
type HealthStatus struct {
	// Healthy indicates the overall health of the workspace
	Healthy bool

	// Components contains health status for individual workspace components
	Components []ComponentHealth

	// LastCheck records when this health status was determined
	LastCheck time.Time

	// Message provides human-readable health summary
	Message string

	// Score provides a numeric health score (0-100)
	Score int

	// Dependencies tracks the health of external dependencies
	Dependencies []DependencyHealth
}

// ComponentHealth represents the health status of a specific workspace component.
type ComponentHealth struct {
	// Name identifies the component (e.g., "api-server", "etcd", "cache")
	Name string

	// Healthy indicates if this component is functioning correctly
	Healthy bool

	// Message provides detailed component status information
	Message string

	// LastCheck records when this component was last checked
	LastCheck time.Time

	// ResponseTime tracks component response performance
	ResponseTime time.Duration

	// Errors contains recent error information
	Errors []string
}

// DependencyHealth tracks the health of external dependencies.
type DependencyHealth struct {
	// Name identifies the dependency (e.g., "kcp-server", "etcd-cluster")
	Name string

	// Available indicates if the dependency is accessible
	Available bool

	// Message provides dependency status details
	Message string

	// LastCheck records when this dependency was last checked
	LastCheck time.Time
}

// LifecyclePolicy defines the operational behavior and constraints
// for workspace lifecycle management.
type LifecyclePolicy struct {
	// AutoStart determines if the workspace should start automatically
	// when dependencies become available
	AutoStart bool

	// RestartPolicy defines when and how the workspace should restart
	RestartPolicy RestartPolicy

	// HealthCheckInterval specifies how often to perform health checks
	HealthCheckInterval time.Duration

	// StartupTimeout sets the maximum time allowed for workspace startup
	StartupTimeout time.Duration

	// ShutdownTimeout sets the maximum time allowed for graceful shutdown
	ShutdownTimeout time.Duration

	// MaxRetries limits the number of automatic restart attempts
	MaxRetries int

	// RetryBackoff defines the delay between restart attempts
	RetryBackoff time.Duration

	// HealthCheckRetries limits health check retry attempts
	HealthCheckRetries int

	// GracePeriod allows time for graceful shutdown before force termination
	GracePeriod time.Duration
}

// RestartPolicy defines when a workspace should be automatically restarted.
type RestartPolicy string

const (
	// RestartPolicyAlways indicates the workspace should always restart on exit
	RestartPolicyAlways RestartPolicy = "Always"

	// RestartPolicyOnFailure indicates restart only on failure (non-zero exit)
	RestartPolicyOnFailure RestartPolicy = "OnFailure"

	// RestartPolicyNever indicates the workspace should never restart automatically
	RestartPolicyNever RestartPolicy = "Never"

	// RestartPolicyUnlessManualStop restarts unless manually stopped
	RestartPolicyUnlessManualStop RestartPolicy = "UnlessManualStop"
)

// LifecycleEvent represents a lifecycle-related event.
type LifecycleEvent struct {
	// Type specifies the lifecycle event type
	Type LifecycleEventType

	// Workspace is the workspace affected by the event
	Workspace *Workspace

	// Operation describes the lifecycle operation being performed
	Operation string

	// Message provides event details
	Message string

	// Error contains error information for failed operations
	Error error

	// Timestamp records when the event occurred
	Timestamp time.Time

	// Duration tracks how long the operation took
	Duration time.Duration
}

// LifecycleEventType represents different types of lifecycle events.
type LifecycleEventType string

const (
	// LifecycleEventStarted indicates an operation started
	LifecycleEventStarted LifecycleEventType = "Started"

	// LifecycleEventCompleted indicates an operation completed successfully
	LifecycleEventCompleted LifecycleEventType = "Completed"

	// LifecycleEventFailed indicates an operation failed
	LifecycleEventFailed LifecycleEventType = "Failed"

	// LifecycleEventTimeout indicates an operation timed out
	LifecycleEventTimeout LifecycleEventType = "Timeout"

	// LifecycleEventRetrying indicates an operation is being retried
	LifecycleEventRetrying LifecycleEventType = "Retrying"
)

// LifecycleEventHandler processes lifecycle events for monitoring and logging.
type LifecycleEventHandler interface {
	// HandleEvent processes a lifecycle event
	HandleEvent(ctx context.Context, event *LifecycleEvent) error
}