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
	"sync"
	"time"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/logicalcluster/v3"
)

// IndexFunc defines a function that extracts index values from a work queue key.
// This follows KCP patterns for workspace-aware indexing.
type IndexFunc func(key string) ([]string, error)

// IndexedQueue provides a workqueue with indexing capabilities for efficient
// key retrieval and filtering. This is particularly useful for TMC controllers
// that need to process items based on specific criteria like workspace,
// resource type, or other metadata encoded in the queue keys.
type IndexedQueue interface {
	workqueue.TypedRateLimitingInterface[string]
	
	// AddIndexer adds an indexer function for the given index name.
	// Index names should be unique and descriptive (e.g., "byWorkspace", "byResourceType").
	AddIndexer(indexName string, indexFunc IndexFunc) error
	
	// GetByIndex returns all queue keys that match the given index value.
	// This allows efficient retrieval of related items without scanning the entire queue.
	GetByIndex(indexName, indexValue string) ([]string, error)
	
	// ListIndexValues returns all index values for the given index name.
	// This is useful for discovering all possible values in an index.
	ListIndexValues(indexName string) []string
	
	// HasIndex returns true if the given index name exists.
	HasIndex(indexName string) bool
	
	// RemoveFromIndexes removes a key from all indexes.
	// This should be called when a key is processed or removed from the queue.
	RemoveFromIndexes(key string)
}

// indexedQueueImpl implements IndexedQueue with thread-safe indexing capabilities.
// It wraps a standard TypedRateLimitingInterface and maintains additional
// index structures for efficient key lookup.
type indexedQueueImpl struct {
	// Embedded queue provides the base workqueue functionality
	workqueue.TypedRateLimitingInterface[string]
	
	// Index management
	mu           sync.RWMutex
	indexers     map[string]IndexFunc               // indexName -> IndexFunc
	indexes      map[string]map[string][]string     // indexName -> indexValue -> []keys
	keyToIndexes map[string]map[string][]string     // key -> indexName -> []indexValues
	
	// Metrics and configuration
	name      string
	workspace logicalcluster.Name
}

// NewIndexedQueue creates a new indexed queue with the given configuration.
// This provides enhanced workqueue functionality with indexing capabilities
// specifically designed for TMC controllers in KCP environments.
func NewIndexedQueue(name string, workspace logicalcluster.Name) IndexedQueue {
	baseQueue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[string](),
		workqueue.TypedRateLimitingQueueConfig[string]{
			Name: name,
		},
	)
	
	return &indexedQueueImpl{
		TypedRateLimitingInterface: baseQueue,
		indexers:                   make(map[string]IndexFunc),
		indexes:                    make(map[string]map[string][]string),
		keyToIndexes:               make(map[string]map[string][]string),
		name:                       name,
		workspace:                  workspace,
	}
}

// AddIndexer implements IndexedQueue.AddIndexer
func (q *indexedQueueImpl) AddIndexer(indexName string, indexFunc IndexFunc) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if _, exists := q.indexers[indexName]; exists {
		klog.V(4).InfoS("Indexer already exists, replacing", 
			"queue", q.name,
			"workspace", q.workspace,
			"indexName", indexName)
	}
	
	q.indexers[indexName] = indexFunc
	q.indexes[indexName] = make(map[string][]string)
	
	klog.V(6).InfoS("Added indexer to queue", 
		"queue", q.name,
		"workspace", q.workspace,
		"indexName", indexName)
	
	return nil
}

// GetByIndex implements IndexedQueue.GetByIndex
func (q *indexedQueueImpl) GetByIndex(indexName, indexValue string) ([]string, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	indexValues, exists := q.indexes[indexName]
	if !exists {
		return nil, NewIndexNotFoundError(indexName)
	}
	
	keys, exists := indexValues[indexValue]
	if !exists {
		return []string{}, nil // No keys for this index value
	}
	
	// Return a copy to prevent external modification
	result := make([]string, len(keys))
	copy(result, keys)
	
	return result, nil
}

// ListIndexValues implements IndexedQueue.ListIndexValues
func (q *indexedQueueImpl) ListIndexValues(indexName string) []string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	indexValues, exists := q.indexes[indexName]
	if !exists {
		return []string{}
	}
	
	result := make([]string, 0, len(indexValues))
	for indexValue := range indexValues {
		result = append(result, indexValue)
	}
	
	return result
}

// HasIndex implements IndexedQueue.HasIndex
func (q *indexedQueueImpl) HasIndex(indexName string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	_, exists := q.indexers[indexName]
	return exists
}

// RemoveFromIndexes implements IndexedQueue.RemoveFromIndexes
func (q *indexedQueueImpl) RemoveFromIndexes(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	// Remove key from all its index values
	if keyIndexes, exists := q.keyToIndexes[key]; exists {
		for indexName, indexValues := range keyIndexes {
			if indexData, indexExists := q.indexes[indexName]; indexExists {
				for _, indexValue := range indexValues {
					if keys, valueExists := indexData[indexValue]; valueExists {
						// Remove key from the slice
						newKeys := make([]string, 0, len(keys))
						for _, k := range keys {
							if k != key {
								newKeys = append(newKeys, k)
							}
						}
						
						if len(newKeys) == 0 {
							// No more keys for this index value, remove it
							delete(indexData, indexValue)
						} else {
							indexData[indexValue] = newKeys
						}
					}
				}
			}
		}
		
		// Remove key from keyToIndexes mapping
		delete(q.keyToIndexes, key)
	}
}

// Add overrides the base Add method to maintain indexes
func (q *indexedQueueImpl) Add(key string) {
	// Add to base queue first
	q.TypedRateLimitingInterface.Add(key)
	
	// Update indexes
	q.updateIndexes(key)
}

// AddRateLimited overrides the base AddRateLimited method to maintain indexes
func (q *indexedQueueImpl) AddRateLimited(key string) {
	// Add to base queue first
	q.TypedRateLimitingInterface.AddRateLimited(key)
	
	// Update indexes
	q.updateIndexes(key)
}

// AddAfter overrides the base AddAfter method to maintain indexes
func (q *indexedQueueImpl) AddAfter(key string, duration time.Duration) {
	// Add to base queue first
	q.TypedRateLimitingInterface.AddAfter(key, duration)
	
	// Update indexes
	q.updateIndexes(key)
}

// updateIndexes updates all indexes for the given key
func (q *indexedQueueImpl) updateIndexes(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	// Remove existing index entries for this key
	q.removeFromIndexesUnlocked(key)
	
	// Add new index entries
	keyIndexes := make(map[string][]string)
	
	for indexName, indexFunc := range q.indexers {
		indexValues, err := indexFunc(key)
		if err != nil {
			klog.V(4).InfoS("Error indexing key, skipping", 
				"queue", q.name,
				"workspace", q.workspace,
				"key", key,
				"indexName", indexName,
				"error", err)
			continue
		}
		
		// Ensure index exists
		if _, exists := q.indexes[indexName]; !exists {
			q.indexes[indexName] = make(map[string][]string)
		}
		
		// Add key to each index value
		for _, indexValue := range indexValues {
			if q.indexes[indexName][indexValue] == nil {
				q.indexes[indexName][indexValue] = []string{}
			}
			
			// Check if key already exists to avoid duplicates
			found := false
			for _, existingKey := range q.indexes[indexName][indexValue] {
				if existingKey == key {
					found = true
					break
				}
			}
			
			if !found {
				q.indexes[indexName][indexValue] = append(q.indexes[indexName][indexValue], key)
			}
		}
		
		// Track index values for this key
		if len(indexValues) > 0 {
			keyIndexes[indexName] = indexValues
		}
	}
	
	// Update keyToIndexes mapping
	if len(keyIndexes) > 0 {
		q.keyToIndexes[key] = keyIndexes
	}
}

// removeFromIndexesUnlocked removes a key from all indexes (must be called with lock held)
func (q *indexedQueueImpl) removeFromIndexesUnlocked(key string) {
	// This is the same logic as RemoveFromIndexes but without locking
	if keyIndexes, exists := q.keyToIndexes[key]; exists {
		for indexName, indexValues := range keyIndexes {
			if indexData, indexExists := q.indexes[indexName]; indexExists {
				for _, indexValue := range indexValues {
					if keys, valueExists := indexData[indexValue]; valueExists {
						// Remove key from the slice
						newKeys := make([]string, 0, len(keys))
						for _, k := range keys {
							if k != key {
								newKeys = append(newKeys, k)
							}
						}
						
						if len(newKeys) == 0 {
							delete(indexData, indexValue)
						} else {
							indexData[indexValue] = newKeys
						}
					}
				}
			}
		}
		
		delete(q.keyToIndexes, key)
	}
}

// GetName returns the queue name for debugging and metrics
func (q *indexedQueueImpl) GetName() string {
	return q.name
}

// GetWorkspace returns the logical cluster workspace for this queue
func (q *indexedQueueImpl) GetWorkspace() logicalcluster.Name {
	return q.workspace
}