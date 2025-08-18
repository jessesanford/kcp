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

package health

import (
	"context"
	"fmt"
	"time"
)

// HealthChecker defines the interface for component health checking.
// Each component in the TMC system implements this interface to provide
// health status information.
type HealthChecker interface {
	// Name returns the unique name of the component being health checked.
	Name() string
	
	// Check performs a health check operation and returns the current status.
	// The context can be used for timeouts and cancellation.
	Check(ctx context.Context) HealthStatus
	
	// LastCheck returns the timestamp of the last successful health check.
	LastCheck() time.Time
}

// HealthStatus represents the health status of a component.
type HealthStatus struct {
	// Healthy indicates if the component is healthy (true) or unhealthy (false).
	Healthy bool `json:"healthy"`
	
	// Message provides a human-readable description of the health status.
	Message string `json:"message"`
	
	// Details contains additional structured information about the health status.
	// This can include metrics, error details, or diagnostic information.
	Details map[string]interface{} `json:"details,omitempty"`
	
	// Timestamp indicates when this health status was determined.
	Timestamp time.Time `json:"timestamp"`
}

// String returns a human-readable representation of the health status.
func (hs HealthStatus) String() string {
	status := "UNHEALTHY"
	if hs.Healthy {
		status = "HEALTHY"
	}
	return fmt.Sprintf("[%s] %s (checked at %s)", status, hs.Message, hs.Timestamp.Format(time.RFC3339))
}

// HealthAggregator combines multiple health checkers into an overall system health status.
type HealthAggregator interface {
	// AddChecker adds a health checker to be included in the aggregated health status.
	AddChecker(checker HealthChecker)
	
	// RemoveChecker removes a health checker from the aggregated health status.
	RemoveChecker(name string)
	
	// CheckAll performs health checks on all registered checkers and returns 
	// the aggregated system health status.
	CheckAll(ctx context.Context) SystemHealthStatus
	
	// CheckComponent performs a health check on a specific component by name.
	CheckComponent(ctx context.Context, name string) (HealthStatus, error)
}

// SystemHealthStatus represents the overall health of the TMC system.
type SystemHealthStatus struct {
	// Healthy indicates if the overall system is healthy.
	Healthy bool `json:"healthy"`
	
	// Message provides an overall system health summary.
	Message string `json:"message"`
	
	// Components contains the health status of individual components.
	Components map[string]HealthStatus `json:"components"`
	
	// Timestamp indicates when this system health check was performed.
	Timestamp time.Time `json:"timestamp"`
	
	// HealthyCount is the number of healthy components.
	HealthyCount int `json:"healthy_count"`
	
	// TotalCount is the total number of components checked.
	TotalCount int `json:"total_count"`
}

// String returns a human-readable representation of the system health status.
func (shs SystemHealthStatus) String() string {
	status := "UNHEALTHY"
	if shs.Healthy {
		status = "HEALTHY"
	}
	return fmt.Sprintf("[%s] %s (%d/%d components healthy, checked at %s)", 
		status, shs.Message, shs.HealthyCount, shs.TotalCount, shs.Timestamp.Format(time.RFC3339))
}

// ComponentHealth represents the health status of a single component for easier access.
type ComponentHealth struct {
	Name   string       `json:"name"`
	Status HealthStatus `json:"status"`
}

// HealthConfiguration provides configuration for health checking behavior.
type HealthConfiguration struct {
	// CheckTimeout is the maximum time to wait for a health check to complete.
	CheckTimeout time.Duration `json:"check_timeout"`
	
	// CheckInterval is the interval between automated health checks.
	CheckInterval time.Duration `json:"check_interval"`
	
	// MaxRetries is the maximum number of retries for failed health checks.
	MaxRetries int `json:"max_retries"`
	
	// FailureThreshold is the number of consecutive failures before marking unhealthy.
	FailureThreshold int `json:"failure_threshold"`
}

// DefaultHealthConfiguration returns a default health configuration.
func DefaultHealthConfiguration() HealthConfiguration {
	return HealthConfiguration{
		CheckTimeout:     30 * time.Second,
		CheckInterval:    10 * time.Second,
		MaxRetries:       3,
		FailureThreshold: 2,
	}
}