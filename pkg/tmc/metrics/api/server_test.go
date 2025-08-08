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

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kcp-dev/kcp/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// mockMetricsStore implements MetricsStore for testing.
type mockMetricsStore struct {
	queryFunc       func(ctx context.Context, query *MetricQuery) ([]MetricSeries, error)
	metricNamesFunc func(ctx context.Context) ([]string, error)
}

func (m *mockMetricsStore) Query(ctx context.Context, query *MetricQuery) ([]MetricSeries, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, query)
	}
	return []MetricSeries{{
		Name: query.MetricName, Help: "Mock metric", Type: "gauge",
		Values: []MetricValue{{Timestamp: time.Now(), Value: 42.0, Labels: map[string]string{"test": "true"}}},
	}}, nil
}

func (m *mockMetricsStore) GetMetricNames(ctx context.Context) ([]string, error) {
	if m.metricNamesFunc != nil {
		return m.metricNamesFunc(ctx)
	}
	return []string{"tmc_cluster_health", "tmc_placement_decisions_total"}, nil
}

// mockAuthorizer implements Authorizer for testing.
type mockAuthorizer struct {
	authorizeFunc func(ctx context.Context, auth *AuthContext, query *MetricQuery) error
}

func (m *mockAuthorizer) Authorize(ctx context.Context, auth *AuthContext, query *MetricQuery) error {
	if m.authorizeFunc != nil {
		return m.authorizeFunc(ctx, auth, query)
	}
	return nil
}

func TestMetricsAPIServer_NewServer(t *testing.T) {
	store := &mockMetricsStore{}
	authorizer := &mockAuthorizer{}
	server := NewMetricsAPIServer(8080, store, authorizer)
	
	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if server.store != store || server.authorizer != authorizer {
		t.Error("Expected store and authorizer to be set correctly")
	}
}

func TestMetricsAPIServer_HandleQuery(t *testing.T) {
	tests := map[string]struct {
		queryParams    map[string]string
		headers        map[string]string
		expectedStatus int
	}{
		"valid query": {
			queryParams:    map[string]string{"metric": "tmc_cluster_health", "workspace": "root:test"},
			headers:        map[string]string{"X-Remote-User": "test-user"},
			expectedStatus: http.StatusOK,
		},
		"missing metric": {
			queryParams:    map[string]string{"workspace": "root:test"},
			headers:        map[string]string{"X-Remote-User": "test-user"},
			expectedStatus: http.StatusBadRequest,
		},
		"missing auth": {
			queryParams:    map[string]string{"metric": "tmc_cluster_health"},
			expectedStatus: http.StatusUnauthorized,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			utilfeature.DefaultMutableFeatureGate.Set(string(features.TMCMetricsAPI), true)
			defer utilfeature.DefaultMutableFeatureGate.Set(string(features.TMCMetricsAPI), false)
			
			server := NewMetricsAPIServer(8080, &mockMetricsStore{}, &mockAuthorizer{})
			
			baseURL := "/api/v1/metrics/query"
			if len(tc.queryParams) > 0 {
				params := url.Values{}
				for k, v := range tc.queryParams {
					params.Add(k, v)
				}
				baseURL += "?" + params.Encode()
			}
			
			req := httptest.NewRequest("GET", baseURL, nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			
			rec := httptest.NewRecorder()
			server.router.ServeHTTP(rec, req)
			
			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
			}
		})
	}
}

func TestMetricsAPIServer_PrometheusCompatibility(t *testing.T) {
	utilfeature.DefaultMutableFeatureGate.Set(string(features.TMCMetricsAPI), true)
	defer utilfeature.DefaultMutableFeatureGate.Set(string(features.TMCMetricsAPI), false)
	
	server := NewMetricsAPIServer(8080, &mockMetricsStore{}, &mockAuthorizer{})
	
	params := url.Values{}
	params.Add("query", "tmc_cluster_health{cluster=\"test\"}")
	params.Add("time", "1609459200")
	
	req := httptest.NewRequest("GET", "/api/v1/query?"+params.Encode(), nil)
	req.Header.Set("X-Remote-User", "test-user")
	
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	var response PrometheusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	
	if response.Status != "success" {
		t.Errorf("Expected success status, got %s", response.Status)
	}
}

func TestMetricsAPIServer_Authorization(t *testing.T) {
	utilfeature.DefaultMutableFeatureGate.Set(string(features.TMCMetricsAPI), true)
	defer utilfeature.DefaultMutableFeatureGate.Set(string(features.TMCMetricsAPI), false)
	
	authorizer := &mockAuthorizer{
		authorizeFunc: func(ctx context.Context, auth *AuthContext, query *MetricQuery) error {
			if auth.User != "admin" {
				return fmt.Errorf("access denied")
			}
			return nil
		},
	}
	
	server := NewMetricsAPIServer(8080, &mockMetricsStore{}, authorizer)
	
	// Test unauthorized user
	params := url.Values{}
	params.Add("metric", "tmc_cluster_health")
	
	req := httptest.NewRequest("GET", "/api/v1/metrics/query?"+params.Encode(), nil)
	req.Header.Set("X-Remote-User", "regular-user")
	
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rec.Code)
	}
	
	// Test authorized admin user
	req = httptest.NewRequest("GET", "/api/v1/metrics/query?"+params.Encode(), nil)
	req.Header.Set("X-Remote-User", "admin")
	
	rec = httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for admin user, got %d", rec.Code)
	}
}

func TestMetricsAPIServer_HealthCheck(t *testing.T) {
	server := NewMetricsAPIServer(8080, &mockMetricsStore{}, &mockAuthorizer{})
	
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	
	if strings.TrimSpace(rec.Body.String()) != "OK" {
		t.Errorf("Expected 'OK' response, got %s", rec.Body.String())
	}
}

func TestParseTimeFormats(t *testing.T) {
	server := NewMetricsAPIServer(8080, &mockMetricsStore{}, &mockAuthorizer{})
	
	tests := map[string]struct {
		timeStr     string
		expectError bool
	}{
		"unix timestamp":         {"1609459200", false},
		"unix with decimal":      {"1609459200.123", false},
		"rfc3339 format":         {"2021-01-01T00:00:00Z", false},
		"iso8601 format":         {"2021-01-01T00:00:00Z", false},
		"invalid format":         {"invalid-time", true},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := server.parseTime(tc.timeStr)
			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}