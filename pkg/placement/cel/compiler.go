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

package cel

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// memoryCache provides an in-memory cache for compiled CEL expressions.
type memoryCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
}

// cacheEntry represents a cache entry with metadata.
type cacheEntry struct {
	expr      *CompiledExpression
	createdAt time.Time
	lastUsed  time.Time
	useCount  int64
}

// NewMemoryCache creates a new in-memory expression cache.
func NewMemoryCache() ExpressionCache {
	return &memoryCache{
		entries: make(map[string]*cacheEntry),
		maxSize: 1000, // Default max size
	}
}

// NewMemoryCacheWithSize creates a new in-memory expression cache with specified size.
func NewMemoryCacheWithSize(maxSize int) ExpressionCache {
	return &memoryCache{
		entries: make(map[string]*cacheEntry),
		maxSize: maxSize,
	}
}

// Get retrieves a compiled expression from the cache.
func (c *memoryCache) Get(hash string) (*CompiledExpression, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[hash]
	if !ok {
		return nil, false
	}

	// Update access statistics
	c.mu.RUnlock()
	c.mu.Lock()
	entry.lastUsed = time.Now()
	entry.useCount++
	c.mu.Unlock()
	c.mu.RLock()

	return entry.expr, true
}

// Set stores a compiled expression in the cache.
func (c *memoryCache) Set(hash string, expr *CompiledExpression) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict entries
	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	now := time.Now()
	c.entries[hash] = &cacheEntry{
		expr:      expr,
		createdAt: now,
		lastUsed:  now,
		useCount:  1,
	}
}

// Delete removes an expression from the cache.
func (c *memoryCache) Delete(hash string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, hash)
}

// Clear removes all expressions from the cache.
func (c *memoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// Size returns the number of cached expressions.
func (c *memoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// evictLRU evicts the least recently used entry from the cache.
func (c *memoryCache) evictLRU() {
	var oldestHash string
	var oldestTime time.Time

	for hash, entry := range c.entries {
		if oldestHash == "" || entry.lastUsed.Before(oldestTime) {
			oldestHash = hash
			oldestTime = entry.lastUsed
		}
	}

	if oldestHash != "" {
		delete(c.entries, oldestHash)
	}
}

// GetStats returns cache statistics.
func (c *memoryCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		Size:    len(c.entries),
		MaxSize: c.maxSize,
	}

	now := time.Now()
	for _, entry := range c.entries {
		stats.TotalHits += entry.useCount
		if now.Sub(entry.lastUsed) < 5*time.Minute {
			stats.RecentHits++
		}
	}

	return stats
}

// CacheStats provides statistics about cache performance.
type CacheStats struct {
	// Size is the current number of cached expressions
	Size int
	
	// MaxSize is the maximum number of expressions that can be cached
	MaxSize int
	
	// TotalHits is the total number of cache hits
	TotalHits int64
	
	// RecentHits is the number of hits in the last 5 minutes
	RecentHits int64
}

// hashExpression creates a hash of the expression string for cache keys.
func hashExpression(expr string) string {
	hash := sha256.Sum256([]byte(expr))
	return fmt.Sprintf("%x", hash)
}

// CompilerOptions provides options for expression compilation.
type CompilerOptions struct {
	// EnableMacros enables CEL macro expansion
	EnableMacros bool
	
	// EnableOptimizations enables expression optimizations
	EnableOptimizations bool
	
	// MaxComplexity limits expression complexity
	MaxComplexity int
	
	// CheckBounds enables bounds checking for array/map access
	CheckBounds bool
}

// ExpressionCompiler provides advanced compilation features.
type ExpressionCompiler struct {
	evaluator CELEvaluator
	options   *CompilerOptions
	cache     ExpressionCache
}

// NewExpressionCompiler creates a new expression compiler.
func NewExpressionCompiler(evaluator CELEvaluator, opts *CompilerOptions) *ExpressionCompiler {
	if opts == nil {
		opts = &CompilerOptions{
			EnableMacros:        true,
			EnableOptimizations: true,
			MaxComplexity:       100,
			CheckBounds:         true,
		}
	}

	return &ExpressionCompiler{
		evaluator: evaluator,
		options:   opts,
		cache:     NewMemoryCache(),
	}
}

// CompileWithOptions compiles an expression with specific options.
func (c *ExpressionCompiler) CompileWithOptions(expr string, opts *CompilerOptions) (*CompiledExpression, error) {
	if opts == nil {
		opts = c.options
	}

	// Create hash including options to ensure proper caching
	hash := c.hashExpressionWithOptions(expr, opts)

	// Check cache
	if cached, ok := c.cache.Get(hash); ok {
		return cached, nil
	}

	// Compile with the evaluator
	compiled, err := c.evaluator.CompileExpression(expr)
	if err != nil {
		return nil, err
	}

	// Update hash to include options
	compiled.Hash = hash

	// Cache the result
	c.cache.Set(hash, compiled)

	return compiled, nil
}

// ValidateExpressionSyntax performs syntax validation without full compilation.
func (c *ExpressionCompiler) ValidateExpressionSyntax(expr string) *ValidationResult {
	env := c.evaluator.GetEnvironment()
	return ValidateExpression(expr, env)
}

// hashExpressionWithOptions creates a hash including compilation options.
func (c *ExpressionCompiler) hashExpressionWithOptions(expr string, opts *CompilerOptions) string {
	data := fmt.Sprintf("%s|macros:%t|opt:%t|complexity:%d|bounds:%t",
		expr, opts.EnableMacros, opts.EnableOptimizations, opts.MaxComplexity, opts.CheckBounds)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// PrecompileExpressions precompiles a batch of expressions for better performance.
func (c *ExpressionCompiler) PrecompileExpressions(expressions []string) ([]*CompiledExpression, []error) {
	compiled := make([]*CompiledExpression, len(expressions))
	errors := make([]error, len(expressions))

	for i, expr := range expressions {
		comp, err := c.CompileWithOptions(expr, nil)
		compiled[i] = comp
		errors[i] = err
	}

	return compiled, errors
}

// GetCacheStats returns cache statistics for monitoring.
func (c *ExpressionCompiler) GetCacheStats() CacheStats {
	if mc, ok := c.cache.(*memoryCache); ok {
		return mc.GetStats()
	}
	return CacheStats{
		Size:    c.cache.Size(),
		MaxSize: -1, // Unknown for non-memory caches
	}
}

// ClearCache clears the compilation cache.
func (c *ExpressionCompiler) ClearCache() {
	c.cache.Clear()
}