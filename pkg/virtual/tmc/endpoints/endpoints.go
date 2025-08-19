/*
Copyright 2025 The KCP Authors.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/handlers/negotiation"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpdynamic "github.com/kcp-dev/client-go/dynamic"
)

// TMCEndpoints manages HTTP endpoints for TMC virtual workspace operations
type TMCEndpoints struct {
	dynamicClient kcpdynamic.ClusterInterface
	workspace     logicalcluster.Name
	pathPrefix    string
}

// EndpointConfig configures TMC endpoints
type EndpointConfig struct {
	DynamicClient kcpdynamic.ClusterInterface
	Workspace     logicalcluster.Name
	PathPrefix    string
}

// NewTMCEndpoints creates a new TMC endpoints handler
func NewTMCEndpoints(config *EndpointConfig) *TMCEndpoints {
	return &TMCEndpoints{
		dynamicClient: config.DynamicClient,
		workspace:     config.Workspace,
		pathPrefix:    config.PathPrefix,
	}
}

// InstallHandlers installs TMC virtual workspace HTTP handlers
func (e *TMCEndpoints) InstallHandlers(mux *http.ServeMux) {
	// Install TMC API resource handlers
	e.installTMCResourceHandlers(mux)
	
	// Install TMC cluster handlers
	e.installClusterHandlers(mux)
	
	// Install TMC placement handlers
	e.installPlacementHandlers(mux)
}

// installTMCResourceHandlers installs handlers for TMC API resources
func (e *TMCEndpoints) installTMCResourceHandlers(mux *http.ServeMux) {
	// TMC API groups and resources
	tmcResources := map[string][]string{
		"tmc.kcp.io": {
			"clusters",
			"placements", 
			"workloaddistributions",
		},
		"scheduling.tmc.kcp.io": {
			"placements",
			"schedulingconstraints",
		},
		"workload.tmc.kcp.io": {
			"workloaddistributions",
			"syncresources",
		},
	}

	for apiGroup, resources := range tmcResources {
		for _, resource := range resources {
			e.installResourceHandler(mux, apiGroup, resource)
		}
	}
}

// installResourceHandler installs a handler for a specific resource
func (e *TMCEndpoints) installResourceHandler(mux *http.ServeMux, apiGroup, resource string) {
	pattern := fmt.Sprintf("%s/%s/%s/%s", e.pathPrefix, "api", apiGroup, resource)
	
	mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		e.handleTMCResource(w, req, apiGroup, resource)
	})
	
	// Handle namespaced resources
	namespacedPattern := fmt.Sprintf("%s/%s/%s/namespaces/{namespace}/%s", 
		e.pathPrefix, "api", apiGroup, resource)
	mux.HandleFunc(namespacedPattern, func(w http.ResponseWriter, req *http.Request) {
		e.handleNamespacedTMCResource(w, req, apiGroup, resource)
	})
}

// handleTMCResource handles TMC resource requests
func (e *TMCEndpoints) handleTMCResource(w http.ResponseWriter, req *http.Request, apiGroup, resource string) {
	ctx := req.Context()
	
	klog.V(4).Infof("TMC endpoint handling %s request for %s/%s in workspace %s", 
		req.Method, apiGroup, resource, e.workspace)
	
	// Extract resource name from path if present
	pathParts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	var resourceName string
	if len(pathParts) > 4 {
		resourceName = pathParts[len(pathParts)-1]
	}
	
	gvr := schema.GroupVersionResource{
		Group:    apiGroup,
		Version:  "v1alpha1", // TMC APIs use v1alpha1
		Resource: resource,
	}
	
	switch req.Method {
	case http.MethodGet:
		e.handleGetResource(ctx, w, req, gvr, resourceName)
	case http.MethodPost:
		e.handleCreateResource(ctx, w, req, gvr)
	case http.MethodPut:
		e.handleUpdateResource(ctx, w, req, gvr, resourceName)
	case http.MethodDelete:
		e.handleDeleteResource(ctx, w, req, gvr, resourceName)
	default:
		responsewriters.ErrorNegotiated(
			fmt.Errorf("method %s not allowed", req.Method),
			server.Codecs, schema.GroupVersion{}, w, req,
		)
	}
}

// handleNamespacedTMCResource handles namespaced TMC resource requests
func (e *TMCEndpoints) handleNamespacedTMCResource(w http.ResponseWriter, req *http.Request, apiGroup, resource string) {
	ctx := req.Context()
	
	// Extract namespace from path
	pathParts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	var namespace, resourceName string
	
	for i, part := range pathParts {
		if part == "namespaces" && i+1 < len(pathParts) {
			namespace = pathParts[i+1]
			if i+3 < len(pathParts) {
				resourceName = pathParts[i+3]
			}
			break
		}
	}
	
	if namespace == "" {
		responsewriters.ErrorNegotiated(
			fmt.Errorf("namespace not found in path"),
			server.Codecs, schema.GroupVersion{}, w, req,
		)
		return
	}
	
	klog.V(4).Infof("TMC endpoint handling namespaced %s request for %s/%s in namespace %s, workspace %s", 
		req.Method, apiGroup, resource, namespace, e.workspace)
	
	gvr := schema.GroupVersionResource{
		Group:    apiGroup,
		Version:  "v1alpha1",
		Resource: resource,
	}
	
	switch req.Method {
	case http.MethodGet:
		e.handleGetNamespacedResource(ctx, w, req, gvr, namespace, resourceName)
	case http.MethodPost:
		e.handleCreateNamespacedResource(ctx, w, req, gvr, namespace)
	case http.MethodPut:
		e.handleUpdateNamespacedResource(ctx, w, req, gvr, namespace, resourceName)
	case http.MethodDelete:
		e.handleDeleteNamespacedResource(ctx, w, req, gvr, namespace, resourceName)
	default:
		responsewriters.ErrorNegotiated(
			fmt.Errorf("method %s not allowed", req.Method),
			server.Codecs, schema.GroupVersion{}, w, req,
		)
	}
}

// Resource operation handlers
func (e *TMCEndpoints) handleGetResource(ctx context.Context, w http.ResponseWriter, req *http.Request, gvr schema.GroupVersionResource, name string) {
	client := e.dynamicClient.Cluster(e.workspace.Path()).Resource(gvr)
	
	if name == "" {
		// List resources
		list, err := client.List(ctx, metav1.ListOptions{})
		if err != nil {
			responsewriters.ErrorNegotiated(err, server.Codecs, schema.GroupVersion{}, w, req)
			return
		}
		responsewriters.WriteObjectNegotiated(server.Codecs, negotiation.DefaultEndpointRestrictions, 
			schema.GroupVersion{}, w, req, http.StatusOK, list)
	} else {
		// Get specific resource
		obj, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			responsewriters.ErrorNegotiated(err, server.Codecs, schema.GroupVersion{}, w, req)
			return
		}
		responsewriters.WriteObjectNegotiated(server.Codecs, negotiation.DefaultEndpointRestrictions, 
			schema.GroupVersion{}, w, req, http.StatusOK, obj)
	}
}

// handleCreateResource creates a new resource
func (e *TMCEndpoints) handleCreateResource(ctx context.Context, w http.ResponseWriter, req *http.Request, gvr schema.GroupVersionResource) {
	// Implementation would decode request body and create resource
	responsewriters.ErrorNegotiated(
		fmt.Errorf("create operation not yet implemented"),
		server.Codecs, schema.GroupVersion{}, w, req,
	)
}

// handleUpdateResource updates an existing resource
func (e *TMCEndpoints) handleUpdateResource(ctx context.Context, w http.ResponseWriter, req *http.Request, gvr schema.GroupVersionResource, name string) {
	// Implementation would decode request body and update resource
	responsewriters.ErrorNegotiated(
		fmt.Errorf("update operation not yet implemented"),
		server.Codecs, schema.GroupVersion{}, w, req,
	)
}

// handleDeleteResource deletes a resource
func (e *TMCEndpoints) handleDeleteResource(ctx context.Context, w http.ResponseWriter, req *http.Request, gvr schema.GroupVersionResource, name string) {
	client := e.dynamicClient.Cluster(e.workspace.Path()).Resource(gvr)
	
	err := client.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		responsewriters.ErrorNegotiated(err, server.Codecs, schema.GroupVersion{}, w, req)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// Namespaced resource handlers
func (e *TMCEndpoints) handleGetNamespacedResource(ctx context.Context, w http.ResponseWriter, req *http.Request, gvr schema.GroupVersionResource, namespace, name string) {
	client := e.dynamicClient.Cluster(e.workspace.Path()).Resource(gvr).Namespace(namespace)
	
	if name == "" {
		list, err := client.List(ctx, metav1.ListOptions{})
		if err != nil {
			responsewriters.ErrorNegotiated(err, server.Codecs, schema.GroupVersion{}, w, req)
			return
		}
		responsewriters.WriteObjectNegotiated(server.Codecs, negotiation.DefaultEndpointRestrictions, 
			schema.GroupVersion{}, w, req, http.StatusOK, list)
	} else {
		obj, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			responsewriters.ErrorNegotiated(err, server.Codecs, schema.GroupVersion{}, w, req)
			return
		}
		responsewriters.WriteObjectNegotiated(server.Codecs, negotiation.DefaultEndpointRestrictions, 
			schema.GroupVersion{}, w, req, http.StatusOK, obj)
	}
}

func (e *TMCEndpoints) handleCreateNamespacedResource(ctx context.Context, w http.ResponseWriter, req *http.Request, gvr schema.GroupVersionResource, namespace string) {
	responsewriters.ErrorNegotiated(
		fmt.Errorf("create namespaced operation not yet implemented"),
		server.Codecs, schema.GroupVersion{}, w, req,
	)
}

func (e *TMCEndpoints) handleUpdateNamespacedResource(ctx context.Context, w http.ResponseWriter, req *http.Request, gvr schema.GroupVersionResource, namespace, name string) {
	responsewriters.ErrorNegotiated(
		fmt.Errorf("update namespaced operation not yet implemented"),
		server.Codecs, schema.GroupVersion{}, w, req,
	)
}

func (e *TMCEndpoints) handleDeleteNamespacedResource(ctx context.Context, w http.ResponseWriter, req *http.Request, gvr schema.GroupVersionResource, namespace, name string) {
	client := e.dynamicClient.Cluster(e.workspace.Path()).Resource(gvr).Namespace(namespace)
	
	err := client.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		responsewriters.ErrorNegotiated(err, server.Codecs, schema.GroupVersion{}, w, req)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// installClusterHandlers installs cluster-specific handlers
func (e *TMCEndpoints) installClusterHandlers(mux *http.ServeMux) {
	clusterPattern := fmt.Sprintf("%s/clusters", e.pathPrefix)
	mux.HandleFunc(clusterPattern, e.handleClusterOperations)
}

// handleClusterOperations handles cluster-level operations
func (e *TMCEndpoints) handleClusterOperations(w http.ResponseWriter, req *http.Request) {
	klog.V(4).Infof("TMC cluster operation: %s %s", req.Method, req.URL.Path)
	
	switch req.Method {
	case http.MethodGet:
		// List clusters or get specific cluster
		e.handleClusterList(w, req)
	default:
		responsewriters.ErrorNegotiated(
			fmt.Errorf("method %s not allowed for cluster operations", req.Method),
			server.Codecs, schema.GroupVersion{}, w, req,
		)
	}
}

// handleClusterList handles cluster listing
func (e *TMCEndpoints) handleClusterList(w http.ResponseWriter, req *http.Request) {
	// This would integrate with the cluster discovery system
	responsewriters.ErrorNegotiated(
		fmt.Errorf("cluster listing not yet implemented"),
		server.Codecs, schema.GroupVersion{}, w, req,
	)
}

// installPlacementHandlers installs placement-specific handlers
func (e *TMCEndpoints) installPlacementHandlers(mux *http.ServeMux) {
	placementPattern := fmt.Sprintf("%s/placement", e.pathPrefix)
	mux.HandleFunc(placementPattern, e.handlePlacementOperations)
}

// handlePlacementOperations handles placement operations
func (e *TMCEndpoints) handlePlacementOperations(w http.ResponseWriter, req *http.Request) {
	klog.V(4).Infof("TMC placement operation: %s %s", req.Method, req.URL.Path)
	
	// This would integrate with the placement system
	responsewriters.ErrorNegotiated(
		fmt.Errorf("placement operations not yet implemented"),
		server.Codecs, schema.GroupVersion{}, w, req,
	)
}