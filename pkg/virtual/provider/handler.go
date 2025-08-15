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

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

// Handler processes HTTP requests for virtual workspace operations
type Handler struct {
	// provider is the parent workspace provider
	provider *WorkspaceProvider

	// codec is used for encoding/decoding objects
	codec runtime.Codec
}

// NewHandler creates a new HTTP handler for virtual workspace requests
func NewHandler(provider *WorkspaceProvider) *Handler {
	if provider == nil {
		panic("provider cannot be nil")
	}

	return &Handler{
		provider: provider,
		// TODO: Initialize proper codec when needed
		codec: nil,
	}
}

// HandleGet handles GET requests for specific resources
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling GET request", "path", r.URL.Path)

	// For now, return a placeholder response
	response := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Status",
		"status":     "Success",
		"message":    "GET operation not yet implemented",
		"code":       200,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// HandleList handles GET requests for listing resources
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling LIST request", "path", r.URL.Path)

	// Extract workspace from request
	workspace, err := h.provider.router.GetWorkspaceName(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to extract workspace: %v", err), http.StatusBadRequest)
		return
	}

	// Get workspace to ensure it exists and is active
	vw, err := h.provider.GetWorkspace(string(workspace))
	if err != nil {
		http.Error(w, fmt.Sprintf("Workspace not found: %v", err), http.StatusNotFound)
		return
	}

	// Return available resources for the workspace
	response := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "List",
		"metadata": map[string]interface{}{
			"resourceVersion": strconv.FormatInt(time.Now().Unix(), 10),
		},
		"items": vw.Resources,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// HandleCreate handles POST requests for creating resources
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling CREATE request", "path", r.URL.Path)

	response := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Status",
		"status":     "Success",
		"message":    "CREATE operation not yet implemented",
		"code":       201,
	}

	h.writeJSONResponse(w, http.StatusCreated, response)
}

// HandleUpdate handles PUT requests for updating resources
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling UPDATE request", "path", r.URL.Path)

	response := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Status",
		"status":     "Success",
		"message":    "UPDATE operation not yet implemented",
		"code":       200,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// HandlePatch handles PATCH requests for patching resources
func (h *Handler) HandlePatch(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling PATCH request", "path", r.URL.Path)

	response := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Status",
		"status":     "Success",
		"message":    "PATCH operation not yet implemented",
		"code":       200,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// HandleDelete handles DELETE requests for removing resources
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling DELETE request", "path", r.URL.Path)

	response := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Status",
		"status":     "Success",
		"message":    "DELETE operation not yet implemented",
		"code":       200,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// HandleWatch handles GET requests with watch parameter for streaming events
func (h *Handler) HandleWatch(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling WATCH request", "path", r.URL.Path)

	// For watch requests, we need to set up streaming
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	// Send initial response indicating watch is not yet implemented
	response := map[string]interface{}{
		"type":   "ERROR",
		"object": map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Status",
			"status":     "Failure",
			"message":    "WATCH operation not yet implemented",
			"code":       501,
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		klog.ErrorS(err, "Failed to encode watch response")
	}

	// Flush the response
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// HandleWorkspaceGet handles GET requests for workspace metadata
func (h *Handler) HandleWorkspaceGet(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling workspace GET request", "path", r.URL.Path)

	workspace, err := h.provider.router.GetWorkspaceName(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to extract workspace: %v", err), http.StatusBadRequest)
		return
	}

	vw, err := h.provider.GetWorkspace(string(workspace))
	if err != nil {
		http.Error(w, fmt.Sprintf("Workspace not found: %v", err), http.StatusNotFound)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, vw)
}

// HandleWorkspaceCreate handles POST requests for creating new workspaces
func (h *Handler) HandleWorkspaceCreate(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling workspace CREATE request", "path", r.URL.Path)

	workspace, err := h.provider.router.GetWorkspaceName(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to extract workspace: %v", err), http.StatusBadRequest)
		return
	}

	// Parse request body for workspace configuration
	var config VirtualWorkspaceConfig
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse request body: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		config = VirtualWorkspaceConfig{Enabled: true}
	}

	vw, err := h.provider.CreateWorkspace(r.Context(), string(workspace), &config)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			http.Error(w, fmt.Sprintf("Workspace already exists: %v", err), http.StatusConflict)
		} else {
			http.Error(w, fmt.Sprintf("Failed to create workspace: %v", err), http.StatusInternalServerError)
		}
		return
	}

	h.writeJSONResponse(w, http.StatusCreated, vw)
}

// HandleWorkspaceUpdate handles PUT requests for updating workspace configuration
func (h *Handler) HandleWorkspaceUpdate(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling workspace UPDATE request", "path", r.URL.Path)

	response := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Status",
		"status":     "Success",
		"message":    "Workspace UPDATE operation not yet implemented",
		"code":       200,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// HandleWorkspaceDelete handles DELETE requests for removing workspaces
func (h *Handler) HandleWorkspaceDelete(w http.ResponseWriter, r *http.Request) {
	klog.V(4).InfoS("Handling workspace DELETE request", "path", r.URL.Path)

	workspace, err := h.provider.router.GetWorkspaceName(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to extract workspace: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.provider.DeleteWorkspace(r.Context(), string(workspace)); err != nil {
		if errors.IsNotFound(err) {
			http.Error(w, fmt.Sprintf("Workspace not found: %v", err), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to delete workspace: %v", err), http.StatusInternalServerError)
		}
		return
	}

	response := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Status",
		"status":     "Success",
		"message":    fmt.Sprintf("Workspace %s deleted successfully", workspace),
		"code":       200,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// writeJSONResponse writes a JSON response to the HTTP response writer
func (h *Handler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		klog.ErrorS(err, "Failed to encode JSON response")
		// At this point, we've already written the status code, so we can't change it
		// Just log the error
	}
}