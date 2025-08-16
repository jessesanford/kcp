/*
Copyright The KCP Authors.

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

// Package collector implements status collection from multiple sources in the TMC system.
package collector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/kcp-dev/kcp/pkg/status/interfaces"
)

// Collector implements the StatusCollector interface for gathering status
// information from multiple sources in a thread-safe manner
type Collector struct {
	// mu protects access to sources
	mu sync.RWMutex

	// sources maps source names to their configurations
	sources map[string]interfaces.SourceConfig

	// clients maps source names to their client instances
	clients map[string]StatusClient

	// defaultTimeout is used when source doesn't specify a timeout
	defaultTimeout time.Duration
}

// StatusClient defines the interface for communicating with status sources
type StatusClient interface {
	// GetStatus retrieves status from the source
	GetStatus(ctx context.Context, gvr schema.GroupVersionResource, key types.NamespacedName) (*unstructured.Unstructured, error)

	// Close closes the client connection
	Close() error
}

// CollectorConfig contains configuration for the status collector
type CollectorConfig struct {
	// DefaultTimeout is used when sources don't specify a timeout
	DefaultTimeout time.Duration

	// MaxConcurrentCollections limits concurrent status collections
	MaxConcurrentCollections int
}

// NewCollector creates a new status collector instance
func NewCollector(config CollectorConfig) *Collector {
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}

	return &Collector{
		sources:        make(map[string]interfaces.SourceConfig),
		clients:        make(map[string]StatusClient),
		defaultTimeout: config.DefaultTimeout,
	}
}

// CollectStatus gathers status from a specific source for the given resource
func (c *Collector) CollectStatus(ctx context.Context, source string, gvr schema.GroupVersionResource, key types.NamespacedName) (*interfaces.StatusUpdate, error) {
	c.mu.RLock()
	sourceConfig, exists := c.sources[source]
	client, hasClient := c.clients[source]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("source %q not found", source)
	}

	if !hasClient {
		return nil, fmt.Errorf("no client available for source %q", source)
	}

	// Set timeout for the collection
	timeout := sourceConfig.Timeout
	if timeout == 0 {
		timeout = c.defaultTimeout
	}

	collectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Collect status with retry policy
	status, err := c.collectWithRetry(collectCtx, client, gvr, key, sourceConfig.RetryPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to collect status from source %q: %w", source, err)
	}

	// Create status update
	update := &interfaces.StatusUpdate{
		Source:          source,
		Timestamp:       time.Now(),
		ResourceVersion: getResourceVersion(status),
		Status:          status,
		Metadata:        sourceConfig.Metadata,
	}

	klog.V(4).InfoS("Successfully collected status", 
		"source", source, 
		"gvr", gvr.String(), 
		"key", key.String(),
		"resourceVersion", update.ResourceVersion)

	return update, nil
}

// CollectAllStatus gathers status from all configured sources for the given resource
func (c *Collector) CollectAllStatus(ctx context.Context, gvr schema.GroupVersionResource, key types.NamespacedName) ([]*interfaces.StatusUpdate, error) {
	c.mu.RLock()
	sourceNames := make([]string, 0, len(c.sources))
	for source := range c.sources {
		sourceNames = append(sourceNames, source)
	}
	c.mu.RUnlock()

	if len(sourceNames) == 0 {
		return nil, fmt.Errorf("no sources configured")
	}

	// Collect from all sources concurrently
	type result struct {
		update *interfaces.StatusUpdate
		err    error
		source string
	}

	resultCh := make(chan result, len(sourceNames))
	var wg sync.WaitGroup

	for _, source := range sourceNames {
		wg.Add(1)
		go func(sourceName string) {
			defer wg.Done()
			update, err := c.CollectStatus(ctx, sourceName, gvr, key)
			resultCh <- result{
				update: update,
				err:    err,
				source: sourceName,
			}
		}(source)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results
	var updates []*interfaces.StatusUpdate
	var errors []error

	for res := range resultCh {
		if res.err != nil {
			klog.V(2).ErrorS(res.err, "Failed to collect status from source", "source", res.source)
			errors = append(errors, fmt.Errorf("source %q: %w", res.source, res.err))
		} else {
			updates = append(updates, res.update)
		}
	}

	// Return partial results if we got at least one successful collection
	if len(updates) > 0 {
		if len(errors) > 0 {
			klog.V(2).InfoS("Partial status collection completed", 
				"successful", len(updates), 
				"failed", len(errors),
				"gvr", gvr.String(),
				"key", key.String())
		}
		return updates, nil
	}

	// If no successful collections, return all errors
	if len(errors) == 1 {
		return nil, errors[0]
	}
	return nil, fmt.Errorf("all status collections failed: %v", errors)
}

// RegisterSource adds a new source to collect status from
func (c *Collector) RegisterSource(source string, config interfaces.SourceConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.sources[source]; exists {
		return fmt.Errorf("source %q already registered", source)
	}

	// Validate configuration
	if err := c.validateSourceConfig(config); err != nil {
		return fmt.Errorf("invalid source config for %q: %w", source, err)
	}

	// Create client for the source
	client, err := c.createClient(source, config)
	if err != nil {
		return fmt.Errorf("failed to create client for source %q: %w", source, err)
	}

	c.sources[source] = config
	c.clients[source] = client

	klog.V(2).InfoS("Registered status collection source", 
		"source", source, 
		"endpoint", config.Endpoint,
		"priority", config.Priority)

	return nil
}

// UnregisterSource removes a source from collection
func (c *Collector) UnregisterSource(source string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	client, hasClient := c.clients[source]
	if hasClient {
		if err := client.Close(); err != nil {
			klog.V(2).ErrorS(err, "Failed to close client for source", "source", source)
		}
		delete(c.clients, source)
	}

	delete(c.sources, source)

	klog.V(2).InfoS("Unregistered status collection source", "source", source)
	return nil
}

// Sources returns all registered sources
func (c *Collector) Sources() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sources := make([]string, 0, len(c.sources))
	for source := range c.sources {
		sources = append(sources, source)
	}
	return sources
}

// collectWithRetry implements retry logic for status collection
func (c *Collector) collectWithRetry(ctx context.Context, client StatusClient, gvr schema.GroupVersionResource, key types.NamespacedName, policy interfaces.RetryPolicy) (*unstructured.Unstructured, error) {
	var lastErr error
	
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		status, err := client.GetStatus(ctx, gvr, key)
		if err == nil {
			return status, nil
		}
		
		lastErr = err
		
		// If this was the last attempt, don't wait
		if attempt == policy.MaxRetries {
			break
		}
		
		// Calculate delay for next attempt
		delay := c.calculateDelay(attempt, policy)
		
		klog.V(4).InfoS("Status collection attempt failed, retrying",
			"attempt", attempt+1,
			"maxRetries", policy.MaxRetries+1,
			"delay", delay,
			"error", err)
		
		// Wait before next attempt, respecting context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}
	
	return nil, fmt.Errorf("failed after %d attempts: %w", policy.MaxRetries+1, lastErr)
}

// calculateDelay computes the retry delay using exponential backoff
func (c *Collector) calculateDelay(attempt int, policy interfaces.RetryPolicy) time.Duration {
	if policy.InitialDelay == 0 {
		policy.InitialDelay = time.Second
	}
	if policy.BackoffFactor == 0 {
		policy.BackoffFactor = 2.0
	}
	
	delay := policy.InitialDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * policy.BackoffFactor)
	}
	
	if policy.MaxDelay > 0 && delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}
	
	return delay
}

// validateSourceConfig validates source configuration
func (c *Collector) validateSourceConfig(config interfaces.SourceConfig) error {
	if config.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	
	if config.RetryPolicy.MaxRetries < 0 {
		return fmt.Errorf("maxRetries cannot be negative")
	}
	
	if config.RetryPolicy.BackoffFactor < 0 {
		return fmt.Errorf("backoffFactor cannot be negative")
	}
	
	return nil
}

// createClient creates a client for the given source
func (c *Collector) createClient(source string, config interfaces.SourceConfig) (StatusClient, error) {
	// For now, return a mock client
	// In a real implementation, this would create appropriate clients
	// based on the source configuration (HTTP, gRPC, etc.)
	return &MockStatusClient{
		endpoint: config.Endpoint,
		timeout:  config.Timeout,
	}, nil
}

// getResourceVersion extracts the resource version from status
func getResourceVersion(status *unstructured.Unstructured) string {
	if status == nil {
		return ""
	}
	
	rv, _, _ := unstructured.NestedString(status.Object, "metadata", "resourceVersion")
	return rv
}

// MockStatusClient is a mock implementation for testing
type MockStatusClient struct {
	endpoint string
	timeout  time.Duration
}

// GetStatus implements StatusClient interface for testing
func (m *MockStatusClient) GetStatus(ctx context.Context, gvr schema.GroupVersionResource, key types.NamespacedName) (*unstructured.Unstructured, error) {
	// Mock implementation - in real code this would make actual API calls
	status := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": gvr.GroupVersion().String(),
			"kind":       gvr.Resource, // This would be properly derived
			"metadata": map[string]interface{}{
				"name":            key.Name,
				"namespace":       key.Namespace,
				"resourceVersion": fmt.Sprintf("mock-%d", time.Now().Unix()),
			},
			"status": map[string]interface{}{
				"phase": "Active",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "True",
						"lastTransitionTime": time.Now().Format(time.RFC3339),
						"reason":             "MockStatus",
						"message":            fmt.Sprintf("Mock status from %s", m.endpoint),
					},
				},
			},
		},
	}
	
	return status, nil
}

// Close implements StatusClient interface
func (m *MockStatusClient) Close() error {
	return nil
}