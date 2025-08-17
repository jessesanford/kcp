# Virtual Workspace Endpoints Implementation Instructions

## Overview
This branch implements the endpoint exposure layer for the Virtual Workspace, providing REST API endpoints that expose unified views of resources across multiple clusters. It builds on the VW core infrastructure to deliver accessible API endpoints.

**Branch**: `feature/tmc-completion/p1w2-vw-endpoints`  
**Estimated Lines**: 600 lines  
**Wave**: 3  
**Dependencies**: p1w2-vw-core must be complete  

## Dependencies

### Required Before Starting
- Phase 0 APIs complete
- p1w2-vw-core merged (provides VW infrastructure)
- Virtual workspace framework available

### Blocks These Features
- None directly, but enhances API accessibility

## Files to Create/Modify

### Primary Implementation Files (600 lines total)

1. **pkg/virtual/syncer/endpoints/endpoint_manager.go** (180 lines)
   - Main endpoint management logic
   - Registration and lifecycle
   - Health checking

2. **pkg/virtual/syncer/endpoints/rest_handler.go** (150 lines)
   - REST request handling
   - Response formatting
   - Error handling

3. **pkg/virtual/syncer/endpoints/router.go** (120 lines)
   - Request routing logic
   - Path matching
   - Method dispatch

4. **pkg/virtual/syncer/endpoints/authenticator.go** (80 lines)
   - Authentication handling
   - Token validation
   - Identity extraction

5. **pkg/virtual/syncer/endpoints/metrics.go** (70 lines)
   - Endpoint metrics collection
   - Performance monitoring
   - Request tracking

### Test Files (not counted in line limit)

1. **pkg/virtual/syncer/endpoints/endpoint_manager_test.go**
2. **pkg/virtual/syncer/endpoints/rest_handler_test.go**
3. **pkg/virtual/syncer/endpoints/router_test.go**

## Step-by-Step Implementation Guide

### Step 1: Setup Endpoint Manager (Hour 1-2)

```go
// pkg/virtual/syncer/endpoints/endpoint_manager.go
package endpoints

import (
    "context"
    "fmt"
    "net/http"
    "sync"
    
    "github.com/kcp-dev/kcp/pkg/virtual/syncer"
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    "github.com/kcp-dev/logicalcluster/v3"
    
    "k8s.io/apiserver/pkg/endpoints/handlers"
    "k8s.io/apiserver/pkg/endpoints/request"
    "k8s.io/klog/v2"
)

// EndpointManager manages virtual workspace endpoints
type EndpointManager struct {
    // Core components
    virtualWorkspace *syncer.VirtualWorkspace
    router          *Router
    authenticator   *Authenticator
    
    // Endpoints
    endpoints       map[string]*Endpoint
    endpointsMutex  sync.RWMutex
    
    // Configuration
    config          *EndpointConfig
    
    // Health and metrics
    healthChecker   *HealthChecker
    metrics         *EndpointMetrics
}

// EndpointConfig holds endpoint configuration
type EndpointConfig struct {
    // Base path for endpoints
    BasePath string
    
    // Logical cluster
    LogicalCluster logicalcluster.Name
    
    // Authentication required
    RequireAuth bool
    
    // Rate limiting
    RateLimitQPS   int
    RateLimitBurst int
    
    // Timeout configuration
    RequestTimeout time.Duration
}

// Endpoint represents a single API endpoint
type Endpoint struct {
    // Path of the endpoint
    Path string
    
    // HTTP methods supported
    Methods []string
    
    // Handler function
    Handler http.HandlerFunc
    
    // Resource type
    ResourceType string
    
    // Metrics
    RequestCount int64
    ErrorCount   int64
    
    // Health status
    Healthy bool
}

// NewEndpointManager creates a new endpoint manager
func NewEndpointManager(vw *syncer.VirtualWorkspace, config *EndpointConfig) (*EndpointManager, error) {
    if vw == nil {
        return nil, fmt.Errorf("virtual workspace is required")
    }
    
    if config == nil {
        config = &EndpointConfig{
            BasePath:       "/apis/tmc.kcp.dev/v1alpha1",
            RequireAuth:    true,
            RateLimitQPS:   100,
            RateLimitBurst: 200,
            RequestTimeout: 30 * time.Second,
        }
    }
    
    em := &EndpointManager{
        virtualWorkspace: vw,
        config:          config,
        endpoints:       make(map[string]*Endpoint),
        router:          NewRouter(config.BasePath),
        authenticator:   NewAuthenticator(config.RequireAuth),
        healthChecker:   NewHealthChecker(),
        metrics:         NewEndpointMetrics(),
    }
    
    // Register default endpoints
    if err := em.registerDefaultEndpoints(); err != nil {
        return nil, fmt.Errorf("failed to register default endpoints: %w", err)
    }
    
    return em, nil
}

// ServeHTTP handles incoming HTTP requests
func (em *EndpointManager) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()
    
    // Start metrics
    start := time.Now()
    em.metrics.RecordRequest(req.URL.Path, req.Method)
    
    // Apply timeout
    ctx, cancel := context.WithTimeout(ctx, em.config.RequestTimeout)
    defer cancel()
    req = req.WithContext(ctx)
    
    // Authenticate if required
    if em.config.RequireAuth {
        user, err := em.authenticator.Authenticate(req)
        if err != nil {
            em.metrics.RecordError(req.URL.Path, "authentication_failed")
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        ctx = request.WithUser(ctx, user)
        req = req.WithContext(ctx)
    }
    
    // Route request
    endpoint, params, err := em.router.Route(req)
    if err != nil {
        em.metrics.RecordError(req.URL.Path, "routing_failed")
        http.Error(w, "Not Found", http.StatusNotFound)
        return
    }
    
    // Check endpoint health
    if !endpoint.Healthy {
        em.metrics.RecordError(req.URL.Path, "unhealthy_endpoint")
        http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
        return
    }
    
    // Add request info to context
    ctx = withRequestParams(ctx, params)
    req = req.WithContext(ctx)
    
    // Execute handler
    endpoint.Handler(w, req)
    
    // Record metrics
    em.metrics.RecordLatency(req.URL.Path, time.Since(start))
    endpoint.RequestCount++
}

// registerDefaultEndpoints registers the default API endpoints
func (em *EndpointManager) registerDefaultEndpoints() error {
    // WorkloadPlacement endpoints
    if err := em.RegisterEndpoint(&Endpoint{
        Path:         "/workloadplacements",
        Methods:      []string{"GET", "POST"},
        Handler:      em.handleWorkloadPlacementList,
        ResourceType: "workloadplacements",
        Healthy:      true,
    }); err != nil {
        return err
    }
    
    if err := em.RegisterEndpoint(&Endpoint{
        Path:         "/workloadplacements/{name}",
        Methods:      []string{"GET", "PUT", "DELETE", "PATCH"},
        Handler:      em.handleWorkloadPlacement,
        ResourceType: "workloadplacements",
        Healthy:      true,
    }); err != nil {
        return err
    }
    
    // SyncTarget endpoints (read-only)
    if err := em.RegisterEndpoint(&Endpoint{
        Path:         "/synctargets",
        Methods:      []string{"GET"},
        Handler:      em.handleSyncTargetList,
        ResourceType: "synctargets",
        Healthy:      true,
    }); err != nil {
        return err
    }
    
    if err := em.RegisterEndpoint(&Endpoint{
        Path:         "/synctargets/{name}",
        Methods:      []string{"GET"},
        Handler:      em.handleSyncTarget,
        ResourceType: "synctargets",
        Healthy:      true,
    }); err != nil {
        return err
    }
    
    // ClusterRegistration endpoints (read-only)
    if err := em.RegisterEndpoint(&Endpoint{
        Path:         "/clusterregistrations",
        Methods:      []string{"GET"},
        Handler:      em.handleClusterRegistrationList,
        ResourceType: "clusterregistrations",
        Healthy:      true,
    }); err != nil {
        return err
    }
    
    // Health endpoint
    if err := em.RegisterEndpoint(&Endpoint{
        Path:     "/healthz",
        Methods:  []string{"GET"},
        Handler:  em.handleHealth,
        Healthy:  true,
    }); err != nil {
        return err
    }
    
    // Metrics endpoint
    if err := em.RegisterEndpoint(&Endpoint{
        Path:     "/metrics",
        Methods:  []string{"GET"},
        Handler:  em.handleMetrics,
        Healthy:  true,
    }); err != nil {
        return err
    }
    
    return nil
}

// RegisterEndpoint registers a new endpoint
func (em *EndpointManager) RegisterEndpoint(endpoint *Endpoint) error {
    em.endpointsMutex.Lock()
    defer em.endpointsMutex.Unlock()
    
    // Validate endpoint
    if endpoint.Path == "" {
        return fmt.Errorf("endpoint path is required")
    }
    
    if len(endpoint.Methods) == 0 {
        return fmt.Errorf("at least one method is required")
    }
    
    if endpoint.Handler == nil {
        return fmt.Errorf("handler is required")
    }
    
    // Register with router
    for _, method := range endpoint.Methods {
        if err := em.router.AddRoute(method, endpoint.Path, endpoint); err != nil {
            return fmt.Errorf("failed to add route: %w", err)
        }
    }
    
    // Store endpoint
    em.endpoints[endpoint.Path] = endpoint
    
    klog.V(2).Infof("Registered endpoint: %s %v", endpoint.Path, endpoint.Methods)
    
    return nil
}

// Start starts the endpoint manager
func (em *EndpointManager) Start(ctx context.Context) error {
    // Start health checker
    go em.healthChecker.Start(ctx, em.endpoints)
    
    // Start metrics collection
    go em.metrics.Start(ctx)
    
    klog.Info("Endpoint manager started")
    return nil
}

// Stop stops the endpoint manager
func (em *EndpointManager) Stop() error {
    em.endpointsMutex.Lock()
    defer em.endpointsMutex.Unlock()
    
    // Mark all endpoints as unhealthy
    for _, endpoint := range em.endpoints {
        endpoint.Healthy = false
    }
    
    klog.Info("Endpoint manager stopped")
    return nil
}
```

### Step 2: Implement REST Handler (Hour 3-4)

```go
// pkg/virtual/syncer/endpoints/rest_handler.go
package endpoints

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    
    tmcv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/klog/v2"
)

// handleWorkloadPlacementList handles list requests for WorkloadPlacements
func (em *EndpointManager) handleWorkloadPlacementList(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()
    
    switch req.Method {
    case http.MethodGet:
        em.listWorkloadPlacements(ctx, w, req)
    case http.MethodPost:
        em.createWorkloadPlacement(ctx, w, req)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// handleWorkloadPlacement handles requests for individual WorkloadPlacements
func (em *EndpointManager) handleWorkloadPlacement(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()
    params := getRequestParams(ctx)
    name := params["name"]
    
    if name == "" {
        http.Error(w, "Name is required", http.StatusBadRequest)
        return
    }
    
    switch req.Method {
    case http.MethodGet:
        em.getWorkloadPlacement(ctx, w, name)
    case http.MethodPut:
        em.updateWorkloadPlacement(ctx, w, req, name)
    case http.MethodDelete:
        em.deleteWorkloadPlacement(ctx, w, name)
    case http.MethodPatch:
        em.patchWorkloadPlacement(ctx, w, req, name)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// listWorkloadPlacements lists WorkloadPlacements
func (em *EndpointManager) listWorkloadPlacements(ctx context.Context, w http.ResponseWriter, req *http.Request) {
    // Parse query parameters
    labelSelector := req.URL.Query().Get("labelSelector")
    fieldSelector := req.URL.Query().Get("fieldSelector")
    
    // Get list from virtual workspace
    provider := em.virtualWorkspace.GetProvider("workloadplacements")
    if provider == nil {
        http.Error(w, "Provider not found", http.StatusInternalServerError)
        return
    }
    
    options := &metainternalversion.ListOptions{}
    if labelSelector != "" {
        selector, err := labels.Parse(labelSelector)
        if err != nil {
            http.Error(w, fmt.Sprintf("Invalid label selector: %v", err), http.StatusBadRequest)
            return
        }
        options.LabelSelector = selector
    }
    
    list, err := provider.List(ctx, options)
    if err != nil {
        klog.Errorf("Failed to list WorkloadPlacements: %v", err)
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
    
    // Convert to JSON
    em.writeJSON(w, list, http.StatusOK)
}

// getWorkloadPlacement gets a single WorkloadPlacement
func (em *EndpointManager) getWorkloadPlacement(ctx context.Context, w http.ResponseWriter, name string) {
    provider := em.virtualWorkspace.GetProvider("workloadplacements")
    if provider == nil {
        http.Error(w, "Provider not found", http.StatusInternalServerError)
        return
    }
    
    obj, err := provider.Get(ctx, name, &metav1.GetOptions{})
    if err != nil {
        if apierrors.IsNotFound(err) {
            http.Error(w, "Not found", http.StatusNotFound)
        } else {
            klog.Errorf("Failed to get WorkloadPlacement %s: %v", name, err)
            http.Error(w, "Internal server error", http.StatusInternalServerError)
        }
        return
    }
    
    em.writeJSON(w, obj, http.StatusOK)
}

// createWorkloadPlacement creates a new WorkloadPlacement
func (em *EndpointManager) createWorkloadPlacement(ctx context.Context, w http.ResponseWriter, req *http.Request) {
    // Read body
    body, err := io.ReadAll(req.Body)
    if err != nil {
        http.Error(w, "Failed to read request body", http.StatusBadRequest)
        return
    }
    
    // Parse WorkloadPlacement
    placement := &tmcv1alpha1.WorkloadPlacement{}
    if err := json.Unmarshal(body, placement); err != nil {
        http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
        return
    }
    
    // Validate
    if placement.Name == "" {
        http.Error(w, "Name is required", http.StatusBadRequest)
        return
    }
    
    // Create via provider
    provider := em.virtualWorkspace.GetProvider("workloadplacements")
    if provider == nil {
        http.Error(w, "Provider not found", http.StatusInternalServerError)
        return
    }
    
    created, err := provider.Create(ctx, placement, nil, &metav1.CreateOptions{})
    if err != nil {
        if apierrors.IsAlreadyExists(err) {
            http.Error(w, "Already exists", http.StatusConflict)
        } else {
            klog.Errorf("Failed to create WorkloadPlacement: %v", err)
            http.Error(w, "Internal server error", http.StatusInternalServerError)
        }
        return
    }
    
    em.writeJSON(w, created, http.StatusCreated)
}

// updateWorkloadPlacement updates a WorkloadPlacement
func (em *EndpointManager) updateWorkloadPlacement(ctx context.Context, w http.ResponseWriter, req *http.Request, name string) {
    // Read body
    body, err := io.ReadAll(req.Body)
    if err != nil {
        http.Error(w, "Failed to read request body", http.StatusBadRequest)
        return
    }
    
    // Parse WorkloadPlacement
    placement := &tmcv1alpha1.WorkloadPlacement{}
    if err := json.Unmarshal(body, placement); err != nil {
        http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
        return
    }
    
    // Ensure name matches
    if placement.Name != name {
        http.Error(w, "Name mismatch", http.StatusBadRequest)
        return
    }
    
    // Update via provider
    provider := em.virtualWorkspace.GetProvider("workloadplacements")
    if provider == nil {
        http.Error(w, "Provider not found", http.StatusInternalServerError)
        return
    }
    
    updater := rest.DefaultUpdatedObjectInfo(placement)
    updated, _, err := provider.Update(ctx, name, updater, nil, nil, false, &metav1.UpdateOptions{})
    if err != nil {
        if apierrors.IsNotFound(err) {
            http.Error(w, "Not found", http.StatusNotFound)
        } else {
            klog.Errorf("Failed to update WorkloadPlacement %s: %v", name, err)
            http.Error(w, "Internal server error", http.StatusInternalServerError)
        }
        return
    }
    
    em.writeJSON(w, updated, http.StatusOK)
}

// deleteWorkloadPlacement deletes a WorkloadPlacement
func (em *EndpointManager) deleteWorkloadPlacement(ctx context.Context, w http.ResponseWriter, name string) {
    provider := em.virtualWorkspace.GetProvider("workloadplacements")
    if provider == nil {
        http.Error(w, "Provider not found", http.StatusInternalServerError)
        return
    }
    
    _, _, err := provider.Delete(ctx, name, nil, &metav1.DeleteOptions{})
    if err != nil {
        if apierrors.IsNotFound(err) {
            http.Error(w, "Not found", http.StatusNotFound)
        } else {
            klog.Errorf("Failed to delete WorkloadPlacement %s: %v", name, err)
            http.Error(w, "Internal server error", http.StatusInternalServerError)
        }
        return
    }
    
    w.WriteHeader(http.StatusNoContent)
}

// writeJSON writes a JSON response
func (em *EndpointManager) writeJSON(w http.ResponseWriter, obj runtime.Object, statusCode int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    
    encoder := json.NewEncoder(w)
    encoder.SetIndent("", "  ")
    
    if err := encoder.Encode(obj); err != nil {
        klog.Errorf("Failed to encode response: %v", err)
    }
}

// handleHealth handles health check requests
func (em *EndpointManager) handleHealth(w http.ResponseWriter, req *http.Request) {
    health := em.healthChecker.GetHealth()
    
    status := http.StatusOK
    if !health.Healthy {
        status = http.StatusServiceUnavailable
    }
    
    em.writeJSON(w, health, status)
}

// handleMetrics handles metrics requests
func (em *EndpointManager) handleMetrics(w http.ResponseWriter, req *http.Request) {
    metrics := em.metrics.GetMetrics()
    em.writeJSON(w, metrics, http.StatusOK)
}
```

### Step 3: Implement Router (Hour 5)

```go
// pkg/virtual/syncer/endpoints/router.go
package endpoints

import (
    "fmt"
    "net/http"
    "strings"
    "sync"
)

// Router handles request routing for endpoints
type Router struct {
    basePath string
    routes   map[string]map[string]*Route
    mutex    sync.RWMutex
}

// Route represents a single route
type Route struct {
    Pattern  string
    Endpoint *Endpoint
    Params   map[string]int
}

// NewRouter creates a new router
func NewRouter(basePath string) *Router {
    return &Router{
        basePath: basePath,
        routes:   make(map[string]map[string]*Route),
    }
}

// AddRoute adds a new route
func (r *Router) AddRoute(method, pattern string, endpoint *Endpoint) error {
    r.mutex.Lock()
    defer r.mutex.Unlock()
    
    // Ensure method map exists
    if _, exists := r.routes[method]; !exists {
        r.routes[method] = make(map[string]*Route)
    }
    
    // Parse pattern for parameters
    route := &Route{
        Pattern:  pattern,
        Endpoint: endpoint,
        Params:   make(map[string]int),
    }
    
    // Extract parameter positions
    parts := strings.Split(pattern, "/")
    for i, part := range parts {
        if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
            paramName := strings.Trim(part, "{}")
            route.Params[paramName] = i
        }
    }
    
    // Store route
    r.routes[method][pattern] = route
    
    return nil
}

// Route finds the matching endpoint for a request
func (r *Router) Route(req *http.Request) (*Endpoint, map[string]string, error) {
    r.mutex.RLock()
    defer r.mutex.RUnlock()
    
    // Strip base path
    path := req.URL.Path
    if r.basePath != "" && strings.HasPrefix(path, r.basePath) {
        path = strings.TrimPrefix(path, r.basePath)
    }
    
    // Get method routes
    methodRoutes, exists := r.routes[req.Method]
    if !exists {
        return nil, nil, fmt.Errorf("no routes for method %s", req.Method)
    }
    
    // Try exact match first
    if route, exists := methodRoutes[path]; exists {
        return route.Endpoint, nil, nil
    }
    
    // Try pattern matching
    for pattern, route := range methodRoutes {
        if params := r.matchPattern(path, pattern); params != nil {
            return route.Endpoint, params, nil
        }
    }
    
    return nil, nil, fmt.Errorf("no matching route for %s %s", req.Method, path)
}

// matchPattern checks if a path matches a pattern and extracts parameters
func (r *Router) matchPattern(path, pattern string) map[string]string {
    pathParts := strings.Split(strings.Trim(path, "/"), "/")
    patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
    
    if len(pathParts) != len(patternParts) {
        return nil
    }
    
    params := make(map[string]string)
    
    for i, patternPart := range patternParts {
        if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
            // Parameter
            paramName := strings.Trim(patternPart, "{}")
            params[paramName] = pathParts[i]
        } else if patternPart != pathParts[i] {
            // Mismatch
            return nil
        }
    }
    
    return params
}

// GetRoutes returns all registered routes
func (r *Router) GetRoutes() map[string][]string {
    r.mutex.RLock()
    defer r.mutex.RUnlock()
    
    routes := make(map[string][]string)
    
    for method, methodRoutes := range r.routes {
        for pattern := range methodRoutes {
            key := fmt.Sprintf("%s %s", method, pattern)
            routes[key] = append(routes[key], pattern)
        }
    }
    
    return routes
}

// Clear removes all routes
func (r *Router) Clear() {
    r.mutex.Lock()
    defer r.mutex.Unlock()
    
    r.routes = make(map[string]map[string]*Route)
}

// contextKey for request parameters
type contextKey string

const requestParamsKey contextKey = "request-params"

// withRequestParams adds request parameters to context
func withRequestParams(ctx context.Context, params map[string]string) context.Context {
    return context.WithValue(ctx, requestParamsKey, params)
}

// getRequestParams retrieves request parameters from context
func getRequestParams(ctx context.Context) map[string]string {
    params, _ := ctx.Value(requestParamsKey).(map[string]string)
    if params == nil {
        params = make(map[string]string)
    }
    return params
}
```

### Step 4: Implement Authentication (Hour 6)

```go
// pkg/virtual/syncer/endpoints/authenticator.go
package endpoints

import (
    "fmt"
    "net/http"
    "strings"
    
    "k8s.io/apiserver/pkg/authentication/user"
    "k8s.io/apiserver/pkg/endpoints/request"
)

// Authenticator handles request authentication
type Authenticator struct {
    required bool
    tokens   map[string]*user.DefaultInfo
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(required bool) *Authenticator {
    return &Authenticator{
        required: required,
        tokens:   make(map[string]*user.DefaultInfo),
    }
}

// Authenticate authenticates a request
func (a *Authenticator) Authenticate(req *http.Request) (user.Info, error) {
    if !a.required {
        // Return anonymous user if auth not required
        return &user.DefaultInfo{
            Name:   "anonymous",
            Groups: []string{"system:unauthenticated"},
        }, nil
    }
    
    // Extract token from header
    token := a.extractToken(req)
    if token == "" {
        return nil, fmt.Errorf("no authentication token provided")
    }
    
    // Validate token
    userInfo, err := a.validateToken(token)
    if err != nil {
        return nil, fmt.Errorf("invalid token: %w", err)
    }
    
    return userInfo, nil
}

// extractToken extracts the token from request
func (a *Authenticator) extractToken(req *http.Request) string {
    // Check Authorization header
    auth := req.Header.Get("Authorization")
    if auth != "" {
        parts := strings.SplitN(auth, " ", 2)
        if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
            return parts[1]
        }
    }
    
    // Check X-Auth-Token header
    if token := req.Header.Get("X-Auth-Token"); token != "" {
        return token
    }
    
    // Check query parameter (for websocket upgrades)
    if token := req.URL.Query().Get("token"); token != "" {
        return token
    }
    
    return ""
}

// validateToken validates a token and returns user info
func (a *Authenticator) validateToken(token string) (user.Info, error) {
    // Check cache
    if userInfo, exists := a.tokens[token]; exists {
        return userInfo, nil
    }
    
    // In production, this would validate against an auth service
    // For now, create a test user
    userInfo := &user.DefaultInfo{
        Name:   "test-user",
        UID:    "test-uid",
        Groups: []string{"system:authenticated"},
    }
    
    // Cache the result
    a.tokens[token] = userInfo
    
    return userInfo, nil
}

// AddToken adds a token for testing
func (a *Authenticator) AddToken(token string, userInfo *user.DefaultInfo) {
    a.tokens[token] = userInfo
}

// ClearTokens clears all cached tokens
func (a *Authenticator) ClearTokens() {
    a.tokens = make(map[string]*user.DefaultInfo)
}
```

### Step 5: Implement Metrics Collection (Hour 7)

```go
// pkg/virtual/syncer/endpoints/metrics.go
package endpoints

import (
    "context"
    "sync"
    "time"
    
    "k8s.io/component-base/metrics"
)

// EndpointMetrics collects metrics for endpoints
type EndpointMetrics struct {
    requestCount   map[string]int64
    errorCount     map[string]int64
    latencies      map[string][]time.Duration
    mutex          sync.RWMutex
}

// MetricsSummary represents a metrics summary
type MetricsSummary struct {
    TotalRequests   int64                     `json:"totalRequests"`
    TotalErrors     int64                     `json:"totalErrors"`
    EndpointMetrics map[string]*EndpointStats `json:"endpoints"`
}

// EndpointStats represents metrics for a single endpoint
type EndpointStats struct {
    Requests      int64         `json:"requests"`
    Errors        int64         `json:"errors"`
    AvgLatency    time.Duration `json:"avgLatency"`
    MaxLatency    time.Duration `json:"maxLatency"`
    MinLatency    time.Duration `json:"minLatency"`
}

// NewEndpointMetrics creates new metrics collector
func NewEndpointMetrics() *EndpointMetrics {
    return &EndpointMetrics{
        requestCount: make(map[string]int64),
        errorCount:   make(map[string]int64),
        latencies:    make(map[string][]time.Duration),
    }
}

// RecordRequest records a request
func (m *EndpointMetrics) RecordRequest(path, method string) {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    key := method + " " + path
    m.requestCount[key]++
}

// RecordError records an error
func (m *EndpointMetrics) RecordError(path, errorType string) {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    key := path + ":" + errorType
    m.errorCount[key]++
}

// RecordLatency records request latency
func (m *EndpointMetrics) RecordLatency(path string, latency time.Duration) {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    m.latencies[path] = append(m.latencies[path], latency)
    
    // Keep only last 100 samples
    if len(m.latencies[path]) > 100 {
        m.latencies[path] = m.latencies[path][1:]
    }
}

// GetMetrics returns current metrics
func (m *EndpointMetrics) GetMetrics() *MetricsSummary {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    
    summary := &MetricsSummary{
        EndpointMetrics: make(map[string]*EndpointStats),
    }
    
    // Calculate totals
    for _, count := range m.requestCount {
        summary.TotalRequests += count
    }
    
    for _, count := range m.errorCount {
        summary.TotalErrors += count
    }
    
    // Calculate per-endpoint stats
    for path, latencies := range m.latencies {
        stats := &EndpointStats{
            Requests: m.requestCount[path],
            Errors:   m.errorCount[path],
        }
        
        if len(latencies) > 0 {
            var total time.Duration
            stats.MinLatency = latencies[0]
            stats.MaxLatency = latencies[0]
            
            for _, l := range latencies {
                total += l
                if l < stats.MinLatency {
                    stats.MinLatency = l
                }
                if l > stats.MaxLatency {
                    stats.MaxLatency = l
                }
            }
            
            stats.AvgLatency = total / time.Duration(len(latencies))
        }
        
        summary.EndpointMetrics[path] = stats
    }
    
    return summary
}

// Start starts metrics collection
func (m *EndpointMetrics) Start(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.flush()
        }
    }
}

// flush resets short-term metrics
func (m *EndpointMetrics) flush() {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    // Reset latencies to prevent unbounded growth
    for path := range m.latencies {
        if len(m.latencies[path]) > 10 {
            m.latencies[path] = m.latencies[path][len(m.latencies[path])-10:]
        }
    }
}
```

## Testing Requirements

### Unit Tests

1. **Endpoint Manager Tests**
   - Test endpoint registration
   - Test request handling
   - Test authentication flow
   - Test health checking

2. **REST Handler Tests**
   - Test CRUD operations
   - Test error handling
   - Test response formatting
   - Test query parameters

3. **Router Tests**
   - Test route matching
   - Test parameter extraction
   - Test method dispatch
   - Test pattern matching

4. **Authentication Tests**
   - Test token extraction
   - Test token validation
   - Test anonymous access
   - Test authorization

5. **Metrics Tests**
   - Test metric collection
   - Test aggregation
   - Test flushing

### Integration Tests

1. **End-to-End API Tests**
   - Test complete request flow
   - Test authentication and authorization
   - Test error responses
   - Test metrics collection

2. **Performance Tests**
   - Test request throughput
   - Test latency
   - Test concurrent requests

## KCP Patterns to Follow

### REST API Standards
- Follow Kubernetes API conventions
- Implement proper error responses
- Support standard query parameters
- Provide OpenAPI documentation

### Authentication/Authorization
- Support bearer tokens
- Implement RBAC checks
- Handle anonymous requests appropriately

### Metrics and Monitoring
- Expose Prometheus metrics
- Track request latencies
- Monitor error rates
- Provide health endpoints

## Integration Points

### With VW Core (p1w2-vw-core)
- Uses virtual workspace for backend
- Leverages providers for data access
- Shares transformation logic

### With VW Discovery (p1w2-vw-discovery)
- Coordinates API exposure
- Shares endpoint information
- Provides discovery data

## Validation Checklist

### Before Commit
- [ ] All files created as specified
- [ ] Line count under 600 (run `/workspaces/kcp-shared-tools/tmc-pr-line-counter.sh`)
- [ ] All tests passing (`make test`)
- [ ] Linting passes (`make lint`)
- [ ] API documentation updated

### Functionality Complete
- [ ] Endpoint manager operational
- [ ] All REST operations work
- [ ] Routing functions correctly
- [ ] Authentication working
- [ ] Metrics collected

### Integration Ready
- [ ] Integrates with VW core
- [ ] Endpoints accessible
- [ ] Health checks pass
- [ ] Metrics exposed

### Documentation Complete
- [ ] API endpoints documented
- [ ] Authentication documented
- [ ] Query parameters documented
- [ ] Error responses documented

## Commit Message Template
```
feat(endpoints): implement Virtual Workspace endpoint exposure

- Add endpoint manager with registration and lifecycle
- Implement REST handlers for TMC resources
- Add request routing with parameter extraction
- Implement authentication and authorization
- Add metrics collection and monitoring
- Ensure proper error handling throughout

Part of TMC Phase 1 Wave 3 implementation
Depends on: p1w2-vw-core
```

## Next Steps
After this branch is complete:
1. Virtual Workspace APIs will be fully accessible
2. Clients can interact with TMC resources
3. Monitoring and metrics available