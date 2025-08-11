// Copyright The KCP Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package workqueue

import (
	"fmt"
	"sort"
	"sync"

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	kcpcache "github.com/kcp-dev/apimachinery/v2/pkg/cache"
	"github.com/kcp-dev/logicalcluster/v3"
)

// ClusterAwareIndex provides cluster-aware indexing for work queue items.
// This allows efficient lookup of work queue items by workspace and other criteria.
type ClusterAwareIndex interface {
	// IndexByWorkspace indexes items by their workspace
	IndexByWorkspace(req ClusterAwareRequest) ([]string, error)
	
	// IndexByPriority indexes items by their priority level
	IndexByPriority(req ClusterAwareRequest) ([]string, error)
	
	// IndexByRetryCount indexes items by their retry count
	IndexByRetryCount(req ClusterAwareRequest) ([]string, error)
	
	// GetByWorkspace returns all requests for a specific workspace
	GetByWorkspace(workspace logicalcluster.Name) ([]ClusterAwareRequest, error)
	
	// GetByPriority returns all requests with a specific priority
	GetByPriority(priority int) ([]ClusterAwareRequest, error)
	
	// GetHighPriorityRequests returns requests above a certain priority threshold
	GetHighPriorityRequests(minPriority int) ([]ClusterAwareRequest, error)
}

// WorkQueueIndexer combines Kubernetes indexer with cluster-aware indexing
type WorkQueueIndexer struct {
	indexer cache.Indexer
	mu      sync.RWMutex
	name    string
}

// Index names for cluster-aware indexing
const (
	WorkspaceIndexName  = "workspace"
	PriorityIndexName   = "priority" 
	RetryCountIndexName = "retryCount"
)

// NewWorkQueueIndexer creates a new cluster-aware work queue indexer
func NewWorkQueueIndexer(name string) *WorkQueueIndexer {
	indexers := cache.Indexers{
		WorkspaceIndexName:                   workspaceIndexFunc,
		PriorityIndexName:                    priorityIndexFunc,
		RetryCountIndexName:                  retryCountIndexFunc,
		kcpcache.ClusterIndexName:            clusterIndexFunc,
		kcpcache.ClusterAndNamespaceIndexName: clusterAndNamespaceIndexFunc,
	}
	
	return &WorkQueueIndexer{
		indexer: cache.NewIndexer(workQueueKeyFunc, indexers),
		name:    name,
	}
}

// workQueueKeyFunc generates a key for ClusterAwareRequest objects
func workQueueKeyFunc(obj interface{}) (string, error) {
	req, ok := obj.(ClusterAwareRequest)
	if !ok {
		return "", fmt.Errorf("object is not a ClusterAwareRequest: %T", obj)
	}
	return req.Key, nil
}

// workspaceIndexFunc indexes requests by workspace
func workspaceIndexFunc(obj interface{}) ([]string, error) {
	req, ok := obj.(ClusterAwareRequest)
	if !ok {
		return nil, fmt.Errorf("object is not a ClusterAwareRequest: %T", obj)
	}
	return []string{req.Workspace.String()}, nil
}

// priorityIndexFunc indexes requests by priority
func priorityIndexFunc(obj interface{}) ([]string, error) {
	req, ok := obj.(ClusterAwareRequest)
	if !ok {
		return nil, fmt.Errorf("object is not a ClusterAwareRequest: %T", obj)
	}
	return []string{fmt.Sprintf("%d", req.Priority)}, nil
}

// retryCountIndexFunc indexes requests by retry count
func retryCountIndexFunc(obj interface{}) ([]string, error) {
	req, ok := obj.(ClusterAwareRequest)
	if !ok {
		return nil, fmt.Errorf("object is not a ClusterAwareRequest: %T", obj)
	}
	return []string{fmt.Sprintf("%d", req.RetryCount)}, nil
}

// clusterIndexFunc indexes requests by cluster (workspace)
func clusterIndexFunc(obj interface{}) ([]string, error) {
	req, ok := obj.(ClusterAwareRequest)
	if !ok {
		return nil, fmt.Errorf("object is not a ClusterAwareRequest: %T", obj)
	}
	return []string{req.Workspace.String()}, nil
}

// clusterAndNamespaceIndexFunc indexes requests by cluster and namespace
func clusterAndNamespaceIndexFunc(obj interface{}) ([]string, error) {
	req, ok := obj.(ClusterAwareRequest)
	if !ok {
		return nil, fmt.Errorf("object is not a ClusterAwareRequest: %T", obj)
	}
	
	// Parse the key to extract namespace
	cluster, namespace, _, err := kcpcache.SplitMetaClusterNamespaceKey(req.Key)
	if err != nil {
		return nil, err
	}
	
	return []string{kcpcache.ClusterAndNamespaceIndexKey(cluster, namespace)}, nil
}

// Add adds a request to the indexer
func (idx *WorkQueueIndexer) Add(req ClusterAwareRequest) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	return idx.indexer.Add(req)
}

// Update updates a request in the indexer
func (idx *WorkQueueIndexer) Update(req ClusterAwareRequest) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	return idx.indexer.Update(req)
}

// Delete removes a request from the indexer
func (idx *WorkQueueIndexer) Delete(req ClusterAwareRequest) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	return idx.indexer.Delete(req)
}

// IndexByWorkspace implements ClusterAwareIndex.IndexByWorkspace
func (idx *WorkQueueIndexer) IndexByWorkspace(req ClusterAwareRequest) ([]string, error) {
	return workspaceIndexFunc(req)
}

// IndexByPriority implements ClusterAwareIndex.IndexByPriority
func (idx *WorkQueueIndexer) IndexByPriority(req ClusterAwareRequest) ([]string, error) {
	return priorityIndexFunc(req)
}

// IndexByRetryCount implements ClusterAwareIndex.IndexByRetryCount
func (idx *WorkQueueIndexer) IndexByRetryCount(req ClusterAwareRequest) ([]string, error) {
	return retryCountIndexFunc(req)
}

// GetByWorkspace implements ClusterAwareIndex.GetByWorkspace
func (idx *WorkQueueIndexer) GetByWorkspace(workspace logicalcluster.Name) ([]ClusterAwareRequest, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	objs, err := idx.indexer.ByIndex(WorkspaceIndexName, workspace.String())
	if err != nil {
		return nil, err
	}
	
	requests := make([]ClusterAwareRequest, 0, len(objs))
	for _, obj := range objs {
		if req, ok := obj.(ClusterAwareRequest); ok {
			requests = append(requests, req)
		}
	}
	
	// Sort by priority (highest first), then by request time (oldest first)
	sort.Slice(requests, func(i, j int) bool {
		if requests[i].Priority == requests[j].Priority {
			return requests[i].RequestTime.Before(requests[j].RequestTime)
		}
		return requests[i].Priority > requests[j].Priority
	})
	
	return requests, nil
}

// GetByPriority implements ClusterAwareIndex.GetByPriority
func (idx *WorkQueueIndexer) GetByPriority(priority int) ([]ClusterAwareRequest, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	objs, err := idx.indexer.ByIndex(PriorityIndexName, fmt.Sprintf("%d", priority))
	if err != nil {
		return nil, err
	}
	
	requests := make([]ClusterAwareRequest, 0, len(objs))
	for _, obj := range objs {
		if req, ok := obj.(ClusterAwareRequest); ok {
			requests = append(requests, req)
		}
	}
	
	return requests, nil
}

// GetHighPriorityRequests implements ClusterAwareIndex.GetHighPriorityRequests
func (idx *WorkQueueIndexer) GetHighPriorityRequests(minPriority int) ([]ClusterAwareRequest, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	var allRequests []ClusterAwareRequest
	
	// Get all requests from the indexer
	for _, obj := range idx.indexer.List() {
		if req, ok := obj.(ClusterAwareRequest); ok && req.Priority >= minPriority {
			allRequests = append(allRequests, req)
		}
	}
	
	// Sort by priority (highest first), then by request time (oldest first)
	sort.Slice(allRequests, func(i, j int) bool {
		if allRequests[i].Priority == allRequests[j].Priority {
			return allRequests[i].RequestTime.Before(allRequests[j].RequestTime)
		}
		return allRequests[i].Priority > allRequests[j].Priority
	})
	
	return allRequests, nil
}

// GetStats returns statistics about the indexed requests
func (idx *WorkQueueIndexer) GetStats() WorkQueueStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	stats := WorkQueueStats{
		TotalRequests: len(idx.indexer.List()),
		ByWorkspace:   make(map[string]int),
		ByPriority:    make(map[int]int),
		ByRetryCount:  make(map[int]int),
	}
	
	for _, obj := range idx.indexer.List() {
		if req, ok := obj.(ClusterAwareRequest); ok {
			stats.ByWorkspace[req.Workspace.String()]++
			stats.ByPriority[req.Priority]++
			stats.ByRetryCount[req.RetryCount]++
			
			if req.Priority > stats.MaxPriority {
				stats.MaxPriority = req.Priority
			}
			if req.Priority < stats.MinPriority || stats.MinPriority == 0 {
				stats.MinPriority = req.Priority
			}
			if req.RetryCount > stats.MaxRetries {
				stats.MaxRetries = req.RetryCount
			}
		}
	}
	
	return stats
}

// WorkQueueStats provides statistics about indexed work queue requests
type WorkQueueStats struct {
	TotalRequests int
	ByWorkspace   map[string]int
	ByPriority    map[int]int
	ByRetryCount  map[int]int
	MaxPriority   int
	MinPriority   int
	MaxRetries    int
}

// LogStats logs the work queue statistics
func (stats WorkQueueStats) LogStats(queueName string) {
	klog.InfoS("Work queue statistics",
		"queue", queueName,
		"totalRequests", stats.TotalRequests,
		"workspaces", len(stats.ByWorkspace),
		"priorityLevels", len(stats.ByPriority),
		"maxPriority", stats.MaxPriority,
		"minPriority", stats.MinPriority,
		"maxRetries", stats.MaxRetries)
	
	// Log workspace breakdown
	for workspace, count := range stats.ByWorkspace {
		klog.V(4).InfoS("Workspace requests",
			"queue", queueName,
			"workspace", workspace,
			"count", count)
	}
}

// ClusterAwareIndexingOptions provides configuration for cluster-aware indexing
type ClusterAwareIndexingOptions struct {
	// EnableWorkspaceIndexing enables indexing by workspace
	EnableWorkspaceIndexing bool
	
	// EnablePriorityIndexing enables indexing by priority
	EnablePriorityIndexing bool
	
	// EnableRetryIndexing enables indexing by retry count
	EnableRetryIndexing bool
	
	// CustomIndexers allows adding custom indexing functions
	CustomIndexers map[string]cache.IndexFunc
}

// DefaultClusterAwareIndexingOptions returns default indexing options
func DefaultClusterAwareIndexingOptions() ClusterAwareIndexingOptions {
	return ClusterAwareIndexingOptions{
		EnableWorkspaceIndexing: true,
		EnablePriorityIndexing:  true,
		EnableRetryIndexing:     true,
		CustomIndexers:          make(map[string]cache.IndexFunc),
	}
}