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

package probes

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kcp-dev/kcp/pkg/health"
)

// ReadinessProbe implements a Kubernetes readiness probe for TMC components.
// The readiness probe determines if the application is ready to serve traffic.
// If the readiness probe fails, Kubernetes will remove the pod from service endpoints.
type ReadinessProbe struct {
	checkers      []health.HealthChecker
	timeout       time.Duration
	startupGrace  time.Duration
	startupTime   time.Time
	dependencies  map[string]bool // Components that must be ready for service
}

// ReadinessConfig configures the readiness probe behavior.
type ReadinessConfig struct {
	// Timeout is the maximum time to wait for health checks.
	Timeout time.Duration `json:"timeout"`
	
	// StartupGrace is the grace period after startup before readiness checks are enforced.
	StartupGrace time.Duration `json:"startup_grace"`
	
	// Dependencies is a list of component names that must be ready for the service to be ready.
	Dependencies []string `json:"dependencies"`
}

// DefaultReadinessConfig returns a default readiness probe configuration.
func DefaultReadinessConfig() ReadinessConfig {
	return ReadinessConfig{
		Timeout:      15 * time.Second,
		StartupGrace: 30 * time.Second,
		Dependencies: []string{
			"controller-api",
			"controller-tenancy",
			"syncer-default",
			"placement-scheduler",
		},
	}
}

// NewReadinessProbe creates a new readiness probe.
func NewReadinessProbe(config ReadinessConfig) *ReadinessProbe {
	dependencies := make(map[string]bool)
	for _, dependency := range config.Dependencies {
		dependencies[dependency] = true
	}
	
	return &ReadinessProbe{
		timeout:      config.Timeout,
		startupGrace: config.StartupGrace,
		startupTime:  time.Now(),
		dependencies: dependencies,
	}
}

// AddChecker adds a health checker to the readiness probe.
func (r *ReadinessProbe) AddChecker(checker health.HealthChecker) {
	r.checkers = append(r.checkers, checker)
}

// ServeHTTP implements the http.Handler interface for the readiness probe endpoint.
func (r *ReadinessProbe) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), r.timeout)
	defer cancel()
	
	status := r.CheckReadiness(ctx)
	
	if status.Healthy {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "ready")
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "readiness check failed: %s", status.Message)
	}
}

// CheckReadiness performs the readiness check on all registered health checkers.
// All dependency components must be healthy for the service to be considered ready.
func (r *ReadinessProbe) CheckReadiness(ctx context.Context) health.HealthStatus {
	// Check if we're still in startup grace period
	timeSinceStartup := time.Since(r.startupTime)
	inGracePeriod := timeSinceStartup < r.startupGrace
	
	var dependencyIssues []string
	var allIssues []string
	dependencyHealthy := 0
	totalDependencies := 0
	totalHealthy := 0
	
	details := make(map[string]interface{})
	componentStatuses := make(map[string]health.HealthStatus)
	
	details["startup_time"] = r.startupTime
	details["time_since_startup_seconds"] = timeSinceStartup.Seconds()
	details["startup_grace_seconds"] = r.startupGrace.Seconds()
	details["in_grace_period"] = inGracePeriod
	
	// Check all components
	for _, checker := range r.checkers {
		checkCtx, cancel := context.WithTimeout(ctx, r.timeout)
		status := checker.Check(checkCtx)
		cancel()
		
		componentStatuses[checker.Name()] = status
		
		isDependency := r.dependencies[checker.Name()]
		if isDependency {
			totalDependencies++
			if status.Healthy {
				dependencyHealthy++
			} else {
				dependencyIssues = append(dependencyIssues, 
					fmt.Sprintf("%s: %s", checker.Name(), status.Message))
			}
		}
		
		if status.Healthy {
			totalHealthy++
		} else {
			allIssues = append(allIssues, 
				fmt.Sprintf("%s: %s", checker.Name(), status.Message))
		}
	}
	
	details["dependency_healthy"] = dependencyHealthy
	details["total_dependencies"] = totalDependencies
	details["total_healthy"] = totalHealthy
	details["total_components"] = len(r.checkers)
	details["component_statuses"] = componentStatuses
	
	// Readiness is determined by dependency components, but allow grace period
	var healthy bool
	var message string
	
	if inGracePeriod {
		// During grace period, we're ready even if some components aren't healthy yet
		healthy = true
		graceRemaining := r.startupGrace - timeSinceStartup
		message = fmt.Sprintf("in startup grace period (%.0fs remaining), readiness assumed", 
			graceRemaining.Seconds())
	} else {
		// After grace period, all dependencies must be healthy
		healthy = len(dependencyIssues) == 0
		
		if healthy {
			if totalDependencies == 0 {
				message = "no dependencies configured, service ready"
			} else {
				message = fmt.Sprintf("all %d dependencies are healthy, service ready", totalDependencies)
			}
		} else {
			message = fmt.Sprintf("readiness check failed: %d dependency component(s) unhealthy: %v", 
				len(dependencyIssues), dependencyIssues)
		}
	}
	
	return health.HealthStatus{
		Healthy:   healthy,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// GetDependencies returns the list of components that are dependencies for readiness.
func (r *ReadinessProbe) GetDependencies() []string {
	var deps []string
	for dependency := range r.dependencies {
		deps = append(deps, dependency)
	}
	return deps
}

// SetDependencies sets the list of components that are dependencies for readiness.
func (r *ReadinessProbe) SetDependencies(dependencies []string) {
	r.dependencies = make(map[string]bool)
	for _, dependency := range dependencies {
		r.dependencies[dependency] = true
	}
}

// AddDependency adds a component to the dependencies list.
func (r *ReadinessProbe) AddDependency(dependency string) {
	r.dependencies[dependency] = true
}

// RemoveDependency removes a component from the dependencies list.
func (r *ReadinessProbe) RemoveDependency(dependency string) {
	delete(r.dependencies, dependency)
}

// ResetStartupTime resets the startup time to the current time.
// This can be useful for testing or when restarting components.
func (r *ReadinessProbe) ResetStartupTime() {
	r.startupTime = time.Now()
}

// GetStartupTime returns the time when the readiness probe was created.
func (r *ReadinessProbe) GetStartupTime() time.Time {
	return r.startupTime
}

// IsInGracePeriod returns true if the probe is still in the startup grace period.
func (r *ReadinessProbe) IsInGracePeriod() bool {
	return time.Since(r.startupTime) < r.startupGrace
}