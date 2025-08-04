/*
Copyright 2022 The KCP Authors.

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
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/reconciler/workload/tmc"
)

// HealthServer provides health check endpoints for the syncer
type HealthServer struct {
	port      int
	server    *http.Server
	tmcHealth *tmc.HealthMonitor

	// Health state
	healthy     bool
	lastCheck   time.Time
	checks      map[string]HealthCheck
	mu          sync.RWMutex
	
	// Lifecycle
	started bool
	stopCh  chan struct{}
}

// HealthCheck represents a health check function
type HealthCheck func() (bool, string)

// HealthStatus represents the overall health status
type HealthStatus struct {
	Healthy   bool                       `json:"healthy"`
	Timestamp time.Time                  `json:"timestamp"`
	Checks    map[string]ComponentHealth `json:"checks"`
}

// ComponentHealth represents the health of a specific component
type ComponentHealth struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message"`
}

// NewHealthServer creates a new health server
func NewHealthServer(port int, tmcHealth *tmc.HealthMonitor) (*HealthServer, error) {
	hs := &HealthServer{
		port:      port,
		tmcHealth: tmcHealth,
		healthy:   true,
		checks:    make(map[string]HealthCheck),
		stopCh:    make(chan struct{}),
	}

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", hs.handleHealthz)
	mux.HandleFunc("/readyz", hs.handleReadyz)
	mux.HandleFunc("/livez", hs.handleLivez)

	hs.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Register default health checks
	hs.RegisterHealthCheck("server", func() (bool, string) {
		return hs.started, "Health server running"
	})

	return hs, nil
}

// Start starts the health server
func (hs *HealthServer) Start(ctx context.Context) error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.started {
		return fmt.Errorf("health server is already started")
	}

	klog.Infof("Starting health server on port %d", hs.port)

	// Start health check loop
	go hs.healthCheckLoop(ctx)

	// Start HTTP server
	go func() {
		if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("Health server error: %v", err)
		}
	}()

	hs.started = true
	klog.Info("Health server started successfully")
	return nil
}

// Stop stops the health server
func (hs *HealthServer) Stop(ctx context.Context) error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if !hs.started {
		return nil
	}

	klog.Info("Stopping health server...")

	// Signal health check loop to stop
	close(hs.stopCh)

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := hs.server.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("Failed to shutdown health server: %v", err)
		return err
	}

	hs.started = false
	klog.Info("Health server stopped")
	return nil
}

// RegisterHealthCheck registers a health check function
func (hs *HealthServer) RegisterHealthCheck(name string, check HealthCheck) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.checks[name] = check
}

// IsHealthy returns true if all health checks pass
func (hs *HealthServer) IsHealthy() bool {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	return hs.healthy
}

// healthCheckLoop runs periodic health checks
func (hs *HealthServer) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	// Run initial health check
	hs.runHealthChecks()

	for {
		select {
		case <-ticker.C:
			hs.runHealthChecks()
		case <-hs.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// runHealthChecks executes all registered health checks
func (hs *HealthServer) runHealthChecks() {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	allHealthy := true
	hs.lastCheck = time.Now()

	// Run each health check
	for name, check := range hs.checks {
		healthy, message := check()
		if !healthy {
			allHealthy = false
			klog.Warningf("Health check %s failed: %s", name, message)
		}
	}

	// Include TMC health if available
	// TODO: Add proper TMC health check when TMC interface is available

	hs.healthy = allHealthy
}

// getHealthStatus returns the current health status
func (hs *HealthServer) getHealthStatus() HealthStatus {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	status := HealthStatus{
		Healthy:   hs.healthy,
		Timestamp: hs.lastCheck,
		Checks:    make(map[string]ComponentHealth),
	}

	// Run checks and collect results
	for name, check := range hs.checks {
		healthy, message := check()
		status.Checks[name] = ComponentHealth{
			Healthy: healthy,
			Message: message,
		}
	}

	// Include TMC health if available
	if hs.tmcHealth != nil {
		status.Checks["tmc"] = ComponentHealth{
			Healthy: true, // TODO: Add proper TMC health check
			Message: "TMC health monitoring",
		}
	}

	return status
}

// HTTP handlers
func (hs *HealthServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
	status := hs.getHealthStatus()
	
	w.Header().Set("Content-Type", "application/json")
	
	if status.Healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	json.NewEncoder(w).Encode(status)
}

func (hs *HealthServer) handleReadyz(w http.ResponseWriter, r *http.Request) {
	// Readiness check - same as health for now
	hs.handleHealthz(w, r)
}

func (hs *HealthServer) handleLivez(w http.ResponseWriter, r *http.Request) {
	// Liveness check - just check if server is running
	hs.mu.RLock()
	started := hs.started
	hs.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	
	status := map[string]interface{}{
		"alive":     started,
		"timestamp": time.Now(),
	}
	
	if started {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	json.NewEncoder(w).Encode(status)
}