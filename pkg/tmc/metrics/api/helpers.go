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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// parseQuery parses standard TMC metrics query parameters.
func (s *MetricsAPIServer) parseQuery(r *http.Request) (*MetricQuery, error) {
	query := &MetricQuery{MetricName: r.URL.Query().Get("metric")}
	if query.MetricName == "" {
		return nil, fmt.Errorf("metric parameter is required")
	}
	
	// Parse time range
	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if start, err := s.parseTime(startStr); err != nil {
			return nil, fmt.Errorf("invalid start time: %w", err)
		} else {
			query.StartTime = &start
		}
	}
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if end, err := s.parseTime(endStr); err != nil {
			return nil, fmt.Errorf("invalid end time: %w", err)
		} else {
			query.EndTime = &end
		}
	}
	
	// Parse step, labels, filters
	if stepStr := r.URL.Query().Get("step"); stepStr != "" {
		if step, err := time.ParseDuration(stepStr); err != nil {
			return nil, fmt.Errorf("invalid step duration: %w", err)
		} else {
			query.Step = &step
		}
	}
	
	query.Labels = make(map[string]string)
	for key, values := range r.URL.Query() {
		if strings.HasPrefix(key, "label.") && len(values) > 0 {
			query.Labels[strings.TrimPrefix(key, "label.")] = values[0]
		}
	}
	
	query.Workspace = r.URL.Query().Get("workspace")
	query.Cluster = r.URL.Query().Get("cluster")
	
	// Parse pagination
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err != nil {
			return nil, fmt.Errorf("invalid limit: %w", err)
		} else {
			query.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err != nil {
			return nil, fmt.Errorf("invalid offset: %w", err)
		} else {
			query.Offset = offset
		}
	}
	return query, nil
}

// parseQueryRange parses range query parameters with required time bounds.
func (s *MetricsAPIServer) parseQueryRange(r *http.Request) (*MetricQuery, error) {
	query, err := s.parseQuery(r)
	if err != nil {
		return nil, err
	}
	if query.StartTime == nil {
		return nil, fmt.Errorf("start time is required for range queries")
	}
	if query.EndTime == nil {
		return nil, fmt.Errorf("end time is required for range queries")
	}
	if query.Step == nil {
		defaultStep := 1 * time.Minute
		query.Step = &defaultStep
	}
	return query, nil
}

// parsePrometheusQuery parses Prometheus-compatible query parameters.
func (s *MetricsAPIServer) parsePrometheusQuery(r *http.Request) (*MetricQuery, error) {
	query := &MetricQuery{MetricName: r.URL.Query().Get("query")}
	if query.MetricName == "" {
		return nil, fmt.Errorf("query parameter is required")
	}
	if timeStr := r.URL.Query().Get("time"); timeStr != "" {
		if t, err := s.parseTime(timeStr); err != nil {
			return nil, fmt.Errorf("invalid time: %w", err)
		} else {
			query.StartTime = &t
			query.EndTime = &t
		}
	}
	return query, nil
}

// parsePrometheusQueryRange parses Prometheus-compatible range query parameters.
func (s *MetricsAPIServer) parsePrometheusQueryRange(r *http.Request) (*MetricQuery, error) {
	query := &MetricQuery{MetricName: r.URL.Query().Get("query")}
	if query.MetricName == "" {
		return nil, fmt.Errorf("query parameter is required")
	}
	
	// Parse required time range
	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if start, err := s.parseTime(startStr); err != nil {
			return nil, fmt.Errorf("invalid start time: %w", err)
		} else {
			query.StartTime = &start
		}
	} else {
		return nil, fmt.Errorf("start parameter is required")
	}
	
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if end, err := s.parseTime(endStr); err != nil {
			return nil, fmt.Errorf("invalid end time: %w", err)
		} else {
			query.EndTime = &end
		}
	} else {
		return nil, fmt.Errorf("end parameter is required")
	}
	
	if stepStr := r.URL.Query().Get("step"); stepStr != "" {
		if step, err := time.ParseDuration(stepStr); err != nil {
			return nil, fmt.Errorf("invalid step: %w", err)
		} else {
			query.Step = &step
		}
	} else {
		return nil, fmt.Errorf("step parameter is required")
	}
	return query, nil
}

// parseTime parses various time formats (Unix timestamp, RFC3339, etc.).
func (s *MetricsAPIServer) parseTime(timeStr string) (time.Time, error) {
	// Try Unix timestamp first
	if timestamp, err := strconv.ParseFloat(timeStr, 64); err == nil {
		return time.Unix(int64(timestamp), int64((timestamp-float64(int64(timestamp)))*1e9)), nil
	}
	// Try RFC3339 format
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t, nil
	}
	// Try ISO 8601 format
	if t, err := time.Parse("2006-01-02T15:04:05Z", timeStr); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: %s", timeStr)
}

// getAuthContext extracts authentication information from the request.
func (s *MetricsAPIServer) getAuthContext(r *http.Request) *AuthContext {
	auth := &AuthContext{}
	if user := r.Header.Get("X-Remote-User"); user != "" {
		auth.User = user
	} else if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		auth.User = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if groups := r.Header.Get("X-Remote-Groups"); groups != "" {
		auth.Groups = strings.Split(groups, ",")
	}
	auth.Workspace = r.URL.Query().Get("workspace")
	if auth.Workspace == "" {
		auth.Workspace = r.Header.Get("X-Kcp-Workspace")
	}
	return auth
}

// convertToPrometheusFormat converts internal format to Prometheus instant query format.
func (s *MetricsAPIServer) convertToPrometheusFormat(series []MetricSeries) interface{} {
	result := make([]map[string]interface{}, 0, len(series))
	for _, s := range series {
		for _, value := range s.Values {
			result = append(result, map[string]interface{}{
				"metric": value.Labels,
				"value":  []interface{}{value.Timestamp.Unix(), fmt.Sprintf("%f", value.Value)},
			})
		}
	}
	return map[string]interface{}{"resultType": "vector", "result": result}
}

// convertToPrometheusRangeFormat converts internal format to Prometheus range query format.
func (s *MetricsAPIServer) convertToPrometheusRangeFormat(series []MetricSeries) interface{} {
	result := make([]map[string]interface{}, 0, len(series))
	for _, s := range series {
		values := make([][]interface{}, 0, len(s.Values))
		var metric map[string]string
		for _, value := range s.Values {
			values = append(values, []interface{}{value.Timestamp.Unix(), fmt.Sprintf("%f", value.Value)})
			if metric == nil {
				metric = value.Labels
			}
		}
		if len(values) > 0 {
			result = append(result, map[string]interface{}{"metric": metric, "values": values})
		}
	}
	return map[string]interface{}{"resultType": "matrix", "result": result}
}

// writeJSONResponse writes a JSON response with the specified status code.
func (s *MetricsAPIServer) writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		klog.ErrorS(err, "Failed to encode JSON response")
	}
}

// writeErrorResponse writes a structured error response.
func (s *MetricsAPIServer) writeErrorResponse(w http.ResponseWriter, status int, errorType, message string) {
	response := &MetricResponse{Status: "error", ErrorType: errorType, Error: message}
	s.writeJSONResponse(w, status, response)
}

// writePrometheusError writes a Prometheus-compatible error response.
func (s *MetricsAPIServer) writePrometheusError(w http.ResponseWriter, status int, message string) {
	response := &PrometheusResponse{Status: "error", Error: message}
	s.writeJSONResponse(w, status, response)
}

// authMiddleware provides authentication and authorization middleware.
func (s *MetricsAPIServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for health checks and OpenAPI spec
		if r.URL.Path == "/healthz" || r.URL.Path == "/openapi.json" {
			next.ServeHTTP(w, r)
			return
		}
		
		auth := s.getAuthContext(r)
		if auth.User == "" {
			s.writeErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests for observability.
func (s *MetricsAPIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapper := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapper, r)
		
		klog.InfoS("HTTP request completed",
			"method", r.Method, "path", r.URL.Path, "status", wrapper.statusCode,
			"duration", time.Since(start), "user", s.getAuthContext(r).User)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}