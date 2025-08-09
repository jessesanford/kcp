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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/rest"
)

func TestNewHealthChecker(t *testing.T) {
	tests := map[string]struct {
		config *HealthMonitorConfig
	}{
		"with config": {
			config: &HealthMonitorConfig{
				Timeout:    5 * time.Second,
				MaxRetries: 2,
			},
		},
		"with nil config": {
			config: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			checker := NewHealthChecker(tc.config)
			if checker == nil {
				t.Errorf("NewHealthChecker() returned nil")
			}
		})
	}
}

func TestHealthChecker_CheckAPIServer(t *testing.T) {
	tests := map[string]struct {
		serverResponse   string
		serverStatusCode int
		wantStatus       HealthStatus
		wantError        bool
	}{
		"healthy api server": {
			serverResponse: `{
				"major": "1",
				"minor": "24", 
				"gitVersion": "v1.24.0"
			}`,
			serverStatusCode: http.StatusOK,
			wantStatus:       HealthStatusHealthy,
			wantError:        false,
		},
		"server error": {
			serverResponse:   "Internal Server Error",
			serverStatusCode: http.StatusInternalServerError,
			wantStatus:       HealthStatusUnhealthy,
			wantError:        true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.serverStatusCode)
				if strings.Contains(r.URL.Path, "/version") {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(tc.serverResponse))
				}
			}))
			defer server.Close()

			config := &HealthMonitorConfig{
				Timeout:    5 * time.Second,
				MaxRetries: 1,
				RetryDelay: 100 * time.Millisecond,
			}
			checker := NewHealthChecker(config)

			restConfig := &rest.Config{
				Host: server.URL,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := checker.CheckAPIServer(ctx, restConfig)

			if tc.wantError && err == nil {
				t.Errorf("CheckAPIServer() expected error but got none")
			}
			if !tc.wantError && err != nil {
				t.Errorf("CheckAPIServer() unexpected error: %v", err)
			}

			if result == nil {
				if !tc.wantError {
					t.Errorf("CheckAPIServer() returned nil result")
				}
				return
			}

			if result.Status != tc.wantStatus {
				t.Errorf("CheckAPIServer() status = %v, want %v", result.Status, tc.wantStatus)
			}

			if result.LastChecked.IsZero() {
				t.Errorf("CheckAPIServer() LastChecked should be set")
			}
		})
	}
}

func TestDefaultHealthMonitorConfig(t *testing.T) {
	config := DefaultHealthMonitorConfig()
	
	if config == nil {
		t.Fatal("DefaultHealthMonitorConfig() returned nil")
	}
	
	if config.Timeout <= 0 {
		t.Errorf("Timeout should be positive, got %v", config.Timeout)
	}
	
	if config.MaxRetries < 0 {
		t.Errorf("MaxRetries should be non-negative, got %v", config.MaxRetries)
	}
	
	if config.RetryDelay < 0 {
		t.Errorf("RetryDelay should be non-negative, got %v", config.RetryDelay)
	}
}

func TestCalculateOverallHealth(t *testing.T) {
	checker := &clusterHealthChecker{
		config: DefaultHealthMonitorConfig(),
	}
	
	tests := map[string]struct {
		health     *ClusterHealth
		wantStatus HealthStatus
	}{
		"all healthy": {
			health: &ClusterHealth{
				APIServerHealth:    ComponentHealth{Status: HealthStatusHealthy},
				ConnectivityHealth: ComponentHealth{Status: HealthStatusHealthy},
			},
			wantStatus: HealthStatusHealthy,
		},
		"api server unhealthy": {
			health: &ClusterHealth{
				APIServerHealth:    ComponentHealth{Status: HealthStatusUnhealthy},
				ConnectivityHealth: ComponentHealth{Status: HealthStatusHealthy},
			},
			wantStatus: HealthStatusUnhealthy,
		},
		"connectivity unhealthy": {
			health: &ClusterHealth{
				APIServerHealth:    ComponentHealth{Status: HealthStatusHealthy},
				ConnectivityHealth: ComponentHealth{Status: HealthStatusUnhealthy},
			},
			wantStatus: HealthStatusUnhealthy,
		},
		"api degraded": {
			health: &ClusterHealth{
				APIServerHealth:    ComponentHealth{Status: HealthStatusDegraded},
				ConnectivityHealth: ComponentHealth{Status: HealthStatusHealthy},
			},
			wantStatus: HealthStatusDegraded,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := checker.calculateOverallHealth(tc.health)
			if result != tc.wantStatus {
				t.Errorf("calculateOverallHealth() = %v, want %v", result, tc.wantStatus)
			}
		})
	}
}