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
	"encoding/json"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// SchemaManager manages OpenAPI schemas for discovered resources in virtual workspaces.
// It maintains a registry of schemas indexed by GroupVersionResource and provides
// functionality for schema validation, merging, and OpenAPI document generation.
type SchemaManager struct {
	mu      sync.RWMutex
	schemas map[string]*spec.Schema
	merged  *spec.Swagger
}

// NewSchemaManager creates a new schema manager with an initialized OpenAPI document structure.
// The manager starts with an empty schema registry and a basic OpenAPI specification.
func NewSchemaManager() *SchemaManager {
	return &SchemaManager{
		schemas: make(map[string]*spec.Schema),
		merged: &spec.Swagger{
			SwaggerProps: spec.SwaggerProps{
				Swagger: "2.0",
				Info: &spec.Info{
					InfoProps: spec.InfoProps{
						Title:   "Virtual Workspace API",
						Version: "v1alpha1",
					},
				},
				Paths: &spec.Paths{
					Paths: make(map[string]spec.PathItem),
				},
				Definitions: spec.Definitions{},
			},
		},
	}
}

// AddSchema adds an OpenAPI schema for a specific GroupVersionResource.
// The schema bytes should contain valid JSON that can be unmarshaled into an OpenAPI schema.
// Returns an error if the schema cannot be parsed.
func (sm *SchemaManager) AddSchema(gvr schema.GroupVersionResource, schemaBytes []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var schema spec.Schema
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return fmt.Errorf("failed to unmarshal schema for %s: %w", gvr.String(), err)
	}

	key := gvr.String()
	sm.schemas[key] = &schema

	// Update the merged OpenAPI document with the new schema
	sm.updateMergedSchema()

	return nil
}

// GetSchema retrieves the OpenAPI schema for a specific GroupVersionResource.
// Returns the schema and true if found, or nil and false if not found.
func (sm *SchemaManager) GetSchema(gvr schema.GroupVersionResource) (*spec.Schema, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	schema, ok := sm.schemas[gvr.String()]
	return schema, ok
}

// GetMergedSchema returns the complete merged OpenAPI document containing all registered schemas.
// This document can be used to serve OpenAPI specifications for the virtual workspace.
func (sm *SchemaManager) GetMergedSchema() *spec.Swagger {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.merged
}

// RemoveSchema removes the schema for a specific GroupVersionResource.
// This also updates the merged OpenAPI document to reflect the removal.
func (sm *SchemaManager) RemoveSchema(gvr schema.GroupVersionResource) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.schemas, gvr.String())
	sm.updateMergedSchema()
}

// Clear removes all registered schemas and resets the merged OpenAPI document.
// This is useful for cleanup or when starting fresh with a new set of schemas.
func (sm *SchemaManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.schemas = make(map[string]*spec.Schema)
	sm.updateMergedSchema()
}

// updateMergedSchema rebuilds the merged OpenAPI document from all registered schemas.
// This method assumes the write lock is already held by the caller.
func (sm *SchemaManager) updateMergedSchema() {
	// Ensure the definitions map exists
	if sm.merged.Definitions == nil {
		sm.merged.Definitions = spec.Definitions{}
	}

	// Clear existing definitions and rebuild from current schemas
	sm.merged.Definitions = spec.Definitions{}
	for key, schema := range sm.schemas {
		sm.merged.Definitions[key] = *schema
	}
}

// ValidateAgainstSchema validates an object against its registered OpenAPI schema.
// Returns an error if no schema is found or if validation fails.
// Note: This is a basic implementation; a full implementation would perform detailed validation.
func (sm *SchemaManager) ValidateAgainstSchema(gvr schema.GroupVersionResource, obj interface{}) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	schema, ok := sm.schemas[gvr.String()]
	if !ok {
		return fmt.Errorf("no schema found for %s", gvr.String())
	}

	// Basic validation: ensure schema exists and is not nil
	if schema == nil {
		return fmt.Errorf("schema is nil for %s", gvr.String())
	}

	// In a full implementation, this would perform comprehensive OpenAPI validation
	// against the schema using libraries like go-openapi/validate
	return nil
}

// GetSchemaCount returns the number of schemas currently registered.
func (sm *SchemaManager) GetSchemaCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return len(sm.schemas)
}

// HasSchema checks if a schema is registered for the given GroupVersionResource.
func (sm *SchemaManager) HasSchema(gvr schema.GroupVersionResource) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	_, ok := sm.schemas[gvr.String()]
	return ok
}