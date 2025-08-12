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
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestNewIndexedQueue(t *testing.T) {
	workspace := logicalcluster.Name("test-workspace")
	queue := NewIndexedQueue("test-queue", workspace)
	
	if queue == nil {
		t.Fatal("NewIndexedQueue returned nil")
	}
	
	impl, ok := queue.(*indexedQueueImpl)
	if !ok {
		t.Fatal("NewIndexedQueue did not return indexedQueueImpl")
	}
	
	if impl.name != "test-queue" {
		t.Errorf("Expected queue name 'test-queue', got %q", impl.name)
	}
	
	if impl.workspace != workspace {
		t.Errorf("Expected workspace %v, got %v", workspace, impl.workspace)
	}
	
	if impl.indexers == nil {
		t.Error("indexers map should be initialized")
	}
	
	if impl.indexes == nil {
		t.Error("indexes map should be initialized")
	}
	
	if impl.keyToIndexes == nil {
		t.Error("keyToIndexes map should be initialized")
	}
}

func TestIndexedQueue_AddIndexer(t *testing.T) {
	tests := map[string]struct {
		indexName string
		indexFunc IndexFunc
		wantError bool
	}{
		"valid indexer": {
			indexName: "test-index",
			indexFunc: func(key string) ([]string, error) {
				return []string{"value1"}, nil
			},
			wantError: false,
		},
		"duplicate indexer": {
			indexName: "duplicate",
			indexFunc: func(key string) ([]string, error) {
				return []string{"value2"}, nil
			},
			wantError: false, // Should replace existing
		},
	}
	
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.indexName == "duplicate" {
				// Add the indexer first to test replacement
				_ = queue.AddIndexer("duplicate", func(key string) ([]string, error) {
					return []string{"old-value"}, nil
				})
			}
			
			err := queue.AddIndexer(tc.indexName, tc.indexFunc)
			
			if (err != nil) != tc.wantError {
				t.Errorf("AddIndexer() error = %v, wantError %v", err, tc.wantError)
			}
			
			if !tc.wantError && !queue.HasIndex(tc.indexName) {
				t.Errorf("Index %q was not added", tc.indexName)
			}
		})
	}
}

func TestIndexedQueue_AddAndIndex(t *testing.T) {
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	// Add a simple indexer that extracts the first character
	err := queue.AddIndexer("byFirstChar", func(key string) ([]string, error) {
		if len(key) == 0 {
			return []string{}, nil
		}
		return []string{string(key[0])}, nil
	})
	if err != nil {
		t.Fatalf("Failed to add indexer: %v", err)
	}
	
	// Add some keys
	testKeys := []string{"apple", "banana", "avocado", "blueberry"}
	for _, key := range testKeys {
		queue.Add(key)
	}
	
	// Test retrieval by index
	aKeys, err := queue.GetByIndex("byFirstChar", "a")
	if err != nil {
		t.Fatalf("GetByIndex failed: %v", err)
	}
	
	expectedAKeys := []string{"apple", "avocado"}
	sort.Strings(aKeys)
	sort.Strings(expectedAKeys)
	
	if !reflect.DeepEqual(aKeys, expectedAKeys) {
		t.Errorf("Expected keys %v for 'a', got %v", expectedAKeys, aKeys)
	}
	
	bKeys, err := queue.GetByIndex("byFirstChar", "b")
	if err != nil {
		t.Fatalf("GetByIndex failed: %v", err)
	}
	
	expectedBKeys := []string{"banana", "blueberry"}
	sort.Strings(bKeys)
	sort.Strings(expectedBKeys)
	
	if !reflect.DeepEqual(bKeys, expectedBKeys) {
		t.Errorf("Expected keys %v for 'b', got %v", expectedBKeys, bKeys)
	}
}

func TestIndexedQueue_GetByIndex_NonExistentIndex(t *testing.T) {
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	_, err := queue.GetByIndex("nonexistent", "value")
	if !IsIndexNotFound(err) {
		t.Errorf("Expected IndexNotFoundError, got %v", err)
	}
}

func TestIndexedQueue_GetByIndex_NonExistentValue(t *testing.T) {
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	err := queue.AddIndexer("test", func(key string) ([]string, error) {
		return []string{"existing"}, nil
	})
	if err != nil {
		t.Fatalf("Failed to add indexer: %v", err)
	}
	
	keys, err := queue.GetByIndex("test", "nonexistent")
	if err != nil {
		t.Fatalf("GetByIndex failed: %v", err)
	}
	
	if len(keys) != 0 {
		t.Errorf("Expected empty slice for nonexistent value, got %v", keys)
	}
}

func TestIndexedQueue_ListIndexValues(t *testing.T) {
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	// Add indexer
	err := queue.AddIndexer("byFirstChar", func(key string) ([]string, error) {
		if len(key) == 0 {
			return []string{}, nil
		}
		return []string{string(key[0])}, nil
	})
	if err != nil {
		t.Fatalf("Failed to add indexer: %v", err)
	}
	
	// Add keys
	testKeys := []string{"apple", "banana", "cherry"}
	for _, key := range testKeys {
		queue.Add(key)
	}
	
	values := queue.ListIndexValues("byFirstChar")
	expectedValues := []string{"a", "b", "c"}
	
	sort.Strings(values)
	sort.Strings(expectedValues)
	
	if !reflect.DeepEqual(values, expectedValues) {
		t.Errorf("Expected index values %v, got %v", expectedValues, values)
	}
}

func TestIndexedQueue_ListIndexValues_NonExistentIndex(t *testing.T) {
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	values := queue.ListIndexValues("nonexistent")
	if len(values) != 0 {
		t.Errorf("Expected empty slice for nonexistent index, got %v", values)
	}
}

func TestIndexedQueue_RemoveFromIndexes(t *testing.T) {
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	// Add indexer
	err := queue.AddIndexer("byFirstChar", func(key string) ([]string, error) {
		if len(key) == 0 {
			return []string{}, nil
		}
		return []string{string(key[0])}, nil
	})
	if err != nil {
		t.Fatalf("Failed to add indexer: %v", err)
	}
	
	// Add keys
	queue.Add("apple")
	queue.Add("avocado")
	
	// Verify keys are indexed
	aKeys, _ := queue.GetByIndex("byFirstChar", "a")
	if len(aKeys) != 2 {
		t.Errorf("Expected 2 keys for 'a', got %d", len(aKeys))
	}
	
	// Remove one key from indexes
	queue.RemoveFromIndexes("apple")
	
	// Verify key is removed from index
	aKeys, _ = queue.GetByIndex("byFirstChar", "a")
	if len(aKeys) != 1 {
		t.Errorf("Expected 1 key for 'a' after removal, got %d", len(aKeys))
	}
	
	if aKeys[0] != "avocado" {
		t.Errorf("Expected remaining key 'avocado', got %q", aKeys[0])
	}
}

func TestIndexedQueue_MultipleIndexValues(t *testing.T) {
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	// Add indexer that returns multiple values
	err := queue.AddIndexer("byChars", func(key string) ([]string, error) {
		var result []string
		for _, char := range key {
			result = append(result, string(char))
		}
		return result, nil
	})
	if err != nil {
		t.Fatalf("Failed to add indexer: %v", err)
	}
	
	// Add a key
	queue.Add("abc")
	
	// Check that key appears in multiple index values
	for _, char := range []string{"a", "b", "c"} {
		keys, err := queue.GetByIndex("byChars", char)
		if err != nil {
			t.Fatalf("GetByIndex failed for %q: %v", char, err)
		}
		
		if len(keys) != 1 || keys[0] != "abc" {
			t.Errorf("Expected key 'abc' for index value %q, got %v", char, keys)
		}
	}
}

func TestIndexedQueue_ErrorInIndexFunc(t *testing.T) {
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	// Add indexer that returns an error for certain keys
	err := queue.AddIndexer("errorIndex", func(key string) ([]string, error) {
		if key == "error" {
			return nil, NewInvalidKeyError(key, "test error")
		}
		return []string{"ok"}, nil
	})
	if err != nil {
		t.Fatalf("Failed to add indexer: %v", err)
	}
	
	// Add normal key
	queue.Add("normal")
	
	// Add error key
	queue.Add("error")
	
	// Check that normal key is indexed
	keys, err := queue.GetByIndex("errorIndex", "ok")
	if err != nil {
		t.Fatalf("GetByIndex failed: %v", err)
	}
	
	if len(keys) != 1 || keys[0] != "normal" {
		t.Errorf("Expected 'normal' key, got %v", keys)
	}
	
	// Check that error key is not indexed
	allValues := queue.ListIndexValues("errorIndex")
	if len(allValues) != 1 || allValues[0] != "ok" {
		t.Errorf("Expected only 'ok' index value, got %v", allValues)
	}
}

func TestIndexedQueue_HasIndex(t *testing.T) {
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	if queue.HasIndex("nonexistent") {
		t.Error("HasIndex should return false for nonexistent index")
	}
	
	err := queue.AddIndexer("existing", func(key string) ([]string, error) {
		return []string{"value"}, nil
	})
	if err != nil {
		t.Fatalf("Failed to add indexer: %v", err)
	}
	
	if !queue.HasIndex("existing") {
		t.Error("HasIndex should return true for existing index")
	}
}

func TestIndexedQueue_IntegrationWithBaseQueue(t *testing.T) {
	workspace := logicalcluster.Name("test")
	queue := NewIndexedQueue("test", workspace)
	
	// Add indexer
	err := queue.AddIndexer("byFirstChar", func(key string) ([]string, error) {
		if len(key) == 0 {
			return []string{}, nil
		}
		return []string{string(key[0])}, nil
	})
	if err != nil {
		t.Fatalf("Failed to add indexer: %v", err)
	}
	
	// Test different add methods
	queue.Add("add")
	queue.AddRateLimited("addRateLimited")
	queue.AddAfter("addAfter", 1*time.Millisecond)
	
	// Wait a bit for AddAfter
	time.Sleep(10 * time.Millisecond)
	
	// Check that all keys are indexed
	aKeys, err := queue.GetByIndex("byFirstChar", "a")
	if err != nil {
		t.Fatalf("GetByIndex failed: %v", err)
	}
	
	expectedKeys := []string{"add", "addRateLimited", "addAfter"}
	sort.Strings(aKeys)
	sort.Strings(expectedKeys)
	
	if !reflect.DeepEqual(aKeys, expectedKeys) {
		t.Errorf("Expected keys %v, got %v", expectedKeys, aKeys)
	}
	
	// Check that queue length is correct
	if queue.Len() != 3 {
		t.Errorf("Expected queue length 3, got %d", queue.Len())
	}
}