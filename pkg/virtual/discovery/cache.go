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
	"sync"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
	"github.com/kcp-dev/logicalcluster/v3"
)

// StubDiscoveryCache is a temporary stub implementation for the discovery cache.
// This allows the core provider to compile and function during the PR split process.
// The actual cache implementation will be added in the cache PR.
type StubDiscoveryCache struct {
	// simple in-memory storage for the stub
	data  map[string][]interfaces.ResourceInfo
	mutex sync.RWMutex
}

// NewStubDiscoveryCache creates a new stub discovery cache.
func NewStubDiscoveryCache() interfaces.DiscoveryCache {
	return &StubDiscoveryCache{
		data: make(map[string][]interfaces.ResourceInfo),
	}
}

// GetResources retrieves cached resources for a workspace.
// This stub implementation always returns cache miss.
func (c *StubDiscoveryCache) GetResources(workspace logicalcluster.Name) ([]interfaces.ResourceInfo, bool) {
	// Stub: always return cache miss to force fresh discovery
	return nil, false
}

// SetResources caches resources for a workspace with TTL.
// This stub implementation stores data but ignores TTL.
func (c *StubDiscoveryCache) SetResources(workspace logicalcluster.Name, resources []interfaces.ResourceInfo, ttl int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Simple storage without TTL handling
	c.data[workspace.String()] = resources
}

// InvalidateWorkspace removes cached data for a workspace.
func (c *StubDiscoveryCache) InvalidateWorkspace(workspace logicalcluster.Name) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	delete(c.data, workspace.String())
}

// Clear removes all cached data.
func (c *StubDiscoveryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.data = make(map[string][]interfaces.ResourceInfo)
}