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

package interfaces

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"github.com/kcp-dev/logicalcluster/v3"
)

// DynamicClusterClient provides cluster-aware dynamic client operations.
type DynamicClusterClient interface {
	// Cluster returns a dynamic interface for the specified logical cluster.
	Cluster(cluster logicalcluster.Path) dynamic.Interface

	// Resource returns a namespaced resource client for the specified GVR and cluster.
	Resource(cluster logicalcluster.Path, gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface
}

// ClusterAwareInformerFactory provides cluster-aware informer creation.
type ClusterAwareInformerFactory interface {
	// ForCluster returns an informer factory for the specified cluster.
	ForCluster(cluster logicalcluster.Path) cache.SharedIndexInformer

	// Start starts all informers managed by this factory.
	Start(stopCh <-chan struct{})

	// WaitForCacheSync waits for all started informers' caches to sync.
	WaitForCacheSync(stopCh <-chan struct{}) map[schema.GroupVersionResource]bool
}

// SyncEventRecorder provides event recording for sync operations.
type SyncEventRecorder interface {
	record.EventRecorder

	// RecordSyncEvent records a sync-specific event.
	RecordSyncEvent(object runtime.Object, operation SyncOperation, eventType, reason, message string)
}

// ResourceWatcher watches resources across clusters for sync operations.
type ResourceWatcher interface {
	// Watch starts watching resources in the specified cluster.
	Watch(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, handler ResourceEventHandler) error

	// StopWatch stops watching resources in the specified cluster.
	StopWatch(cluster logicalcluster.Path, gvr schema.GroupVersionResource)

	// IsWatching returns true if actively watching the specified resource.
	IsWatching(cluster logicalcluster.Path, gvr schema.GroupVersionResource) bool
}

// ResourceEventHandler handles resource events for synchronization.
type ResourceEventHandler interface {
	// OnAdd is called when a resource is added.
	OnAdd(obj *unstructured.Unstructured, cluster logicalcluster.Path)

	// OnUpdate is called when a resource is updated.
	OnUpdate(oldObj, newObj *unstructured.Unstructured, cluster logicalcluster.Path)

	// OnDelete is called when a resource is deleted.
	OnDelete(obj *unstructured.Unstructured, cluster logicalcluster.Path)
}

// ResourceAccessor provides unified access to resources across clusters.
type ResourceAccessor interface {
	// Get retrieves a resource from the specified cluster.
	Get(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error)

	// List lists resources in the specified cluster.
	List(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, namespace string) (*unstructured.UnstructuredList, error)

	// Create creates a resource in the specified cluster.
	Create(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)

	// Update updates a resource in the specified cluster.
	Update(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)

	// Delete deletes a resource from the specified cluster.
	Delete(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, namespace, name string) error

	// Patch patches a resource in the specified cluster.
	Patch(ctx context.Context, cluster logicalcluster.Path, gvr schema.GroupVersionResource, namespace, name string, pt types.PatchType, data []byte) (*unstructured.Unstructured, error)
}
