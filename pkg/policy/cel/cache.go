package cel

import (
	"context"
	"sync"
	"time"

	"github.com/kcp-dev/kcp/pkg/policy/interfaces"
)

// ExpressionCache caches compiled expressions
type ExpressionCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
	stats   interfaces.CacheStats
}

type cacheEntry struct {
	expr      interfaces.CompiledExpression
	timestamp time.Time
	hits      int64
}

// NewExpressionCache creates a new expression cache
func NewExpressionCache() *ExpressionCache {
	return &ExpressionCache{
		entries: make(map[string]*cacheEntry),
		maxSize: 1000,
	}
}

// Get retrieves a cached expression
func (c *ExpressionCache) Get(ctx context.Context, key string) (interfaces.CompiledExpression, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, ok := c.entries[key]
	if !ok {
		c.stats.Misses++
		return nil, false
	}
	
	entry.hits++
	c.stats.Hits++
	return entry.expr, true
}

// Put stores a compiled expression
func (c *ExpressionCache) Put(ctx context.Context, key string, expr interfaces.CompiledExpression, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple eviction if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}
	
	c.entries[key] = &cacheEntry{
		expr:      expr,
		timestamp: time.Now(),
	}
	
	c.stats.Size = int64(len(c.entries))
	return nil
}

// Stats returns cache statistics
func (c *ExpressionCache) Stats(ctx context.Context) (*interfaces.CacheStats, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Return a copy of the stats
	stats := c.stats
	stats.Size = int64(len(c.entries))
	return &stats, nil
}

// Clear removes all cached expressions
func (c *ExpressionCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries = make(map[string]*cacheEntry)
	c.stats = interfaces.CacheStats{}
	return nil
}

// evictOldest removes the oldest cache entry
func (c *ExpressionCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	
	for k, v := range c.entries {
		if oldestKey == "" || v.timestamp.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.timestamp
		}
	}
	
	if oldestKey != "" {
		delete(c.entries, oldestKey)
		c.stats.Evictions++
	}
}