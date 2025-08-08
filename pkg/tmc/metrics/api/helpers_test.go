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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseQuery(t *testing.T) {
	tests := map[string]struct {
		url         string
		wantMetric  string
		wantError   bool
		wantLabels  map[string]string
		wantLimit   int
		wantOffset  int
	}{
		"valid basic query": {
			url:        "/api/v1/metrics/query?metric=cpu_usage",
			wantMetric: "cpu_usage",
			wantError:  false,
		},
		"query with labels": {
			url:        "/api/v1/metrics/query?metric=cpu_usage&label.cluster=test&label.workspace=prod",
			wantMetric: "cpu_usage",
			wantError:  false,
			wantLabels: map[string]string{"cluster": "test", "workspace": "prod"},
		},
		"query with pagination": {
			url:        "/api/v1/metrics/query?metric=cpu_usage&limit=100&offset=50",
			wantMetric: "cpu_usage",
			wantError:  false,
			wantLimit:  100,
			wantOffset: 50,
		},
		"missing metric parameter": {
			url:       "/api/v1/metrics/query",
			wantError: true,
		},
		"invalid limit": {
			url:       "/api/v1/metrics/query?metric=cpu_usage&limit=invalid",
			wantError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			u, err := url.Parse(tc.url)
			if err != nil {
				t.Fatalf("Failed to parse test URL: %v", err)
			}
			
			req := &http.Request{URL: u}
			query, err := ParseQuery(req)
			
			if tc.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if query.MetricName != tc.wantMetric {
				t.Errorf("Expected metric name %q, got %q", tc.wantMetric, query.MetricName)
			}
			
			if tc.wantLabels != nil {
				for k, v := range tc.wantLabels {
					if query.Labels[k] != v {
						t.Errorf("Expected label %s=%s, got %s", k, v, query.Labels[k])
					}
				}
			}
			
			if query.Limit != tc.wantLimit {
				t.Errorf("Expected limit %d, got %d", tc.wantLimit, query.Limit)
			}
			
			if query.Offset != tc.wantOffset {
				t.Errorf("Expected offset %d, got %d", tc.wantOffset, query.Offset)
			}
		})
	}
}

func TestParseQueryRange(t *testing.T) {
	tests := map[string]struct {
		url       string
		wantError bool
		wantStep  time.Duration
	}{
		"valid range query": {
			url:      "/api/v1/metrics/query_range?metric=cpu_usage&start=1640995200&end=1641081600&step=5m",
			wantStep: 5 * time.Minute,
		},
		"missing start time": {
			url:       "/api/v1/metrics/query_range?metric=cpu_usage&end=1641081600",
			wantError: true,
		},
		"missing end time": {
			url:       "/api/v1/metrics/query_range?metric=cpu_usage&start=1640995200",
			wantError: true,
		},
		"default step": {
			url:      "/api/v1/metrics/query_range?metric=cpu_usage&start=1640995200&end=1641081600",
			wantStep: 1 * time.Minute,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			u, err := url.Parse(tc.url)
			if err != nil {
				t.Fatalf("Failed to parse test URL: %v", err)
			}
			
			req := &http.Request{URL: u}
			query, err := ParseQueryRange(req)
			
			if tc.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if query.Step != nil && *query.Step != tc.wantStep {
				t.Errorf("Expected step %v, got %v", tc.wantStep, *query.Step)
			}
		})
	}
}

func TestGetAuthContext(t *testing.T) {
	tests := map[string]struct {
		headers       map[string]string
		queryParams   map[string]string
		wantUser      string
		wantWorkspace string
		wantGroups    []string
	}{
		"X-Remote-User header": {
			headers:  map[string]string{"X-Remote-User": "testuser"},
			wantUser: "testuser",
		},
		"Authorization Bearer": {
			headers:  map[string]string{"Authorization": "Bearer token123"},
			wantUser: "token123",
		},
		"X-Remote-Groups header": {
			headers:    map[string]string{"X-Remote-Groups": "admin,developer"},
			wantGroups: []string{"admin", "developer"},
		},
		"workspace from query": {
			queryParams:   map[string]string{"workspace": "production"},
			wantWorkspace: "production",
		},
		"workspace from header": {
			headers:       map[string]string{"X-Kcp-Workspace": "development"},
			wantWorkspace: "development",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			
			if len(tc.queryParams) > 0 {
				q := req.URL.Query()
				for k, v := range tc.queryParams {
					q.Set(k, v)
				}
				req.URL.RawQuery = q.Encode()
			}
			
			auth := GetAuthContext(req)
			
			if auth.User != tc.wantUser {
				t.Errorf("Expected user %q, got %q", tc.wantUser, auth.User)
			}
			
			if auth.Workspace != tc.wantWorkspace {
				t.Errorf("Expected workspace %q, got %q", tc.wantWorkspace, auth.Workspace)
			}
			
			if len(auth.Groups) != len(tc.wantGroups) {
				t.Errorf("Expected %d groups, got %d", len(tc.wantGroups), len(auth.Groups))
			} else {
				for i, group := range tc.wantGroups {
					if auth.Groups[i] != group {
						t.Errorf("Expected group[%d] %q, got %q", i, group, auth.Groups[i])
					}
				}
			}
		})
	}
}

func TestConvertToPrometheusFormat(t *testing.T) {
	series := []MetricSeries{
		{
			Name: "test_metric",
			Values: []MetricValue{
				{
					Timestamp: time.Unix(1640995200, 0),
					Value:     42.5,
					Labels:    map[string]string{"job": "test"},
				},
			},
		},
	}
	
	result := ConvertToPrometheusFormat(series)
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}
	
	if resultMap["resultType"] != "vector" {
		t.Errorf("Expected resultType 'vector', got %v", resultMap["resultType"])
	}
	
	resultSlice, ok := resultMap["result"].([]map[string]interface{})
	if !ok || len(resultSlice) != 1 {
		t.Fatal("Expected result array with one element")
	}
	
	metric := resultSlice[0]
	if metric["metric"].(map[string]string)["job"] != "test" {
		t.Error("Expected metric label job=test")
	}
}

func TestWriteJSONResponse(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"status": "success"}
	
	WriteJSONResponse(w, http.StatusOK, data)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}
	
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}
	
	body := w.Body.String()
	if !strings.Contains(body, "success") {
		t.Error("Expected response body to contain 'success'")
	}
}

func TestWriteErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	
	WriteErrorResponse(w, http.StatusBadRequest, "bad_data", "Invalid parameter")
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code 400, got %d", w.Code)
	}
	
	body := w.Body.String()
	if !strings.Contains(body, "error") {
		t.Error("Expected response body to contain 'error'")
	}
	if !strings.Contains(body, "bad_data") {
		t.Error("Expected response body to contain error type")
	}
	if !strings.Contains(body, "Invalid parameter") {
		t.Error("Expected response body to contain error message")
	}
}

func TestAuthMiddleware(t *testing.T) {
	authContext := func(r *http.Request) *AuthContext {
		return &AuthContext{User: r.Header.Get("X-User")}
	}
	
	middleware := AuthMiddleware(authContext)
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// Test authenticated request
	t.Run("authenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/query", nil)
		req.Header.Set("X-User", "testuser")
		w := httptest.NewRecorder()
		
		middleware(handler).ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
	
	// Test unauthenticated request
	t.Run("unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/query", nil)
		w := httptest.NewRecorder()
		
		middleware(handler).ServeHTTP(w, req)
		
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
	
	// Test health check bypass
	t.Run("health check bypass", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/healthz", nil)
		w := httptest.NewRecorder()
		
		middleware(handler).ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for health check, got %d", w.Code)
		}
	})
}