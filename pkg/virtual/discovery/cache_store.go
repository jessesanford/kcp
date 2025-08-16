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
	"fmt"
	"sync"
	"time"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
	"github.com/kcp-dev/logicalcluster/v3"
)

// CacheStore defines the interface for cache storage backends.
// This allows for different storage implementations (in-memory, Redis, etc.)
type CacheStore interface {
	// Get retrieves a cache entry by key
	Get(ctx context.Context, key string) (*StoredEntry, error)
	
	// Set stores a cache entry with expiration
	Set(ctx context.Context, key string, entry *StoredEntry, ttl time.Duration) error
	
	// Delete removes a cache entry
	Delete(ctx context.Context, key string) error
	
	// Clear removes all cache entries
	Clear(ctx context.Context) error
	
	// Keys returns all stored keys (for cleanup purposes)
	Keys(ctx context.Context) ([]string, error)
	
	// Close releases any resources held by the store
	Close() error
}

// StoredEntry represents a cache entry as stored in the backend
type StoredEntry struct {
	// Key is the cache key
	Key string `json:"key"`
	
	// Resources contains the cached resource information
	Resources []interfaces.ResourceInfo `json:"resources"`
	
	// ExpiresAt indicates when this entry expires
	ExpiresAt time.Time `json:"expires_at"`
	
	// LastAccessed tracks when this entry was last accessed
	LastAccessed time.Time `json:"last_accessed"`
	
	// Workspace identifies the workspace this entry belongs to
	Workspace string `json:"workspace"`
	
	// Version can be used for optimistic locking
	Version int64 `json:"version"`
}

// InMemoryCacheStore is an in-memory implementation of CacheStore
type InMemoryCacheStore struct {
	// entries stores the cached data
	entries map[string]*StoredEntry
	
	// mutex protects concurrent access
	mutex sync.RWMutex
}

// NewInMemoryCacheStore creates a new in-memory cache store
func NewInMemoryCacheStore() CacheStore {
	return &InMemoryCacheStore{
		entries: make(map[string]*StoredEntry),
	}
}

// Get retrieves a cache entry by key
func (s *InMemoryCacheStore) Get(ctx context.Context, key string) (*StoredEntry, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	entry, exists := s.entries[key]
	if !exists {
		return nil, fmt.Errorf("entry not found: %s", key)
	}
	
	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		return nil, fmt.Errorf("entry expired: %s", key)
	}
	
	// Create a copy to prevent external mutation
	return s.copyEntry(entry), nil
}

// Set stores a cache entry with expiration
func (s *InMemoryCacheStore) Set(ctx context.Context, key string, entry *StoredEntry, ttl time.Duration) error {
	if entry == nil {
		return fmt.Errorf("entry cannot be nil")
	}
	
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	now := time.Now()
	
	// Create a copy and set expiration
	storedEntry := s.copyEntry(entry)
	storedEntry.Key = key
	storedEntry.ExpiresAt = now.Add(ttl)
	storedEntry.LastAccessed = now
	storedEntry.Version++
	
	s.entries[key] = storedEntry
	return nil
}

// Delete removes a cache entry
func (s *InMemoryCacheStore) Delete(ctx context.Context, key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	delete(s.entries, key)
	return nil
}

// Clear removes all cache entries
func (s *InMemoryCacheStore) Clear(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.entries = make(map[string]*StoredEntry)
	return nil
}

// Keys returns all stored keys
func (s *InMemoryCacheStore) Keys(ctx context.Context) ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	keys := make([]string, 0, len(s.entries))
	for key := range s.entries {
		keys = append(keys, key)
	}
	
	return keys, nil
}

// Close releases resources (no-op for in-memory store)
func (s *InMemoryCacheStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.entries = nil
	return nil
}

// copyEntry creates a deep copy of a stored entry
func (s *InMemoryCacheStore) copyEntry(entry *StoredEntry) *StoredEntry {
	if entry == nil {
		return nil
	}
	
	// Copy resources slice
	resources := make([]interfaces.ResourceInfo, len(entry.Resources))
	copy(resources, entry.Resources)
	
	return &StoredEntry{
		Key:          entry.Key,
		Resources:    resources,
		ExpiresAt:    entry.ExpiresAt,
		LastAccessed: entry.LastAccessed,
		Workspace:    entry.Workspace,
		Version:      entry.Version,
	}
}

// CacheStoreManager manages cache store operations and provides higher-level functionality
type CacheStoreManager struct {
	// store is the underlying cache store
	store CacheStore
	
	// cleanupInterval controls how often expired entries are cleaned up
	cleanupInterval time.Duration
	
	// stopCh signals cleanup goroutine to stop
	stopCh chan struct{}
	
	// cleanupOnce ensures cleanup starts only once
	cleanupOnce sync.Once
}

// NewCacheStoreManager creates a new cache store manager
func NewCacheStoreManager(store CacheStore, cleanupInterval time.Duration) *CacheStoreManager {
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute
	}
	
	manager := &CacheStoreManager{
		store:           store,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}
	
	// Start cleanup goroutine
	manager.startCleanup()
	
	return manager
}

// GetResources retrieves cached resources for a workspace
func (m *CacheStoreManager) GetResources(ctx context.Context, workspace logicalcluster.Name) ([]interfaces.ResourceInfo, bool) {
	key := m.makeKey(workspace)
	
	entry, err := m.store.Get(ctx, key)
	if err != nil {
		RecordCacheHit(workspace.String(), false)
		return nil, false
	}
	
	RecordCacheHit(workspace.String(), true)
	return entry.Resources, true
}

// SetResources caches resources for a workspace
func (m *CacheStoreManager) SetResources(ctx context.Context, workspace logicalcluster.Name, resources []interfaces.ResourceInfo, ttl time.Duration) error {
	if len(resources) == 0 {
		return nil // Don't cache empty results
	}
	
	key := m.makeKey(workspace)
	entry := &StoredEntry{
		Key:       key,
		Resources: resources,
		Workspace: workspace.String(),
	}
	
	return m.store.Set(ctx, key, entry, ttl)
}

// InvalidateWorkspace removes cached data for a workspace
func (m *CacheStoreManager) InvalidateWorkspace(ctx context.Context, workspace logicalcluster.Name) error {
	key := m.makeKey(workspace)
	return m.store.Delete(ctx, key)
}

// Clear removes all cached data
func (m *CacheStoreManager) Clear(ctx context.Context) error {
	return m.store.Clear(ctx)
}

// Stop stops the cleanup goroutine and closes the store
func (m *CacheStoreManager) Stop() error {
	close(m.stopCh)
	return m.store.Close()
}

// makeKey creates a cache key for a workspace
func (m *CacheStoreManager) makeKey(workspace logicalcluster.Name) string {
	return fmt.Sprintf("discovery:%s", workspace.String())
}

// startCleanup starts the background cleanup process
func (m *CacheStoreManager) startCleanup() {
	m.cleanupOnce.Do(func() {
		go m.cleanupExpiredEntries()
	})
}

// cleanupExpiredEntries periodically removes expired entries
func (m *CacheStoreManager) cleanupExpiredEntries() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			m.performCleanup(ctx)
			cancel()
		case <-m.stopCh:
			return
		}
	}
}

// performCleanup removes expired entries from the store
func (m *CacheStoreManager) performCleanup(ctx context.Context) {
	keys, err := m.store.Keys(ctx)
	if err != nil {
		return
	}
	
	now := time.Now()
	for _, key := range keys {
		entry, err := m.store.Get(ctx, key)
		if err != nil {
			continue
		}
		
		if now.After(entry.ExpiresAt) {
			m.store.Delete(ctx, key)
		}
	}
}