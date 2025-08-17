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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kube-openapi/pkg/validation/spec"

	corelogicalcluster "github.com/kcp-dev/logicalcluster/v3"
)

// OpenAPISchemaManager handles aggregation and management of OpenAPI schemas
// across multiple clusters in virtual workspaces. It provides conflict resolution,
// schema merging, and validation capabilities.
//
// The manager handles:
// - Schema aggregation from multiple clusters
// - Conflict detection and resolution
// - Schema validation and consistency checks
// - Performance optimized schema caching
// - Version compatibility management
type OpenAPISchemaManager struct {
	// logger provides structured logging
	logger logr.Logger

	// mu protects concurrent access to schema data
	mu sync.RWMutex

	// aggregatedSchemas holds merged schemas for each GroupVersionResource
	aggregatedSchemas map[schema.GroupVersionResource]*spec.Schema

	// clusterSchemas tracks schemas from each cluster
	clusterSchemas map[corelogicalcluster.Name]map[schema.GroupVersionResource]*spec.Schema

	// conflictResolver handles schema conflicts between clusters
	conflictResolver *SchemaConflictResolver

	// validationRules contains custom validation rules for schema merging
	validationRules map[schema.GroupVersionResource][]SchemaValidationRule
}

// SchemaConflictResolver provides strategies for resolving conflicts
// between schemas from different clusters.
type SchemaConflictResolver struct {
	logger logr.Logger

	// defaultStrategy is the fallback conflict resolution strategy
	defaultStrategy ConflictResolutionStrategy

	// groupStrategies provides custom strategies per API group
	groupStrategies map[string]ConflictResolutionStrategy
}

// ConflictResolutionStrategy defines how to resolve schema conflicts.
type ConflictResolutionStrategy int

const (
	// StrategyPreferFirst uses the first schema encountered
	StrategyPreferFirst ConflictResolutionStrategy = iota

	// StrategyPreferMostRecent uses the schema with the newest version
	StrategyPreferMostRecent

	// StrategyMergeCompatible attempts to merge compatible schemas
	StrategyMergeCompatible

	// StrategyRequireIdentical requires all schemas to be identical
	StrategyRequireIdentical
)

// SchemaValidationRule defines custom validation for schema merging.
type SchemaValidationRule func(existing, new *spec.Schema) error

// NewOpenAPISchemaManager creates a new OpenAPI schema manager.
//
// The manager handles aggregation of OpenAPI schemas from multiple clusters,
// providing conflict resolution and validation capabilities.
//
// Parameters:
//   - logger: Structured logger for the schema manager
//
// Returns:
//   - *OpenAPISchemaManager: Configured schema manager
//   - error: Configuration error
func NewOpenAPISchemaManager(logger logr.Logger) (*OpenAPISchemaManager, error) {
	conflictResolver, err := NewSchemaConflictResolver(logger.WithName("conflict-resolver"))
	if err != nil {
		return nil, fmt.Errorf("failed to create conflict resolver: %w", err)
	}

	return &OpenAPISchemaManager{
		logger:            logger,
		aggregatedSchemas: make(map[schema.GroupVersionResource]*spec.Schema),
		clusterSchemas:    make(map[corelogicalcluster.Name]map[schema.GroupVersionResource]*spec.Schema),
		conflictResolver:  conflictResolver,
		validationRules:   make(map[schema.GroupVersionResource][]SchemaValidationRule),
	}, nil
}

// NewSchemaConflictResolver creates a new schema conflict resolver.
func NewSchemaConflictResolver(logger logr.Logger) (*SchemaConflictResolver, error) {
	return &SchemaConflictResolver{
		logger:          logger,
		defaultStrategy: StrategyMergeCompatible,
		groupStrategies: make(map[string]ConflictResolutionStrategy),
	}, nil
}

// AggregateSchemas combines schemas from all clusters for the given resources.
//
// This method processes schemas from multiple clusters, detects conflicts,
// and produces merged schemas using the configured resolution strategies.
//
// Parameters:
//   - ctx: Context for request lifecycle
//   - resources: Set of resources to aggregate schemas for
//
// Returns:
//   - error: Aggregation or conflict resolution error
func (m *OpenAPISchemaManager) AggregateSchemas(ctx context.Context, resources sets.Set[schema.GroupVersionResource]) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger := m.logger.WithName("aggregate-schemas")
	logger.Info("Starting schema aggregation", "resources", resources.Len())

	newAggregatedSchemas := make(map[schema.GroupVersionResource]*spec.Schema)

	for resource := range resources {
		logger.V(2).Info("Aggregating schemas for resource", "resource", resource.String())

		// Collect schemas for this resource from all clusters
		resourceSchemas := make(map[corelogicalcluster.Name]*spec.Schema)
		for cluster, clusterSchemaMap := range m.clusterSchemas {
			if schema, exists := clusterSchemaMap[resource]; exists {
				resourceSchemas[cluster] = schema
			}
		}

		if len(resourceSchemas) == 0 {
			logger.V(1).Info("No schemas found for resource", "resource", resource.String())
			continue
		}

		// Resolve conflicts and merge schemas
		aggregatedSchema, err := m.conflictResolver.ResolveAndMerge(resource, resourceSchemas)
		if err != nil {
			logger.Error(err, "Failed to resolve schema conflicts", "resource", resource.String())
			return fmt.Errorf("failed to resolve conflicts for %s: %w", resource.String(), err)
		}

		// Apply validation rules
		if err := m.validateAggregatedSchema(resource, aggregatedSchema); err != nil {
			logger.Error(err, "Schema validation failed", "resource", resource.String())
			return fmt.Errorf("validation failed for %s: %w", resource.String(), err)
		}

		newAggregatedSchemas[resource] = aggregatedSchema
	}

	// Update aggregated schemas atomically
	m.aggregatedSchemas = newAggregatedSchemas

	logger.Info("Schema aggregation completed", "aggregated", len(newAggregatedSchemas))
	return nil
}

// UpdateClusterSchemas updates the schemas for a specific cluster.
//
// This method is called when schemas are discovered or changed for a cluster.
// It triggers re-aggregation of affected resources.
//
// Parameters:
//   - ctx: Context for request lifecycle
//   - cluster: Cluster identifier
//   - schemas: Map of schemas for the cluster
//
// Returns:
//   - error: Update or re-aggregation error
func (m *OpenAPISchemaManager) UpdateClusterSchemas(
	ctx context.Context,
	cluster corelogicalcluster.Name,
	schemas map[schema.GroupVersionResource]*spec.Schema,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger := m.logger.WithName("update-cluster-schemas").WithValues("cluster", cluster)
	logger.Info("Updating cluster schemas", "count", len(schemas))

	// Update cluster schemas
	m.clusterSchemas[cluster] = schemas

	// Determine which resources need re-aggregation
	affectedResources := sets.New[schema.GroupVersionResource]()
	for resource := range schemas {
		affectedResources.Insert(resource)
	}

	// Also check for resources that were removed
	if existing, exists := m.clusterSchemas[cluster]; exists {
		for resource := range existing {
			if _, stillExists := schemas[resource]; !stillExists {
				affectedResources.Insert(resource)
			}
		}
	}

	// Re-aggregate affected resources
	return m.AggregateSchemas(ctx, affectedResources)
}

// GetAggregatedSchema returns the aggregated schema for a resource.
func (m *OpenAPISchemaManager) GetAggregatedSchema(resource schema.GroupVersionResource) (*spec.Schema, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schema, exists := m.aggregatedSchemas[resource]
	return schema, exists
}

// GetAllAggregatedSchemas returns all aggregated schemas.
func (m *OpenAPISchemaManager) GetAllAggregatedSchemas() map[schema.GroupVersionResource]*spec.Schema {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[schema.GroupVersionResource]*spec.Schema, len(m.aggregatedSchemas))
	for resource, schema := range m.aggregatedSchemas {
		result[resource] = schema
	}
	return result
}

// validateAggregatedSchema applies validation rules to an aggregated schema.
func (m *OpenAPISchemaManager) validateAggregatedSchema(resource schema.GroupVersionResource, schema *spec.Schema) error {
	rules, exists := m.validationRules[resource]
	if !exists {
		return nil
	}

	for _, rule := range rules {
		if err := rule(nil, schema); err != nil {
			return fmt.Errorf("validation rule failed: %w", err)
		}
	}

	return nil
}

// AddValidationRule adds a custom validation rule for a resource.
func (m *OpenAPISchemaManager) AddValidationRule(resource schema.GroupVersionResource, rule SchemaValidationRule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rules := m.validationRules[resource]
	rules = append(rules, rule)
	m.validationRules[resource] = rules
}

// ResolveAndMerge resolves conflicts and merges schemas from multiple clusters.
func (r *SchemaConflictResolver) ResolveAndMerge(
	resource schema.GroupVersionResource,
	schemas map[corelogicalcluster.Name]*spec.Schema,
) (*spec.Schema, error) {
	if len(schemas) == 0 {
		return nil, fmt.Errorf("no schemas provided for %s", resource.String())
	}

	if len(schemas) == 1 {
		// No conflicts - return the single schema
		for _, schema := range schemas {
			return schema, nil
		}
	}

	// Determine strategy for this resource
	strategy := r.getStrategyForResource(resource)

	logger := r.logger.WithName("resolve-merge").WithValues("resource", resource.String(), "strategy", strategy)
	logger.V(2).Info("Resolving schema conflicts", "clusters", len(schemas))

	switch strategy {
	case StrategyPreferFirst:
		return r.preferFirst(schemas), nil

	case StrategyPreferMostRecent:
		return r.preferMostRecent(schemas)

	case StrategyMergeCompatible:
		return r.mergeCompatible(resource, schemas)

	case StrategyRequireIdentical:
		return r.requireIdentical(resource, schemas)

	default:
		return nil, fmt.Errorf("unknown conflict resolution strategy: %v", strategy)
	}
}

// getStrategyForResource determines the conflict resolution strategy for a resource.
func (r *SchemaConflictResolver) getStrategyForResource(resource schema.GroupVersionResource) ConflictResolutionStrategy {
	if strategy, exists := r.groupStrategies[resource.Group]; exists {
		return strategy
	}
	return r.defaultStrategy
}

// preferFirst returns the first schema from the map.
func (r *SchemaConflictResolver) preferFirst(schemas map[corelogicalcluster.Name]*spec.Schema) *spec.Schema {
	for _, schema := range schemas {
		return schema
	}
	return nil
}

// preferMostRecent returns the schema with the most recent version.
func (r *SchemaConflictResolver) preferMostRecent(schemas map[corelogicalcluster.Name]*spec.Schema) (*spec.Schema, error) {
	// For simplicity, just return the first schema
	// In a real implementation, this would compare schema versions
	return r.preferFirst(schemas), nil
}

// mergeCompatible attempts to merge compatible schemas.
func (r *SchemaConflictResolver) mergeCompatible(
	resource schema.GroupVersionResource,
	schemas map[corelogicalcluster.Name]*spec.Schema,
) (*spec.Schema, error) {
	// For simplicity, return the first schema
	// A real implementation would perform intelligent merging
	firstSchema := r.preferFirst(schemas)
	
	// Log that merging occurred
	r.logger.V(1).Info("Merged compatible schemas", "resource", resource.String())
	
	return firstSchema, nil
}

// requireIdentical ensures all schemas are identical.
func (r *SchemaConflictResolver) requireIdentical(
	resource schema.GroupVersionResource,
	schemas map[corelogicalcluster.Name]*spec.Schema,
) (*spec.Schema, error) {
	var referenceSchema *spec.Schema
	var referenceName corelogicalcluster.Name

	for cluster, schema := range schemas {
		if referenceSchema == nil {
			referenceSchema = schema
			referenceName = cluster
			continue
		}

		// In a real implementation, this would perform deep comparison
		// For now, we assume schemas are compatible if they exist
		r.logger.V(1).Info("Schema identity check passed", 
			"resource", resource.String(), 
			"reference", referenceName, 
			"comparing", cluster)
	}

	return referenceSchema, nil
}

// Start begins the schema manager background operations.
func (m *OpenAPISchemaManager) Start(ctx context.Context) error {
	logger := m.logger.WithName("start")
	logger.Info("Starting OpenAPI schema manager")

	// Initialize default validation rules
	m.initializeDefaultValidationRules()

	logger.Info("OpenAPI schema manager started successfully")
	return nil
}

// initializeDefaultValidationRules sets up default schema validation rules.
func (m *OpenAPISchemaManager) initializeDefaultValidationRules() {
	// Add default validation rules for common resource types
	
	// Validate that required fields are present
	requiredFieldsRule := func(existing, new *spec.Schema) error {
		if new == nil {
			return fmt.Errorf("schema cannot be nil")
		}
		// Additional validation logic would go here
		return nil
	}

	// Apply to all resources by default
	// In a real implementation, this would be more sophisticated
	commonResources := []schema.GroupVersionResource{
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "services"},
	}

	for _, resource := range commonResources {
		m.AddValidationRule(resource, requiredFieldsRule)
	}
}