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
	"time"

	"github.com/kcp-dev/kcp/pkg/policy/types"
)

// ExpressionCache provides caching for compiled CEL expressions to improve performance.
// Implementations should be thread-safe and support concurrent access.
type ExpressionCache interface {
	// Get retrieves a compiled expression from cache by key.
	// Returns the expression and true if found, nil and false if not cached.
	Get(key string) (CompiledExpression, bool)

	// Put stores a compiled expression in cache with specified TTL.
	// TTL of 0 means no expiration. Returns error if storage fails.
	Put(key string, expr CompiledExpression, ttl time.Duration) error

	// Evict removes a specific expression from cache.
	// Returns true if the key existed and was removed.
	Evict(key string) bool

	// Clear removes all cached expressions.
	// Returns error if clearing fails.
	Clear() error

	// Stats returns current cache performance statistics.
	Stats() types.CacheStats
}

// ResultCache caches policy evaluation results to avoid repeated computation.
// This is separate from expression caching and focuses on evaluation outcomes.
type ResultCache interface {
	// GetResult retrieves a cached evaluation result by key.
	// Returns the result and true if found, nil and false if not cached.
	GetResult(ctx context.Context, key string) (*types.EvaluationResult, bool)

	// PutResult stores an evaluation result in cache with specified TTL.
	// Context allows for request-scoped caching behavior.
	PutResult(ctx context.Context, key string,
		result *types.EvaluationResult, ttl time.Duration) error

	// InvalidatePattern removes all cached results matching a pattern.
	// Useful for invalidating related results when policies change.
	InvalidatePattern(pattern string) error
}

// CacheKey generates consistent cache keys for expressions and results.
// Different implementations may use different strategies for key generation.
type CacheKey interface {
	// Generate creates a cache key from expression and variable context.
	// Should produce consistent keys for identical inputs.
	Generate(expression string, variables map[string]interface{}) string
}