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
	"k8s.io/klog/v2"

	apisv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"

	"github.com/kcp-dev/kcp/pkg/virtual/interfaces"
)

// APIExportConverter converts APIExport data to ResourceInfo
type APIExportConverter struct {
	// workspace identifies the logical cluster for conversions
	workspace string
}

// NewAPIExportConverter creates a new APIExport converter
func NewAPIExportConverter(workspace string) *APIExportConverter {
	return &APIExportConverter{
		workspace: workspace,
	}
}

// ConvertAPIExport converts an APIExport to ResourceInfo array
func (c *APIExportConverter) ConvertAPIExport(apiExport *apisv1alpha1.APIExport) ([]interfaces.ResourceInfo, error) {
	if apiExport == nil {
		return nil, fmt.Errorf("apiExport cannot be nil")
	}

	var resources []interfaces.ResourceInfo

	// Convert each schema in the APIExport
	for _, schemaName := range apiExport.Spec.LatestResourceSchemas {
		// In a real implementation, you would resolve the APIResourceSchema
		// For now, we'll create a basic resource info from the schema name
		resourceInfo, err := c.convertSchemaReference(schemaName, apiExport)
		if err != nil {
			klog.ErrorS(err, "Failed to convert schema reference", "schema", schemaName, "apiExport", apiExport.Name)
			continue
		}

		resources = append(resources, resourceInfo)
	}

	klog.V(5).InfoS("Converted APIExport to resources", "apiExport", apiExport.Name, "resourceCount", len(resources))
	return resources, nil
}

// convertSchemaReference converts a schema reference to ResourceInfo
func (c *APIExportConverter) convertSchemaReference(schemaName string, apiExport *apisv1alpha1.APIExport) (interfaces.ResourceInfo, error) {
	// This is a simplified conversion - in practice, you would fetch the actual APIResourceSchema
	// and extract the full resource definition
	
	// Parse the schema name to extract group/version/kind information
	// Schema names typically follow a pattern like "v<version>.<kind>.<group>"
	gvr, err := c.parseSchemaNameToGVR(schemaName)
	if err != nil {
		return interfaces.ResourceInfo{}, fmt.Errorf("failed to parse schema name %q: %w", schemaName, err)
	}

	// Create basic APIResource
	apiResource := metav1.APIResource{
		Name:         gvr.Resource,
		SingularName: gvr.Resource,
		Namespaced:   !c.isClusterScoped(schemaName),
		Group:        gvr.Group,
		Version:      gvr.Version,
		Kind:         c.extractKindFromSchema(schemaName),
		Verbs:        []string{"get", "list", "create", "update", "patch", "watch", "delete"}, // Default verbs
	}

	// Determine workspace for this resource
	workspace := c.workspace
	// In a real implementation, you would extract workspace from annotations or context
	// For now, use the converter's workspace

	// Extract OpenAPI schema if available
	openAPISchema, err := c.extractOpenAPISchemaFromAPIExport(apiExport, schemaName)
	if err != nil {
		klog.V(4).InfoS("Failed to extract OpenAPI schema", "schema", schemaName, "error", err)
		openAPISchema = nil
	}

	resourceInfo := interfaces.ResourceInfo{
		GroupVersionResource: gvr,
		APIResource:         apiResource,
		Workspace:           workspace,
		APIExportName:       apiExport.Name,
		OpenAPISchema:       openAPISchema,
		IsWorkspaceScoped:   c.isWorkspaceScoped(schemaName),
	}

	return resourceInfo, nil
}

// parseSchemaNameToGVR parses a schema name to extract GroupVersionResource
func (c *APIExportConverter) parseSchemaNameToGVR(schemaName string) (schema.GroupVersionResource, error) {
	// This is a simplified parser - actual implementation would be more robust
	// Expected format: "v<version>.<resource>.<group>" or just "<resource>"
	
	// For now, create a basic GVR
	return schema.GroupVersionResource{
		Group:    c.extractGroupFromSchema(schemaName),
		Version:  c.extractVersionFromSchema(schemaName),
		Resource: c.extractResourceFromSchema(schemaName),
	}, nil
}

// extractGroupFromSchema extracts group from schema name
func (c *APIExportConverter) extractGroupFromSchema(schemaName string) string {
	// Simplified - extract group from schema name pattern
	// In practice, this would parse the actual schema structure
	if len(schemaName) > 0 {
		return "example.com" // Default group
	}
	return ""
}

// extractVersionFromSchema extracts version from schema name
func (c *APIExportConverter) extractVersionFromSchema(schemaName string) string {
	// Simplified - extract version from schema name pattern
	return "v1" // Default version
}

// extractResourceFromSchema extracts resource name from schema name
func (c *APIExportConverter) extractResourceFromSchema(schemaName string) string {
	// Simplified - use schema name as resource name
	if schemaName == "" {
		return "resources"
	}
	return schemaName
}

// extractKindFromSchema extracts kind from schema name
func (c *APIExportConverter) extractKindFromSchema(schemaName string) string {
	// Simplified - capitalize first letter for kind
	if len(schemaName) == 0 {
		return "Resource"
	}
	// Convert resource name to Kind (e.g., "pods" -> "Pod")
	return schemaName[:1] + schemaName[1:]
}

// isClusterScoped determines if a resource is cluster-scoped
func (c *APIExportConverter) isClusterScoped(schemaName string) bool {
	// Simplified logic - in practice, this would check the actual schema
	clusterScopedResources := []string{"nodes", "persistentvolumes", "namespaces", "clusterroles", "clusterrolebindings"}
	for _, resource := range clusterScopedResources {
		if schemaName == resource {
			return true
		}
	}
	return false
}

// isWorkspaceScoped determines if a resource is workspace-scoped
func (c *APIExportConverter) isWorkspaceScoped(schemaName string) bool {
	// Simplified logic - assume most resources are workspace-scoped in KCP
	return !c.isClusterScoped(schemaName)
}

// extractOpenAPISchemaFromAPIExport extracts OpenAPI schema from APIExport
func (c *APIExportConverter) extractOpenAPISchemaFromAPIExport(apiExport *apisv1alpha1.APIExport, schemaName string) ([]byte, error) {
	// This is a placeholder implementation
	// In practice, you would extract the actual OpenAPI schema from the APIExport or related schemas
	
	// Create a minimal OpenAPI schema representation
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"apiVersion": map[string]interface{}{
				"type": "string",
			},
			"kind": map[string]interface{}{
				"type": "string",
			},
			"metadata": map[string]interface{}{
				"type": "object",
			},
			"spec": map[string]interface{}{
				"type": "object",
			},
			"status": map[string]interface{}{
				"type": "object",
			},
		},
	}

	return json.Marshal(schema)
}