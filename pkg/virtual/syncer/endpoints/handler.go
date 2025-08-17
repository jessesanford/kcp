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

package endpoints

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcpinformers "github.com/kcp-dev/kcp/sdk/client/informers/externalversions"
	"github.com/kcp-dev/logicalcluster/v3"
)

// Handler provides the main HTTP handler for virtual workspace endpoints.
// It manages request routing, authentication, authorization, and response aggregation
// across multiple clusters in the TMC environment.
type Handler struct {
	// clusterClient provides cluster-aware access to KCP resources
	clusterClient cluster.ClusterInterface
	
	// informerFactory provides shared informers for the virtual workspace
	informerFactory kcpinformers.SharedInformerFactory
	
	// authorizer handles authorization decisions
	authorizer authorizer.Authorizer
	
	// restHandler handles REST operations
	restHandler *RESTHandler
	
	// transformer handles resource transformation between virtual and cluster representations
	transformer *ResourceTransformer
	
	// scheme contains the runtime scheme for resource serialization
	scheme *runtime.Scheme
	
	// mutex protects concurrent handler operations
	mutex sync.RWMutex
	
	// Ready indicates whether the handler is ready to serve requests
	Ready bool
}

// HandlerConfig provides configuration for creating a new Handler
type HandlerConfig struct {
	ClusterClient   cluster.ClusterInterface
	InformerFactory kcpinformers.SharedInformerFactory
	Authorizer      authorizer.Authorizer
	Scheme          *runtime.Scheme
}

// NewHandler creates a new virtual workspace endpoints handler with the provided configuration.
// It initializes the REST handler and transformer components needed for processing virtual workspace requests.
//
// Parameters:
//   - config: Configuration containing cluster client, informer factory, authorizer, and scheme
//
// Returns:
//   - *Handler: Configured handler ready to serve virtual workspace endpoints
//   - error: Configuration or initialization error
func NewHandler(config *HandlerConfig) (*Handler, error) {
	if config.ClusterClient == nil {
		return nil, fmt.Errorf("cluster client is required")
	}
	if config.InformerFactory == nil {
		return nil, fmt.Errorf("informer factory is required")
	}
	if config.Authorizer == nil {
		return nil, fmt.Errorf("authorizer is required")
	}
	if config.Scheme == nil {
		return nil, fmt.Errorf("runtime scheme is required")
	}

	// Create the resource transformer
	transformer := NewResourceTransformer(&TransformerConfig{
		ClusterClient: config.ClusterClient,
		Scheme:        config.Scheme,
	})

	// Create the REST handler
	restHandler := NewRESTHandler(&RESTConfig{
		ClusterClient:   config.ClusterClient,
		InformerFactory: config.InformerFactory,
		Transformer:     transformer,
		Scheme:          config.Scheme,
	})

	handler := &Handler{
		clusterClient:   config.ClusterClient,
		informerFactory: config.InformerFactory,
		authorizer:      config.Authorizer,
		restHandler:     restHandler,
		transformer:     transformer,
		scheme:          config.Scheme,
		Ready:          false,
	}

	return handler, nil
}

// Start initializes the handler and starts background processes.
// It must be called before the handler can serve requests.
func (h *Handler) Start(ctx context.Context) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	klog.V(2).Info("Starting virtual workspace endpoints handler")

	// Start the informer factory
	h.informerFactory.Start(ctx.Done())

	// Wait for caches to sync with a timeout
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	synced := h.informerFactory.WaitForCacheSync(syncCtx.Done())
	for _, s := range synced {
		if !s {
			return fmt.Errorf("failed to sync caches within timeout")
		}
	}

	// Start the REST handler
	if err := h.restHandler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start REST handler: %w", err)
	}

	h.Ready = true
	klog.V(2).Info("Virtual workspace endpoints handler started successfully")
	return nil
}

// ServeHTTP implements the http.Handler interface for virtual workspace operations
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mutex.RLock()
	if !h.Ready {
		h.mutex.RUnlock()
		http.Error(w, "Handler not ready", http.StatusServiceUnavailable)
		return
	}
	h.mutex.RUnlock()

	requestInfo, ok := request.RequestInfoFrom(r.Context())
	if !ok {
		http.Error(w, "Unable to get request info", http.StatusBadRequest)
		return
	}

	if !h.isValidVirtualWorkspaceRequest(requestInfo) {
		http.Error(w, "Invalid virtual workspace request", http.StatusBadRequest)
		return
	}

	workspace, err := h.extractWorkspace(r)
	if err != nil {
		klog.Errorf("Failed to extract workspace: %v", err)
		http.Error(w, "Failed to extract workspace", http.StatusBadRequest)
		return
	}

	ctx := request.WithCluster(r.Context(), request.Cluster{
		Name: logicalcluster.Name(workspace),
	})
	r = r.WithContext(ctx)

	if err := h.authorize(r, requestInfo); err != nil {
		klog.Errorf("Authorization failed: %v", err)
		if errors.IsForbidden(err) {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else {
			http.Error(w, "Authorization error", http.StatusUnauthorized)
		}
		return
	}

	switch {
	case h.isAPIDiscoveryRequest(requestInfo):
		h.handleDiscovery(w, r, requestInfo)
	case h.isResourceRequest(requestInfo):
		h.restHandler.ServeHTTP(w, r)
	default:
		http.Error(w, "Unsupported request type", http.StatusNotFound)
	}
}

// isValidVirtualWorkspaceRequest validates that the request is appropriate for virtual workspace handling
func (h *Handler) isValidVirtualWorkspaceRequest(requestInfo *request.RequestInfo) bool {
	// Check if this is an API request
	if !requestInfo.IsResourceRequest {
		return requestInfo.Path == "/api" || requestInfo.Path == "/apis" || 
			   strings.HasPrefix(requestInfo.Path, "/api/") || 
			   strings.HasPrefix(requestInfo.Path, "/apis/")
	}

	// Validate resource request components
	if requestInfo.APIGroup == "" && requestInfo.APIVersion == "" {
		return false
	}

	return true
}

// extractWorkspace extracts the workspace name from the request
func (h *Handler) extractWorkspace(r *http.Request) (string, error) {
	// Try to extract from cluster in context first
	if cluster := request.ClusterFrom(r.Context()); cluster.Name != "" {
		return cluster.Name.String(), nil
	}

	// Extract from path if present
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) >= 2 && pathParts[0] == "clusters" {
		return pathParts[1], nil
	}

	return "root", nil
}

// authorize performs authorization checks for the request
func (h *Handler) authorize(r *http.Request, requestInfo *request.RequestInfo) error {
	user, ok := request.UserFrom(r.Context())
	if !ok {
		return fmt.Errorf("no user found in request context")
	}

	attrs := authorizer.AttributesRecord{
		User:            user,
		Verb:            requestInfo.Verb,
		Namespace:       requestInfo.Namespace,
		APIGroup:        requestInfo.APIGroup,
		APIVersion:      requestInfo.APIVersion,
		Resource:        requestInfo.Resource,
		Subresource:     requestInfo.Subresource,
		Name:            requestInfo.Name,
		ResourceRequest: requestInfo.IsResourceRequest,
		Path:            requestInfo.Path,
	}

	decision, reason, err := h.authorizer.Authorize(r.Context(), attrs)
	if err != nil {
		return fmt.Errorf("authorization error: %w", err)
	}

	if decision != authorizer.DecisionAllow {
		return errors.NewForbidden(schema.GroupResource{
			Group:    requestInfo.APIGroup,
			Resource: requestInfo.Resource,
		}, requestInfo.Name, fmt.Errorf("access denied: %s", reason))
	}

	return nil
}

// isAPIDiscoveryRequest checks if the request is for API discovery
func (h *Handler) isAPIDiscoveryRequest(requestInfo *request.RequestInfo) bool {
	if requestInfo.IsResourceRequest {
		return false
	}
	
	return requestInfo.Path == "/api" || requestInfo.Path == "/apis" ||
		   strings.HasPrefix(requestInfo.Path, "/api/v1") ||
		   strings.HasPrefix(requestInfo.Path, "/apis/")
}

// isResourceRequest checks if the request is for a specific resource
func (h *Handler) isResourceRequest(requestInfo *request.RequestInfo) bool {
	return requestInfo.IsResourceRequest
}

// handleDiscovery handles API discovery requests
func (h *Handler) handleDiscovery(w http.ResponseWriter, r *http.Request, requestInfo *request.RequestInfo) {
	klog.V(4).Infof("Handling discovery request for path: %s", requestInfo.Path)
	
	// TODO: Implement full discovery in follow-up PR
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := `{"kind": "APIVersions", "apiVersion": "v1", "versions": ["v1"]}`
	if strings.HasPrefix(requestInfo.Path, "/apis") {
		response = `{"kind": "APIGroupList", "apiVersion": "v1", "groups": []}`
	}
	
	if _, err := w.Write([]byte(response)); err != nil {
		klog.Errorf("Failed to write discovery response: %v", err)
	}
}

// Stop gracefully shuts down the handler
func (h *Handler) Stop() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	
	klog.V(2).Info("Stopping virtual workspace endpoints handler")
	h.Ready = false
}