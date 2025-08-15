/*
Copyright 2023 The KCP Authors.

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

package discovery

import (
	"context"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
	"github.com/kcp-dev/kcp/pkg/virtual/contracts"
)

// ResourceWatcher monitors APIExport changes and generates discovery events
type ResourceWatcher struct {
	// provider is the parent discovery provider
	provider *KCPDiscoveryProvider

	// eventCh broadcasts discovery events
	eventCh chan interfaces.DiscoveryEvent

	// subscribers tracks active event subscribers
	subscribers map[string]chan interfaces.DiscoveryEvent

	// mutex protects concurrent access to subscribers
	mutex sync.RWMutex

	// stopCh signals shutdown
	stopCh <-chan struct{}
}

// NewResourceWatcher creates a new resource watcher
func NewResourceWatcher(provider *KCPDiscoveryProvider, stopCh <-chan struct{}) *ResourceWatcher {
	return &ResourceWatcher{
		provider:    provider,
		eventCh:     make(chan interfaces.DiscoveryEvent, contracts.DefaultWatchChannelBuffer),
		subscribers: make(map[string]chan interfaces.DiscoveryEvent),
		stopCh:      stopCh,
	}
}

// Start begins watching for resource changes
func (w *ResourceWatcher) Start(ctx context.Context) error {
	klog.V(3).InfoS("Starting resource watcher")

	// Register event handlers for APIExport changes
	_, err := w.provider.apiExportInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleAPIExportAdd,
		UpdateFunc: w.handleAPIExportUpdate,
		DeleteFunc: w.handleAPIExportDelete,
	})
	if err != nil {
		return err
	}

	// Start event processing goroutine
	go w.processEvents()

	klog.V(3).InfoS("Resource watcher started successfully")
	return nil
}

// Subscribe creates a new event subscription for a workspace
func (w *ResourceWatcher) Subscribe(workspace string) <-chan interfaces.DiscoveryEvent {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	eventCh := make(chan interfaces.DiscoveryEvent, contracts.DefaultWatchChannelBuffer)
	w.subscribers[workspace] = eventCh

	UpdateActiveWatchers(workspace, 1)
	klog.V(4).InfoS("Added discovery event subscription", "workspace", workspace)

	return eventCh
}

// Unsubscribe removes an event subscription
func (w *ResourceWatcher) Unsubscribe(workspace string) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if eventCh, exists := w.subscribers[workspace]; exists {
		close(eventCh)
		delete(w.subscribers, workspace)
		UpdateActiveWatchers(workspace, -1)
		klog.V(4).InfoS("Removed discovery event subscription", "workspace", workspace)
	}
}

// handleAPIExportAdd processes new APIExport additions
func (w *ResourceWatcher) handleAPIExportAdd(obj interface{}) {
	apiExport, ok := obj.(*apisv1alpha1.APIExport)
	if !ok {
		klog.V(2).InfoS("Received non-APIExport object in add handler")
		return
	}

	workspace := w.extractWorkspaceFromAPIExport(apiExport)
	if workspace == "" {
		return
	}

	// Invalidate cache for this workspace
	w.provider.cache.InvalidateWorkspace(workspace)

	// Convert APIExport to ResourceInfo events
	resources, err := w.provider.converter.ConvertAPIExport(apiExport)
	if err != nil {
		klog.ErrorS(err, "Failed to convert APIExport in add handler", "name", apiExport.Name)
		return
	}

	// Generate events for each resource
	for _, resource := range resources {
		event := interfaces.DiscoveryEvent{
			Type:      interfaces.DiscoveryEventAdded,
			Workspace: workspace,
			Resource:  resource,
			Timestamp: metav1.Now(),
		}
		w.broadcastEvent(event)
	}

	klog.V(4).InfoS("Processed APIExport addition", "name", apiExport.Name, "workspace", workspace, "resources", len(resources))
}

// handleAPIExportUpdate processes APIExport updates
func (w *ResourceWatcher) handleAPIExportUpdate(oldObj, newObj interface{}) {
	newAPIExport, ok := newObj.(*apisv1alpha1.APIExport)
	if !ok {
		klog.V(2).InfoS("Received non-APIExport object in update handler")
		return
	}

	workspace := w.extractWorkspaceFromAPIExport(newAPIExport)
	if workspace == "" {
		return
	}

	// Invalidate cache for this workspace
	w.provider.cache.InvalidateWorkspace(workspace)

	// Convert APIExport to ResourceInfo events
	resources, err := w.provider.converter.ConvertAPIExport(newAPIExport)
	if err != nil {
		klog.ErrorS(err, "Failed to convert APIExport in update handler", "name", newAPIExport.Name)
		return
	}

	// Generate update events for each resource
	for _, resource := range resources {
		event := interfaces.DiscoveryEvent{
			Type:      interfaces.DiscoveryEventUpdated,
			Workspace: workspace,
			Resource:  resource,
			Timestamp: metav1.Now(),
		}
		w.broadcastEvent(event)
	}

	klog.V(4).InfoS("Processed APIExport update", "name", newAPIExport.Name, "workspace", workspace, "resources", len(resources))
}

// handleAPIExportDelete processes APIExport deletions
func (w *ResourceWatcher) handleAPIExportDelete(obj interface{}) {
	apiExport, ok := obj.(*apisv1alpha1.APIExport)
	if !ok {
		// Handle tombstone
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			apiExport, ok = tombstone.Obj.(*apisv1alpha1.APIExport)
			if !ok {
				klog.V(2).InfoS("Received non-APIExport object in delete handler tombstone")
				return
			}
		} else {
			klog.V(2).InfoS("Received non-APIExport object in delete handler")
			return
		}
	}

	workspace := w.extractWorkspaceFromAPIExport(apiExport)
	if workspace == "" {
		return
	}

	// Invalidate cache for this workspace
	w.provider.cache.InvalidateWorkspace(workspace)

	// Convert APIExport to ResourceInfo events
	resources, err := w.provider.converter.ConvertAPIExport(apiExport)
	if err != nil {
		klog.ErrorS(err, "Failed to convert APIExport in delete handler", "name", apiExport.Name)
		return
	}

	// Generate delete events for each resource
	for _, resource := range resources {
		event := interfaces.DiscoveryEvent{
			Type:      interfaces.DiscoveryEventDeleted,
			Workspace: workspace,
			Resource:  resource,
			Timestamp: metav1.Now(),
		}
		w.broadcastEvent(event)
	}

	klog.V(4).InfoS("Processed APIExport deletion", "name", apiExport.Name, "workspace", workspace, "resources", len(resources))
}

// broadcastEvent sends an event to all subscribers
func (w *ResourceWatcher) broadcastEvent(event interfaces.DiscoveryEvent) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	// Send to workspace-specific subscribers
	if eventCh, exists := w.subscribers[event.Workspace]; exists {
		select {
		case eventCh <- event:
		default:
			klog.V(2).InfoS("Event channel full, dropping event", "workspace", event.Workspace, "type", event.Type)
		}
	}
}

// processEvents handles the main event processing loop
func (w *ResourceWatcher) processEvents() {
	defer klog.V(3).InfoS("Resource watcher event processing stopped")

	for {
		select {
		case <-w.stopCh:
			// Close all subscriber channels
			w.mutex.Lock()
			for workspace, eventCh := range w.subscribers {
				close(eventCh)
				UpdateActiveWatchers(workspace, -1)
			}
			w.subscribers = make(map[string]chan interfaces.DiscoveryEvent)
			w.mutex.Unlock()
			return
		}
	}
}

// extractWorkspaceFromAPIExport determines the workspace for an APIExport
func (w *ResourceWatcher) extractWorkspaceFromAPIExport(apiExport *apisv1alpha1.APIExport) string {
	// Extract workspace from annotations or use provider workspace
	if workspace, ok := apiExport.Annotations["cluster.kcp.io/workspace"]; ok {
		return workspace
	}

	// Fallback to provider workspace
	return w.provider.workspace
}