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
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/handlers"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
	kcpdynamic "github.com/kcp-dev/client-go/dynamic"

	"github.com/kcp-dev/kcp/pkg/features"
	tmcstorage "github.com/kcp-dev/kcp/pkg/virtual/tmc/storage"
)

// TMCVirtualWorkspaceEndpoints manages HTTP endpoints for TMC virtual workspace operations.
type TMCVirtualWorkspaceEndpoints struct {
	dynamicClient kcpdynamic.ClusterInterface
	workspace     logicalcluster.Name
	pathPrefix    string
	storages      map[schema.GroupVersionResource]*tmcstorage.TMCVirtualWorkspaceStorage
}

// TMCEndpointsConfig configures the TMC virtual workspace endpoints.
type TMCEndpointsConfig struct {
	// DynamicClient provides access to cluster resources
	DynamicClient kcpdynamic.ClusterInterface
	
	// Workspace specifies the target logical cluster
	Workspace logicalcluster.Name
	
	// PathPrefix is the URL path prefix for TMC endpoints
	PathPrefix string
	
	// EnabledResources specifies which TMC resources should be exposed
	EnabledResources []schema.GroupVersionResource
}

// NewTMCVirtualWorkspaceEndpoints creates new TMC virtual workspace endpoints.
func NewTMCVirtualWorkspaceEndpoints(config TMCEndpointsConfig) (*TMCVirtualWorkspaceEndpoints, error) {
	endpoints := &TMCVirtualWorkspaceEndpoints{
		dynamicClient: config.DynamicClient,
		workspace:     config.Workspace,
		pathPrefix:    config.PathPrefix,
		storages:      make(map[schema.GroupVersionResource]*tmcstorage.TMCVirtualWorkspaceStorage),
	}
	
	// Initialize storage for each enabled resource
	for _, gvr := range config.EnabledResources {
		storage := tmcstorage.NewTMCVirtualWorkspaceStorage(tmcstorage.TMCStorageConfig{
			DynamicClient:        config.DynamicClient,
			GroupVersionResource: gvr,
			Workspace:           config.Workspace,
			IsNamespaced:        isNamespacedResource(gvr),
		})
		endpoints.storages[gvr] = storage
	}
	
	return endpoints, nil
}

// InstallHandlers installs HTTP handlers for TMC virtual workspace endpoints.
func (e *TMCVirtualWorkspaceEndpoints) InstallHandlers(mux *http.ServeMux) error {
	if !features.DefaultMutableFeatureGate.Enabled(features.TMCEnabled) {
		klog.V(2).Info("TMC virtual workspace endpoints disabled by feature gate")
		return nil
	}
	
	// Install handlers for each TMC resource
	for gvr, storage := range e.storages {
		if err := e.installResourceHandlers(mux, gvr, storage); err != nil {
			return fmt.Errorf("failed to install handlers for %s: %w", gvr, err)
		}
	}
	
	// Install discovery endpoints
	e.installDiscoveryHandlers(mux)
	
	klog.V(2).Infof("Installed TMC virtual workspace endpoints for workspace %s at path %s", 
		e.workspace, e.pathPrefix)
	
	return nil
}

// installResourceHandlers installs HTTP handlers for a specific TMC resource.
func (e *TMCVirtualWorkspaceEndpoints) installResourceHandlers(mux *http.ServeMux, gvr schema.GroupVersionResource, storage *tmcstorage.TMCVirtualWorkspaceStorage) error {
	// Build resource path: /services/tmc/workspaces/{workspace}/api/{version}/{resource}
	resourcePath := path.Join(e.pathPrefix, "workspaces", string(e.workspace), "api", gvr.Version, gvr.Resource)
	
	// Collection endpoints (list, create)
	mux.HandleFunc(resourcePath, e.handleResourceCollection(gvr, storage))
	mux.HandleFunc(resourcePath+"/", e.handleResourceCollection(gvr, storage))
	
	// Item endpoints (get, update, delete)
	itemPath := resourcePath + "/"
	mux.HandleFunc(itemPath, e.handleResourceItem(gvr, storage))
	
	// Namespaced resource endpoints if applicable
	if isNamespacedResource(gvr) {
		namespacedPath := path.Join(e.pathPrefix, "workspaces", string(e.workspace), "api", gvr.Version, "namespaces")
		mux.HandleFunc(namespacedPath+"/", e.handleNamespacedResource(gvr, storage))
	}
	
	klog.V(4).Infof("Installed handlers for TMC resource %s at path %s", gvr, resourcePath)
	return nil
}

// handleResourceCollection handles collection-level operations (list, create).
func (e *TMCVirtualWorkspaceEndpoints) handleResourceCollection(gvr schema.GroupVersionResource, storage *tmcstorage.TMCVirtualWorkspaceStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		
		switch req.Method {
		case http.MethodGet:
			e.handleList(ctx, w, req, storage)
		case http.MethodPost:
			e.handleCreate(ctx, w, req, storage)
		default:
			responsewriters.ErrorNegotiated(
				&server.RequestContextMapper{},
				schema.GroupVersion{},
				w,
				req,
				fmt.Errorf("method %s not supported for resource collection", req.Method),
				nil,
			)
		}
	}
}

// handleResourceItem handles item-level operations (get, update, delete).
func (e *TMCVirtualWorkspaceEndpoints) handleResourceItem(gvr schema.GroupVersionResource, storage *tmcstorage.TMCVirtualWorkspaceStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		
		// Extract resource name from URL path
		pathParts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
		if len(pathParts) == 0 {
			responsewriters.ErrorNegotiated(
				&server.RequestContextMapper{},
				schema.GroupVersion{},
				w,
				req,
				fmt.Errorf("resource name not specified"),
				nil,
			)
			return
		}
		
		name := pathParts[len(pathParts)-1]
		if name == "" {
			responsewriters.ErrorNegotiated(
				&server.RequestContextMapper{},
				schema.GroupVersion{},
				w,
				req,
				fmt.Errorf("resource name is empty"),
				nil,
			)
			return
		}
		
		switch req.Method {
		case http.MethodGet:
			e.handleGet(ctx, w, req, storage, name)
		case http.MethodPut:
			e.handleUpdate(ctx, w, req, storage, name)
		case http.MethodPatch:
			e.handlePatch(ctx, w, req, storage, name)
		case http.MethodDelete:
			e.handleDelete(ctx, w, req, storage, name)
		default:
			responsewriters.ErrorNegotiated(
				&server.RequestContextMapper{},
				schema.GroupVersion{},
				w,
				req,
				fmt.Errorf("method %s not supported for resource item", req.Method),
				nil,
			)
		}
	}
}

// handleNamespacedResource handles namespaced resource operations.
func (e *TMCVirtualWorkspaceEndpoints) handleNamespacedResource(gvr schema.GroupVersionResource, storage *tmcstorage.TMCVirtualWorkspaceStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		
		// Extract namespace from URL path
		// URL pattern: /services/tmc/workspaces/{workspace}/api/{version}/namespaces/{namespace}/{resource}
		pathParts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
		
		// Find namespace in path
		namespaceIndex := -1
		for i, part := range pathParts {
			if part == "namespaces" && i+1 < len(pathParts) {
				namespaceIndex = i + 1
				break
			}
		}
		
		if namespaceIndex == -1 || namespaceIndex >= len(pathParts) {
			responsewriters.ErrorNegotiated(
				&server.RequestContextMapper{},
				schema.GroupVersion{},
				w,
				req,
				fmt.Errorf("namespace not specified in URL path"),
				nil,
			)
			return
		}
		
		namespace := pathParts[namespaceIndex]
		if namespace == "" {
			responsewriters.ErrorNegotiated(
				&server.RequestContextMapper{},
				schema.GroupVersion{},
				w,
				req,
				fmt.Errorf("namespace is empty"),
				nil,
			)
			return
		}
		
		// Handle the request with namespace context
		// For now, delegate to the same handlers but with namespace awareness
		switch req.Method {
		case http.MethodGet:
			e.handleList(ctx, w, req, storage)
		case http.MethodPost:
			e.handleCreate(ctx, w, req, storage)
		default:
			responsewriters.ErrorNegotiated(
				&server.RequestContextMapper{},
				schema.GroupVersion{},
				w,
				req,
				fmt.Errorf("method %s not supported for namespaced resource", req.Method),
				nil,
			)
		}
	}
}

// Individual operation handlers (simplified implementations)

func (e *TMCVirtualWorkspaceEndpoints) handleList(ctx context.Context, w http.ResponseWriter, req *http.Request, storage *tmcstorage.TMCVirtualWorkspaceStorage) {
	// TODO: Implement proper list handling using storage.List()
	klog.V(4).Info("TMC virtual workspace list operation")
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"kind":"List","items":[]}`))
}

func (e *TMCVirtualWorkspaceEndpoints) handleGet(ctx context.Context, w http.ResponseWriter, req *http.Request, storage *tmcstorage.TMCVirtualWorkspaceStorage, name string) {
	// TODO: Implement proper get handling using storage.Get()
	klog.V(4).Infof("TMC virtual workspace get operation for %s", name)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"kind":"Object","metadata":{"name":"` + name + `"}}`))
}

func (e *TMCVirtualWorkspaceEndpoints) handleCreate(ctx context.Context, w http.ResponseWriter, req *http.Request, storage *tmcstorage.TMCVirtualWorkspaceStorage) {
	// TODO: Implement proper create handling using storage.Create()
	klog.V(4).Info("TMC virtual workspace create operation")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"kind":"Object","metadata":{"name":"created"}}`))
}

func (e *TMCVirtualWorkspaceEndpoints) handleUpdate(ctx context.Context, w http.ResponseWriter, req *http.Request, storage *tmcstorage.TMCVirtualWorkspaceStorage, name string) {
	// TODO: Implement proper update handling using storage.Update()
	klog.V(4).Infof("TMC virtual workspace update operation for %s", name)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"kind":"Object","metadata":{"name":"` + name + `"}}`))
}

func (e *TMCVirtualWorkspaceEndpoints) handlePatch(ctx context.Context, w http.ResponseWriter, req *http.Request, storage *tmcstorage.TMCVirtualWorkspaceStorage, name string) {
	// TODO: Implement proper patch handling
	klog.V(4).Infof("TMC virtual workspace patch operation for %s", name)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"kind":"Object","metadata":{"name":"` + name + `"}}`))
}

func (e *TMCVirtualWorkspaceEndpoints) handleDelete(ctx context.Context, w http.ResponseWriter, req *http.Request, storage *tmcstorage.TMCVirtualWorkspaceStorage, name string) {
	// TODO: Implement proper delete handling using storage.Delete()
	klog.V(4).Infof("TMC virtual workspace delete operation for %s", name)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"kind":"Status","status":"Success"}`))
}

// installDiscoveryHandlers installs API discovery endpoints.
func (e *TMCVirtualWorkspaceEndpoints) installDiscoveryHandlers(mux *http.ServeMux) {
	// API groups discovery
	groupsPath := path.Join(e.pathPrefix, "workspaces", string(e.workspace), "api")
	mux.HandleFunc(groupsPath, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"kind":"APIGroupList","groups":[{"name":"tmc.kcp.io","versions":[{"groupVersion":"tmc.kcp.io/v1alpha1","version":"v1alpha1"}]}]}`))
	})
	
	klog.V(4).Infof("Installed TMC discovery handlers at path %s", groupsPath)
}

// isNamespacedResource determines if a TMC resource is namespaced.
func isNamespacedResource(gvr schema.GroupVersionResource) bool {
	namespacedResources := map[string]bool{
		"workloadplacements": true,
		"workloadsyncs":      true,
	}
	
	return namespacedResources[gvr.Resource]
}