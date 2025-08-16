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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
)

// ResourceConverter provides conversion capabilities for discovered resources
// between different API versions and formats. This supports backward compatibility
// and schema transformation in virtual workspaces.
type ResourceConverter struct {
	// conversionRules maps from GVR to conversion configuration
	conversionRules map[schema.GroupVersionResource]*ConversionRule
}

// ConversionRule defines how to convert a resource between versions
type ConversionRule struct {
	// SourceGVR is the original GroupVersionResource
	SourceGVR schema.GroupVersionResource
	
	// TargetGVR is the target GroupVersionResource for conversion
	TargetGVR schema.GroupVersionResource
	
	// FieldMappings defines how to map fields between versions
	FieldMappings map[string]string
	
	// RequiredFields lists fields that must be present after conversion
	RequiredFields []string
	
	// RemovedFields lists fields that should be removed during conversion
	RemovedFields []string
}

// NewResourceConverter creates a new resource converter with default rules
func NewResourceConverter() *ResourceConverter {
	converter := &ResourceConverter{
		conversionRules: make(map[schema.GroupVersionResource]*ConversionRule),
	}
	
	// Add default conversion rules for common Kubernetes resources
	converter.addDefaultConversionRules()
	
	return converter
}

// ConvertResources converts a slice of ResourceInfo to a target API version
func (c *ResourceConverter) ConvertResources(resources []interfaces.ResourceInfo, targetVersion string) ([]interfaces.ResourceInfo, error) {
	if len(resources) == 0 {
		return resources, nil
	}
	
	converted := make([]interfaces.ResourceInfo, 0, len(resources))
	
	for _, resource := range resources {
		convertedResource, err := c.ConvertResource(resource, targetVersion)
		if err != nil {
			// Log error but continue with other resources
			continue
		}
		converted = append(converted, convertedResource)
	}
	
	return converted, nil
}

// ConvertResource converts a single ResourceInfo to a target API version
func (c *ResourceConverter) ConvertResource(resource interfaces.ResourceInfo, targetVersion string) (interfaces.ResourceInfo, error) {
	gvr := resource.GroupVersionResource
	
	// Check if we have a conversion rule for this resource
	rule, exists := c.conversionRules[gvr]
	if !exists {
		// No conversion rule, return as-is
		return resource, nil
	}
	
	// Apply conversion rule
	converted := resource
	converted.GroupVersionResource = rule.TargetGVR
	
	// Convert APIResource metadata
	convertedAPIResource, err := c.convertAPIResource(resource.APIResource, rule)
	if err != nil {
		return resource, fmt.Errorf("failed to convert APIResource: %w", err)
	}
	converted.APIResource = convertedAPIResource
	
	// Convert OpenAPI schema if present
	if len(resource.OpenAPISchema) > 0 {
		convertedSchema, err := c.convertOpenAPISchema(resource.OpenAPISchema, rule)
		if err != nil {
			return resource, fmt.Errorf("failed to convert OpenAPI schema: %w", err)
		}
		converted.OpenAPISchema = convertedSchema
	}
	
	return converted, nil
}

// AddConversionRule adds a custom conversion rule
func (c *ResourceConverter) AddConversionRule(rule *ConversionRule) {
	c.conversionRules[rule.SourceGVR] = rule
}

// RemoveConversionRule removes a conversion rule
func (c *ResourceConverter) RemoveConversionRule(gvr schema.GroupVersionResource) {
	delete(c.conversionRules, gvr)
}

// GetConversionRules returns all configured conversion rules
func (c *ResourceConverter) GetConversionRules() map[schema.GroupVersionResource]*ConversionRule {
	// Return a copy to prevent external mutation
	rules := make(map[schema.GroupVersionResource]*ConversionRule)
	for k, v := range c.conversionRules {
		rules[k] = v
	}
	return rules
}

// convertAPIResource applies conversion rules to APIResource metadata
func (c *ResourceConverter) convertAPIResource(apiResource metav1.APIResource, rule *ConversionRule) (metav1.APIResource, error) {
	converted := apiResource
	
	// Update version information
	converted.Version = rule.TargetGVR.Version
	if converted.Group == "" {
		converted.Group = rule.TargetGVR.Group
	}
	
	// Apply field mappings to verbs and other string slice fields
	converted.Verbs = c.applyFieldMappingsToSlice(apiResource.Verbs, rule.FieldMappings)
	converted.ShortNames = c.applyFieldMappingsToSlice(apiResource.ShortNames, rule.FieldMappings)
	converted.Categories = c.applyFieldMappingsToSlice(apiResource.Categories, rule.FieldMappings)
	
	return converted, nil
}

// convertOpenAPISchema applies conversion rules to OpenAPI schema
func (c *ResourceConverter) convertOpenAPISchema(schema []byte, rule *ConversionRule) ([]byte, error) {
	if len(schema) == 0 {
		return schema, nil
	}
	
	// Parse the schema as JSON
	var schemaObj map[string]interface{}
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		return schema, fmt.Errorf("failed to parse OpenAPI schema: %w", err)
	}
	
	// Apply field mappings to the schema
	convertedSchema := c.applyFieldMappingsToSchema(schemaObj, rule.FieldMappings)
	
	// Remove fields that should be removed
	for _, field := range rule.RemovedFields {
		c.removeFieldFromSchema(convertedSchema, field)
	}
	
	// Serialize back to JSON
	convertedBytes, err := json.Marshal(convertedSchema)
	if err != nil {
		return schema, fmt.Errorf("failed to serialize converted schema: %w", err)
	}
	
	return convertedBytes, nil
}

// applyFieldMappingsToSlice applies field mappings to a string slice
func (c *ResourceConverter) applyFieldMappingsToSlice(slice []string, mappings map[string]string) []string {
	if len(mappings) == 0 {
		return slice
	}
	
	result := make([]string, len(slice))
	for i, item := range slice {
		if mapped, exists := mappings[item]; exists {
			result[i] = mapped
		} else {
			result[i] = item
		}
	}
	
	return result
}

// applyFieldMappingsToSchema applies field mappings to a schema object
func (c *ResourceConverter) applyFieldMappingsToSchema(schema map[string]interface{}, mappings map[string]string) map[string]interface{} {
	if len(mappings) == 0 {
		return schema
	}
	
	// This is a simplified implementation. In practice, you would need
	// more sophisticated schema transformation logic.
	converted := make(map[string]interface{})
	for key, value := range schema {
		if mapped, exists := mappings[key]; exists {
			converted[mapped] = value
		} else {
			converted[key] = value
		}
	}
	
	return converted
}

// removeFieldFromSchema removes a field from a schema object
func (c *ResourceConverter) removeFieldFromSchema(schema map[string]interface{}, fieldPath string) {
	// Simple implementation - just remove top-level fields
	// In practice, you'd need to handle nested field paths
	delete(schema, fieldPath)
}

// addDefaultConversionRules adds common conversion rules for Kubernetes resources
func (c *ResourceConverter) addDefaultConversionRules() {
	// Example: Convert apps/v1beta1 Deployments to apps/v1
	c.AddConversionRule(&ConversionRule{
		SourceGVR: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1beta1",
			Resource: "deployments",
		},
		TargetGVR: schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		FieldMappings: map[string]string{
			"spec.replicas": "spec.replicas",
			"spec.template": "spec.template",
		},
		RequiredFields: []string{"spec", "metadata"},
		RemovedFields:  []string{"spec.rollbackTo"},
	})
	
	// Example: Convert extensions/v1beta1 Ingresses to networking.k8s.io/v1
	c.AddConversionRule(&ConversionRule{
		SourceGVR: schema.GroupVersionResource{
			Group:    "extensions",
			Version:  "v1beta1",
			Resource: "ingresses",
		},
		TargetGVR: schema.GroupVersionResource{
			Group:    "networking.k8s.io",
			Version:  "v1",
			Resource: "ingresses",
		},
		FieldMappings: map[string]string{
			"spec.rules": "spec.rules",
			"spec.tls":   "spec.tls",
		},
		RequiredFields: []string{"spec"},
		RemovedFields:  []string{},
	})
}