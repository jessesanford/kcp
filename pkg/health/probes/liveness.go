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

// LivenessProbe implements a Kubernetes liveness probe for TMC components.
// The liveness probe determines if the application is alive and running.
// If the liveness probe fails, Kubernetes will restart the pod.
type LivenessProbe struct {
	checkers   []health.HealthChecker
	timeout    time.Duration
	criticalComponents map[string]bool // Components that are critical for liveness
}

// LivenessConfig configures the liveness probe behavior.
type LivenessConfig struct {
	// Timeout is the maximum time to wait for health checks.
	Timeout time.Duration `json:"timeout"`
	
	// CriticalComponents is a list of component names that are critical for liveness.
	// If any of these components fail, the liveness probe will fail.
	CriticalComponents []string `json:"critical_components"`
}

// DefaultLivenessConfig returns a default liveness probe configuration.
func DefaultLivenessConfig() LivenessConfig {
	return LivenessConfig{
		Timeout: 10 * time.Second,
		CriticalComponents: []string{
			"controller-api",
			"controller-tenancy", 
		},
	}
}

// NewLivenessProbe creates a new liveness probe.
func NewLivenessProbe(config LivenessConfig) *LivenessProbe {
	criticalComponents := make(map[string]bool)
	for _, component := range config.CriticalComponents {
		criticalComponents[component] = true
	}
	
	return &LivenessProbe{
		timeout:            config.Timeout,
		criticalComponents: criticalComponents,
	}
}

// AddChecker adds a health checker to the liveness probe.
func (l *LivenessProbe) AddChecker(checker health.HealthChecker) {
	l.checkers = append(l.checkers, checker)
}

// ServeHTTP implements the http.Handler interface for the liveness probe endpoint.
func (l *LivenessProbe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), l.timeout)
	defer cancel()
	
	status := l.CheckLiveness(ctx)
	
	if status.Healthy {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "ok")
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "liveness check failed: %s", status.Message)
	}
}

// CheckLiveness performs the liveness check on all registered health checkers.
// Only critical components are considered for the liveness status.
func (l *LivenessProbe) CheckLiveness(ctx context.Context) health.HealthStatus {
	var criticalIssues []string
	var allIssues []string
	criticalHealthy := 0
	totalCritical := 0
	totalHealthy := 0
	
	details := make(map[string]interface{})
	componentStatuses := make(map[string]health.HealthStatus)
	
	// Check all components
	for _, checker := range l.checkers {
		checkCtx, cancel := context.WithTimeout(ctx, l.timeout)
		status := checker.Check(checkCtx)
		cancel()
		
		componentStatuses[checker.Name()] = status
		
		isCritical := l.criticalComponents[checker.Name()]
		if isCritical {
			totalCritical++
			if status.Healthy {
				criticalHealthy++
			} else {
				criticalIssues = append(criticalIssues, 
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
	
	details["critical_healthy"] = criticalHealthy
	details["total_critical"] = totalCritical
	details["total_healthy"] = totalHealthy
	details["total_components"] = len(l.checkers)
	details["component_statuses"] = componentStatuses
	
	// Liveness is determined by critical components only
	healthy := len(criticalIssues) == 0
	var message string
	
	if healthy {
		if totalCritical == 0 {
			message = "no critical components configured, all components running"
		} else {
			message = fmt.Sprintf("all %d critical components are healthy", totalCritical)
		}
	} else {
		message = fmt.Sprintf("liveness check failed: %d critical component(s) unhealthy: %v", 
			len(criticalIssues), criticalIssues)
	}
	
	return health.HealthStatus{
		Healthy:   healthy,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// GetCriticalComponents returns the list of components that are critical for liveness.
func (l *LivenessProbe) GetCriticalComponents() []string {
	var critical []string
	for component := range l.criticalComponents {
		critical = append(critical, component)
	}
	return critical
}

// SetCriticalComponents sets the list of components that are critical for liveness.
func (l *LivenessProbe) SetCriticalComponents(components []string) {
	l.criticalComponents = make(map[string]bool)
	for _, component := range components {
		l.criticalComponents[component] = true
	}
}

// AddCriticalComponent adds a component to the critical components list.
func (l *LivenessProbe) AddCriticalComponent(component string) {
	l.criticalComponents[component] = true
}

// RemoveCriticalComponent removes a component from the critical components list.
func (l *LivenessProbe) RemoveCriticalComponent(component string) {
	delete(l.criticalComponents, component)
}