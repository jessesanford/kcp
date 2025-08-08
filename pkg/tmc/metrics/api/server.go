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

// Package api provides REST endpoints for TMC metrics retrieval with query parameter
// support, response formatting (JSON, Prometheus), pagination, and authentication hooks.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// MetricQuery represents query parameters for metrics retrieval.
type MetricQuery struct {
	MetricName string            `json:"metricName"`
	StartTime  *time.Time        `json:"startTime,omitempty"`
	EndTime    *time.Time        `json:"endTime,omitempty"`
	Step       *time.Duration    `json:"step,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Workspace  string            `json:"workspace,omitempty"`
	Cluster    string            `json:"cluster,omitempty"`
	Limit      int               `json:"limit,omitempty"`
	Offset     int               `json:"offset,omitempty"`
}

// MetricValue represents a single metric value at a specific time.
type MetricValue struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels"`
}

// MetricSeries represents a time series of metric values.
type MetricSeries struct {
	Name   string        `json:"name"`
	Help   string        `json:"help"`
	Type   string        `json:"type"`
	Values []MetricValue `json:"values"`
}

// MetricResponse represents the response from a metrics query.
type MetricResponse struct {
	Status     string          `json:"status"`
	Data       []MetricSeries  `json:"data"`
	ErrorType  string          `json:"errorType,omitempty"`
	Error      string          `json:"error,omitempty"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// PaginationInfo provides pagination metadata.
type PaginationInfo struct {
	Total   int  `json:"total"`
	Offset  int  `json:"offset"`
	Limit   int  `json:"limit"`
	HasNext bool `json:"hasNext"`
	HasPrev bool `json:"hasPrev"`
}

// PrometheusResponse represents a Prometheus-compatible query response.
type PrometheusResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
	Error  string      `json:"error,omitempty"`
}

// AuthContext contains authentication information for metrics access.
type AuthContext struct {
	User        string
	Groups      []string
	Workspace   string
	Permissions []string
}

// MetricsStore defines the interface for storing and retrieving metrics.
type MetricsStore interface {
	Query(ctx context.Context, query *MetricQuery) ([]MetricSeries, error)
	GetMetricNames(ctx context.Context) ([]string, error)
}

// Authorizer defines the interface for authorizing metrics access.
type Authorizer interface {
	Authorize(ctx context.Context, auth *AuthContext, query *MetricQuery) error
}

// MetricsAPIServer provides REST endpoints for TMC metrics retrieval.
type MetricsAPIServer struct {
	server     *http.Server
	store      MetricsStore
	authorizer Authorizer
	router     *mux.Router
}

// NewMetricsAPIServer creates a new metrics API server.
func NewMetricsAPIServer(port int, store MetricsStore, authorizer Authorizer) *MetricsAPIServer {
	router := mux.NewRouter()
	server := &MetricsAPIServer{
		server:     &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: router},
		store:      store,
		authorizer: authorizer,
		router:     router,
	}
	server.setupRoutes()
	return server
}

// Start starts the metrics API server with graceful shutdown support.
func (s *MetricsAPIServer) Start(ctx context.Context) error {
	if !utilfeature.DefaultFeatureGate.Enabled(features.TMCMetricsAPI) {
		klog.InfoS("TMC Metrics API is disabled by feature flag")
		return nil
	}
	
	klog.InfoS("Starting TMC Metrics API server", "address", s.server.Addr)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.ErrorS(err, "TMC Metrics API server failed")
		}
	}()
	
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	klog.InfoS("Shutting down TMC Metrics API server")
	return s.server.Shutdown(shutdownCtx)
}

// setupRoutes configures HTTP routes for the metrics API.
func (s *MetricsAPIServer) setupRoutes() {
	// TMC-specific endpoints
	s.router.HandleFunc("/api/v1/metrics/query", s.handleQuery).Methods("GET", "POST")
	s.router.HandleFunc("/api/v1/metrics/query_range", s.handleQueryRange).Methods("GET", "POST")
	s.router.HandleFunc("/api/v1/metrics/names", s.handleMetricNames).Methods("GET")
	
	// Prometheus-compatible endpoints
	s.router.HandleFunc("/api/v1/query", s.handlePrometheusQuery).Methods("GET", "POST")
	s.router.HandleFunc("/api/v1/query_range", s.handlePrometheusQueryRange).Methods("GET", "POST")
	s.router.HandleFunc("/api/v1/label/values", s.handleLabelValues).Methods("GET")
	
	// Health and documentation
	s.router.HandleFunc("/healthz", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/openapi.json", s.handleOpenAPISpec).Methods("GET")
	
	// Authentication and logging middleware
	s.router.Use(s.authMiddleware)
	s.router.Use(s.loggingMiddleware)
}

// handleQuery handles TMC metrics queries with full JSON response format.
func (s *MetricsAPIServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query, err := s.parseQuery(r)
	if err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "bad_data", err.Error())
		return
	}
	
	auth := s.getAuthContext(r)
	if err := s.authorizer.Authorize(ctx, auth, query); err != nil {
		s.writeErrorResponse(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	
	series, err := s.store.Query(ctx, query)
	if err != nil {
		klog.ErrorS(err, "Failed to execute metrics query")
		s.writeErrorResponse(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	
	response := &MetricResponse{Status: "success", Data: series}
	if query.Limit > 0 {
		response.Pagination = &PaginationInfo{
			Total: len(series), Offset: query.Offset, Limit: query.Limit,
			HasNext: len(series) == query.Limit, HasPrev: query.Offset > 0,
		}
	}
	s.writeJSONResponse(w, http.StatusOK, response)
}

// handleQueryRange handles time range queries with expanded time series data.
func (s *MetricsAPIServer) handleQueryRange(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query, err := s.parseQueryRange(r)
	if err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "bad_data", err.Error())
		return
	}
	
	auth := s.getAuthContext(r)
	if err := s.authorizer.Authorize(ctx, auth, query); err != nil {
		s.writeErrorResponse(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	
	series, err := s.store.Query(ctx, query)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	
	s.writeJSONResponse(w, http.StatusOK, &MetricResponse{Status: "success", Data: series})
}

// handlePrometheusQuery handles Prometheus-compatible instant queries.
func (s *MetricsAPIServer) handlePrometheusQuery(w http.ResponseWriter, r *http.Request) {
	query, err := s.parsePrometheusQuery(r)
	if err != nil {
		s.writePrometheusError(w, http.StatusBadRequest, err.Error())
		return
	}
	
	ctx := r.Context()
	auth := s.getAuthContext(r)
	if err := s.authorizer.Authorize(ctx, auth, query); err != nil {
		s.writePrometheusError(w, http.StatusForbidden, err.Error())
		return
	}
	
	series, err := s.store.Query(ctx, query)
	if err != nil {
		s.writePrometheusError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	promData := s.convertToPrometheusFormat(series)
	s.writeJSONResponse(w, http.StatusOK, &PrometheusResponse{Status: "success", Data: promData})
}

// handlePrometheusQueryRange handles Prometheus-compatible range queries.
func (s *MetricsAPIServer) handlePrometheusQueryRange(w http.ResponseWriter, r *http.Request) {
	query, err := s.parsePrometheusQueryRange(r)
	if err != nil {
		s.writePrometheusError(w, http.StatusBadRequest, err.Error())
		return
	}
	
	ctx := r.Context()
	auth := s.getAuthContext(r)
	if err := s.authorizer.Authorize(ctx, auth, query); err != nil {
		s.writePrometheusError(w, http.StatusForbidden, err.Error())
		return
	}
	
	series, err := s.store.Query(ctx, query)
	if err != nil {
		s.writePrometheusError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	promData := s.convertToPrometheusRangeFormat(series)
	s.writeJSONResponse(w, http.StatusOK, &PrometheusResponse{Status: "success", Data: promData})
}

// handleMetricNames returns available metric names.
func (s *MetricsAPIServer) handleMetricNames(w http.ResponseWriter, r *http.Request) {
	names, err := s.store.GetMetricNames(r.Context())
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{"status": "success", "data": names})
}

// handleLabelValues returns available label values for Prometheus compatibility.
func (s *MetricsAPIServer) handleLabelValues(w http.ResponseWriter, r *http.Request) {
	values := []string{"cluster", "workspace", "controller", "workload"}
	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{"status": "success", "data": values})
}

// handleHealth provides a simple health check endpoint.
func (s *MetricsAPIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleOpenAPISpec serves the OpenAPI specification for the API.
func (s *MetricsAPIServer) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title": "TMC Metrics API", "version": "v1",
			"description": "REST API for querying TMC cluster metrics with filtering, pagination, and Prometheus compatibility",
		},
		"paths": map[string]interface{}{
			"/api/v1/metrics/query": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "Query metrics with optional filtering and pagination",
					"parameters": []map[string]interface{}{
						{"name": "metric", "in": "query", "required": true, "schema": map[string]string{"type": "string"}},
						{"name": "start", "in": "query", "schema": map[string]string{"type": "string"}},
						{"name": "end", "in": "query", "schema": map[string]string{"type": "string"}},
						{"name": "workspace", "in": "query", "schema": map[string]string{"type": "string"}},
						{"name": "cluster", "in": "query", "schema": map[string]string{"type": "string"}},
						{"name": "limit", "in": "query", "schema": map[string]string{"type": "integer"}},
						{"name": "offset", "in": "query", "schema": map[string]string{"type": "integer"}},
					},
				},
			},
			"/api/v1/query": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "Prometheus-compatible instant query",
					"parameters": []map[string]interface{}{
						{"name": "query", "in": "query", "required": true, "schema": map[string]string{"type": "string"}},
						{"name": "time", "in": "query", "schema": map[string]string{"type": "string"}},
					},
				},
			},
		},
	}
	s.writeJSONResponse(w, http.StatusOK, spec)
}

// Placeholder methods - these will be implemented in the helpers package
func (s *MetricsAPIServer) parseQuery(r *http.Request) (*MetricQuery, error) {
	// TODO: This will be implemented in the helpers package
	return &MetricQuery{}, fmt.Errorf("not implemented - parseQuery")
}

func (s *MetricsAPIServer) parseQueryRange(r *http.Request) (*MetricQuery, error) {
	// TODO: This will be implemented in the helpers package
	return &MetricQuery{}, fmt.Errorf("not implemented - parseQueryRange")
}

func (s *MetricsAPIServer) parsePrometheusQuery(r *http.Request) (*MetricQuery, error) {
	// TODO: This will be implemented in the helpers package
	return &MetricQuery{}, fmt.Errorf("not implemented - parsePrometheusQuery")
}

func (s *MetricsAPIServer) parsePrometheusQueryRange(r *http.Request) (*MetricQuery, error) {
	// TODO: This will be implemented in the helpers package
	return &MetricQuery{}, fmt.Errorf("not implemented - parsePrometheusQueryRange")
}

func (s *MetricsAPIServer) getAuthContext(r *http.Request) *AuthContext {
	// TODO: This will be implemented in the helpers package
	return &AuthContext{}
}

func (s *MetricsAPIServer) convertToPrometheusFormat(series []MetricSeries) interface{} {
	// TODO: This will be implemented in the helpers package
	return map[string]interface{}{"result": []interface{}{}}
}

func (s *MetricsAPIServer) convertToPrometheusRangeFormat(series []MetricSeries) interface{} {
	// TODO: This will be implemented in the helpers package
	return map[string]interface{}{"result": []interface{}{}}
}

func (s *MetricsAPIServer) writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	// TODO: This will be implemented in the helpers package
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		klog.ErrorS(err, "Failed to encode JSON response")
	}
}

func (s *MetricsAPIServer) writeErrorResponse(w http.ResponseWriter, status int, errorType, message string) {
	// TODO: This will be implemented in the helpers package
	response := &MetricResponse{Status: "error", ErrorType: errorType, Error: message}
	s.writeJSONResponse(w, status, response)
}

func (s *MetricsAPIServer) writePrometheusError(w http.ResponseWriter, status int, message string) {
	// TODO: This will be implemented in the helpers package
	response := &PrometheusResponse{Status: "error", Error: message}
	s.writeJSONResponse(w, status, response)
}

func (s *MetricsAPIServer) authMiddleware(next http.Handler) http.Handler {
	// TODO: This will be implemented in the helpers package
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for health checks and OpenAPI spec
		if r.URL.Path == "/healthz" || r.URL.Path == "/openapi.json" {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *MetricsAPIServer) loggingMiddleware(next http.Handler) http.Handler {
	// TODO: This will be implemented in the helpers package
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		klog.InfoS("HTTP request completed",
			"method", r.Method, "path", r.URL.Path, 
			"duration", time.Since(start))
	})
}