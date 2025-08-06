// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &Config{
			KCPConfig:      &rest.Config{Host: "https://kcp.example.com"},
			Workspace:      "root:test",
			ClusterConfigs: map[string]*rest.Config{},
			ResyncPeriod:   30 * time.Second,
			WorkerCount:    5,
		}

		err := validateConfig(config)
		assert.NoError(t, err)
	})

	t.Run("missing KCPConfig", func(t *testing.T) {
		config := &Config{
			Workspace:    "root:test",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  5,
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "KCPConfig is required")
	})

	t.Run("empty workspace", func(t *testing.T) {
		config := &Config{
			KCPConfig:    &rest.Config{Host: "https://kcp.example.com"},
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  5,
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Workspace is required")
	})

	t.Run("invalid resync period", func(t *testing.T) {
		config := &Config{
			KCPConfig:    &rest.Config{Host: "https://kcp.example.com"},
			Workspace:    "root:test",
			ResyncPeriod: -1 * time.Second,
			WorkerCount:  5,
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ResyncPeriod must be positive")
	})

	t.Run("invalid worker count", func(t *testing.T) {
		config := &Config{
			KCPConfig:    &rest.Config{Host: "https://kcp.example.com"},
			Workspace:    "root:test",
			ResyncPeriod: 30 * time.Second,
			WorkerCount:  0,
		}

		err := validateConfig(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "WorkerCount must be positive")
	})
}

func TestNewManager(t *testing.T) {
	t.Run("nil config error", func(t *testing.T) {
		ctx := context.Background()
		manager, err := NewManager(ctx, nil)
		assert.Error(t, err)
		assert.Nil(t, manager)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("invalid config error", func(t *testing.T) {
		ctx := context.Background()
		config := &Config{
			// Missing required fields
		}

		manager, err := NewManager(ctx, config)
		assert.Error(t, err)
		assert.Nil(t, manager)
		assert.Contains(t, err.Error(), "invalid configuration")
	})

	t.Run("valid config but invalid kcp config", func(t *testing.T) {
		ctx := context.Background()
		config := &Config{
			KCPConfig:      &rest.Config{Host: "invalid-url"}, // Invalid URL
			Workspace:      "root:test",
			ClusterConfigs: map[string]*rest.Config{},
			ResyncPeriod:   30 * time.Second,
			WorkerCount:    5,
		}

		manager, err := NewManager(ctx, config)
		assert.Error(t, err)
		assert.Nil(t, manager)
		assert.Contains(t, err.Error(), "failed to create KCP cluster client")
	})
}

func TestManager_Lifecycle(t *testing.T) {
	t.Run("start not started manager error", func(t *testing.T) {
		// Create a mock HTTP server to simulate KCP
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &Config{
			KCPConfig: &rest.Config{
				Host:    server.URL,
				Timeout: 1 * time.Second,
			},
			Workspace:      "root:test",
			ClusterConfigs: map[string]*rest.Config{},
			ResyncPeriod:   30 * time.Second,
			WorkerCount:    1,
			MetricsPort:    0, // Disable metrics server
			HealthPort:     0, // Disable health server
		}

		ctx := context.Background()
		manager, err := NewManager(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, manager)

		// Verify manager is created but not started
		manager.mu.RLock()
		started := manager.started
		manager.mu.RUnlock()
		assert.False(t, started)

		// Try to start the same manager twice - should get error
		manager.mu.Lock()
		manager.started = true
		manager.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err = manager.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "manager already started")
	})

	t.Run("shutdown lifecycle", func(t *testing.T) {
		manager := &Manager{}

		// Test shutdown when not started - should succeed
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := manager.Shutdown(ctx)
		assert.NoError(t, err)

		// Test shutdown when already stopping - should succeed
		manager.mu.Lock()
		manager.started = true
		manager.stopping = true
		manager.mu.Unlock()

		err = manager.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

func TestManager_ObservabilityServers(t *testing.T) {
	t.Run("initialize metrics server", func(t *testing.T) {
		config := &Config{
			MetricsPort: 9090,
			HealthPort:  0, // Disable health server
		}

		manager := &Manager{
			config: config,
		}

		err := manager.initializeObservabilityServers()
		assert.NoError(t, err)
		assert.NotNil(t, manager.metricsServer)
		assert.Nil(t, manager.healthServer)
		assert.Equal(t, ":9090", manager.metricsServer.Addr)
	})

	t.Run("initialize health server", func(t *testing.T) {
		config := &Config{
			MetricsPort: 0, // Disable metrics server
			HealthPort:  8081,
		}

		manager := &Manager{
			config: config,
		}

		err := manager.initializeObservabilityServers()
		assert.NoError(t, err)
		assert.Nil(t, manager.metricsServer)
		assert.NotNil(t, manager.healthServer)
		assert.Equal(t, ":8081", manager.healthServer.Addr)
	})

	t.Run("initialize both servers", func(t *testing.T) {
		config := &Config{
			MetricsPort: 9090,
			HealthPort:  8081,
		}

		manager := &Manager{
			config: config,
		}

		err := manager.initializeObservabilityServers()
		assert.NoError(t, err)
		assert.NotNil(t, manager.metricsServer)
		assert.NotNil(t, manager.healthServer)
		assert.Equal(t, ":9090", manager.metricsServer.Addr)
		assert.Equal(t, ":8081", manager.healthServer.Addr)
	})

	t.Run("disabled servers", func(t *testing.T) {
		config := &Config{
			MetricsPort: 0, // Disabled
			HealthPort:  0, // Disabled
		}

		manager := &Manager{
			config: config,
		}

		err := manager.initializeObservabilityServers()
		assert.NoError(t, err)
		assert.Nil(t, manager.metricsServer)
		assert.Nil(t, manager.healthServer)
	})
}

func TestManager_HealthHandlers(t *testing.T) {
	t.Run("health handler when started", func(t *testing.T) {
		manager := &Manager{}
		manager.mu.Lock()
		manager.started = true
		manager.stopping = false
		manager.mu.Unlock()

		// Create test request
		req := httptest.NewRequest("GET", "/healthz", nil)
		w := httptest.NewRecorder()

		// Call handler
		manager.healthHandler(w, req)

		// Should return OK
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("health handler when not started", func(t *testing.T) {
		manager := &Manager{}
		manager.mu.Lock()
		manager.started = false
		manager.mu.Unlock()

		// Create test request
		req := httptest.NewRequest("GET", "/healthz", nil)
		w := httptest.NewRecorder()

		// Call handler
		manager.healthHandler(w, req)

		// Should return Service Unavailable
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("health handler when stopping", func(t *testing.T) {
		manager := &Manager{}
		manager.mu.Lock()
		manager.started = true
		manager.stopping = true
		manager.mu.Unlock()

		// Create test request
		req := httptest.NewRequest("GET", "/healthz", nil)
		w := httptest.NewRecorder()

		// Call handler
		manager.healthHandler(w, req)

		// Should return Service Unavailable
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("readiness handler delegates to health", func(t *testing.T) {
		manager := &Manager{}
		manager.mu.Lock()
		manager.started = true
		manager.stopping = false
		manager.mu.Unlock()

		// Create test request
		req := httptest.NewRequest("GET", "/readyz", nil)
		w := httptest.NewRecorder()

		// Call handler
		manager.readinessHandler(w, req)

		// Should return OK (same as health)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())
	})
}