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

package controller

import (
	"context"
	"time"
)

// Lifecycle defines hooks for controller startup and shutdown phases.
// Controllers can implement this interface to perform initialization
// and cleanup operations beyond the basic Start/Stop cycle.
type Lifecycle interface {
	// PreStart is called before the controller's main Start method.
	// This is the place to perform initialization that must complete
	// before the controller begins processing work items.
	// Examples: cache warming, resource validation, dependency checks.
	PreStart(ctx context.Context) error

	// PostStart is called after the controller's main Start method.
	// This runs concurrently with the controller's main loop and can
	// be used for background tasks that don't block controller startup.
	// Examples: periodic health checks, metrics collection setup.
	PostStart(ctx context.Context) error

	// PreStop is called before the controller's main Stop method.
	// This should prepare the controller for graceful shutdown.
	// Examples: stop accepting new work, drain work queues.
	PreStop(ctx context.Context) error

	// PostStop is called after the controller's main Stop method.
	// This is for final cleanup after the controller has stopped.
	// Examples: resource cleanup, connection closing, metric cleanup.
	PostStop(ctx context.Context) error
}

// HealthChecker provides health and readiness checking for controllers.
// This enables proper integration with Kubernetes health check systems
// and allows for more sophisticated failure detection.
type HealthChecker interface {
	// Check performs a comprehensive health check of the controller.
	// This should verify that all critical dependencies are available
	// and the controller is functioning properly.
	// Returns an error if the controller is unhealthy.
	Check(ctx context.Context) error

	// Ready returns true if the controller is ready to serve requests.
	// This is typically used for readiness probes and should be fast.
	// A controller may be healthy but not yet ready (e.g., during startup).
	Ready() bool

	// Live returns true if the controller is alive and not deadlocked.
	// This is typically used for liveness probes and should be very fast.
	// A controller should only return false if it needs to be restarted.
	Live() bool

	// GetHealthStatus returns a detailed health status with diagnostic information.
	// This provides more detailed information than the boolean checks above.
	GetHealthStatus() HealthStatus
}

// HealthStatus provides detailed health information about a controller.
type HealthStatus interface {
	// IsHealthy returns true if the controller is healthy.
	IsHealthy() bool

	// IsReady returns true if the controller is ready.
	IsReady() bool

	// IsLive returns true if the controller is alive.
	IsLive() bool

	// GetMessage returns a human-readable status message.
	GetMessage() string

	// GetDetails returns detailed diagnostic information.
	GetDetails() map[string]interface{}
}

// LeaderElection provides leader election capabilities for controllers.
// This is essential in multi-replica deployments where only one instance
// should actively process work items to avoid conflicts.
type LeaderElection interface {
	// IsLeader returns true if this controller instance is the current leader.
	// Only the leader should actively process work items.
	IsLeader() bool

	// BecomeLeader attempts to acquire leadership.
	// This may block until leadership is acquired or the context is canceled.
	// Returns an error if leadership cannot be acquired.
	BecomeLeader(ctx context.Context) error

	// ResignLeader voluntarily gives up leadership.
	// This allows for graceful leadership transitions.
	// Returns an error if resignation fails.
	ResignLeader(ctx context.Context) error

	// GetLeaderInfo returns information about the current leader.
	// This is useful for debugging and monitoring leader election status.
	GetLeaderInfo() LeaderInfo
}

// LeaderInfo provides information about the current leader.
type LeaderInfo interface {
	// GetLeaderID returns the unique identifier of the current leader.
	GetLeaderID() string

	// GetAcquireTime returns when leadership was acquired.
	GetAcquireTime() *time.Time

	// GetRenewTime returns when leadership was last renewed.
	GetRenewTime() *time.Time

	// GetTransitions returns the number of leadership transitions.
	GetTransitions() int
}

