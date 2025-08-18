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

package apidocs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// OpenAPIGenerator generates OpenAPI 3.0 specifications for KCP APIs.
type OpenAPIGenerator struct {
	apiTypes  map[schema.GroupVersionKind]*APIType
	outputDir string
}

// OpenAPISpec represents an OpenAPI 3.0 specification.
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi" yaml:"openapi"`
	Info       OpenAPIInfo            `json:"info" yaml:"info"`
	Servers    []OpenAPIServer        `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths      map[string]OpenAPIPath `json:"paths" yaml:"paths"`
	Components OpenAPIComponents      `json:"components" yaml:"components"`
}

// OpenAPIInfo contains API metadata.
type OpenAPIInfo struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description" yaml:"description"`
	Version     string `json:"version" yaml:"version"`
}

// OpenAPIServer describes an API server.
type OpenAPIServer struct {
	URL         string `json:"url" yaml:"url"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// OpenAPIPath describes operations available on a path.
type OpenAPIPath struct {
	Get    *OpenAPIOperation `json:"get,omitempty" yaml:"get,omitempty"`
	Post   *OpenAPIOperation `json:"post,omitempty" yaml:"post,omitempty"`
	Put    *OpenAPIOperation `json:"put,omitempty" yaml:"put,omitempty"`
	Patch  *OpenAPIOperation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Delete *OpenAPIOperation `json:"delete,omitempty" yaml:"delete,omitempty"`
}

// OpenAPIOperation describes a single API operation.
type OpenAPIOperation struct {
	Summary     string                            `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string                            `json:"description,omitempty" yaml:"description,omitempty"`
	OperationID string                            `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Tags        []string                          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Parameters  []OpenAPIParameter                `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody               `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse        `json:"responses" yaml:"responses"`
}

// OpenAPIParameter describes an operation parameter.
type OpenAPIParameter struct {
	Name        string          `json:"name" yaml:"name"`
	In          string          `json:"in" yaml:"in"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool            `json:"required,omitempty" yaml:"required,omitempty"`
	Schema      *OpenAPISchema  `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// OpenAPIRequestBody describes a request body.
type OpenAPIRequestBody struct {
	Description string                       `json:"description,omitempty" yaml:"description,omitempty"`
	Content     map[string]OpenAPIMediaType  `json:"content" yaml:"content"`
	Required    bool                         `json:"required,omitempty" yaml:"required,omitempty"`
}

// OpenAPIResponse describes a response.
type OpenAPIResponse struct {
	Description string                       `json:"description" yaml:"description"`
	Content     map[string]OpenAPIMediaType  `json:"content,omitempty" yaml:"content,omitempty"`
}

// OpenAPIMediaType describes a media type.
type OpenAPIMediaType struct {
	Schema *OpenAPISchema `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// OpenAPIComponents contains reusable components.
type OpenAPIComponents struct {
	Schemas map[string]OpenAPISchema `json:"schemas,omitempty" yaml:"schemas,omitempty"`
}

// OpenAPISchema describes a data schema.
type OpenAPISchema struct {
	Type                 string                    `json:"type,omitempty" yaml:"type,omitempty"`
	Format               string                    `json:"format,omitempty" yaml:"format,omitempty"`
	Description          string                    `json:"description,omitempty" yaml:"description,omitempty"`
	Properties           map[string]OpenAPISchema  `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required             []string                  `json:"required,omitempty" yaml:"required,omitempty"`
	Items                *OpenAPISchema            `json:"items,omitempty" yaml:"items,omitempty"`
	AdditionalProperties interface{}               `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
	Ref                  string                    `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Enum                 []interface{}             `json:"enum,omitempty" yaml:"enum,omitempty"`
}

// NewOpenAPIGenerator creates a new OpenAPI generator.
func NewOpenAPIGenerator(apiTypes map[schema.GroupVersionKind]*APIType, outputDir string) *OpenAPIGenerator {
	return &OpenAPIGenerator{
		apiTypes:  apiTypes,
		outputDir: outputDir,
	}
}

// Generate creates OpenAPI specifications for all discovered API types.
func (g *OpenAPIGenerator) Generate() error {
	// Create output directory
	openAPIDir := filepath.Join(g.outputDir, "openapi")
	if err := os.MkdirAll(openAPIDir, 0755); err != nil {
		return fmt.Errorf("failed to create openapi directory: %w", err)
	}

	// Group APIs by group and version
	groupedAPIs := g.groupAPIsByGroupVersion()

	// Generate OpenAPI spec for each group/version
	for groupVersion, apiTypes := range groupedAPIs {
		spec := g.generateOpenAPISpec(groupVersion, apiTypes)
		
		// Write JSON version
		jsonFile := filepath.Join(openAPIDir, fmt.Sprintf("%s.json", strings.ReplaceAll(groupVersion, "/", "_")))
		if err := g.writeJSONFile(jsonFile, spec); err != nil {
			return fmt.Errorf("failed to write JSON spec for %s: %w", groupVersion, err)
		}

		// Write YAML version
		yamlFile := filepath.Join(openAPIDir, fmt.Sprintf("%s.yaml", strings.ReplaceAll(groupVersion, "/", "_")))
		if err := g.writeYAMLFile(yamlFile, spec); err != nil {
			return fmt.Errorf("failed to write YAML spec for %s: %w", groupVersion, err)
		}
	}

	// Generate combined specification
	combinedSpec := g.generateCombinedSpec()
	
	combinedJSONFile := filepath.Join(openAPIDir, "kcp-api-combined.json")
	if err := g.writeJSONFile(combinedJSONFile, combinedSpec); err != nil {
		return fmt.Errorf("failed to write combined JSON spec: %w", err)
	}

	combinedYAMLFile := filepath.Join(openAPIDir, "kcp-api-combined.yaml")
	if err := g.writeYAMLFile(combinedYAMLFile, combinedSpec); err != nil {
		return fmt.Errorf("failed to write combined YAML spec: %w", err)
	}

	return nil
}

// groupAPIsByGroupVersion groups API types by group and version.
func (g *OpenAPIGenerator) groupAPIsByGroupVersion() map[string][]*APIType {
	grouped := make(map[string][]*APIType)

	for _, apiType := range g.apiTypes {
		groupVersion := fmt.Sprintf("%s/%s", apiType.Group, apiType.Version)
		grouped[groupVersion] = append(grouped[groupVersion], apiType)
	}

	// Sort API types within each group
	for groupVersion := range grouped {
		sort.Slice(grouped[groupVersion], func(i, j int) bool {
			return grouped[groupVersion][i].Kind < grouped[groupVersion][j].Kind
		})
	}

	return grouped
}

// generateOpenAPISpec generates an OpenAPI specification for a group/version.
func (g *OpenAPIGenerator) generateOpenAPISpec(groupVersion string, apiTypes []*APIType) *OpenAPISpec {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: OpenAPIInfo{
			Title:       fmt.Sprintf("KCP %s API", groupVersion),
			Description: fmt.Sprintf("OpenAPI specification for KCP %s API resources", groupVersion),
			Version:     "v1.0.0",
		},
		Servers: []OpenAPIServer{
			{
				URL:         "https://localhost:6443",
				Description: "Local KCP server",
			},
		},
		Paths:      make(map[string]OpenAPIPath),
		Components: OpenAPIComponents{
			Schemas: make(map[string]OpenAPISchema),
		},
	}

	// Generate schemas and paths for each API type
	for _, apiType := range apiTypes {
		// Generate schema
		schema := g.generateSchema(apiType)
		spec.Components.Schemas[apiType.Kind] = schema

		// Generate paths for this resource type
		g.generatePathsForResource(spec, apiType)
	}

	return spec
}

// generateCombinedSpec generates a combined OpenAPI specification for all APIs.
func (g *OpenAPIGenerator) generateCombinedSpec() *OpenAPISpec {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: OpenAPIInfo{
			Title:       "KCP API",
			Description: "Combined OpenAPI specification for all KCP API resources",
			Version:     "v1.0.0",
		},
		Servers: []OpenAPIServer{
			{
				URL:         "https://localhost:6443",
				Description: "Local KCP server",
			},
		},
		Paths:      make(map[string]OpenAPIPath),
		Components: OpenAPIComponents{
			Schemas: make(map[string]OpenAPISchema),
		},
	}

	// Add all API types to the combined spec
	for _, apiType := range g.apiTypes {
		// Generate schema with group/version prefix to avoid conflicts
		schemaName := fmt.Sprintf("%s_%s_%s", apiType.Group, apiType.Version, apiType.Kind)
		schemaName = strings.ReplaceAll(schemaName, ".", "_")
		schemaName = strings.ReplaceAll(schemaName, "-", "_")
		
		schema := g.generateSchema(apiType)
		spec.Components.Schemas[schemaName] = schema

		// Generate paths for this resource type
		g.generatePathsForResource(spec, apiType)
	}

	return spec
}

// generateSchema generates an OpenAPI schema for an API type.
func (g *OpenAPIGenerator) generateSchema(apiType *APIType) OpenAPISchema {
	schema := OpenAPISchema{
		Type:        "object",
		Description: apiType.Description,
		Properties:  make(map[string]OpenAPISchema),
		Required:    []string{},
	}

	// Add standard Kubernetes fields
	schema.Properties["apiVersion"] = OpenAPISchema{
		Type:        "string",
		Description: "APIVersion defines the versioned schema of this representation of an object",
	}
	schema.Properties["kind"] = OpenAPISchema{
		Type:        "string",
		Description: "Kind is a string value representing the REST resource this object represents",
	}
	schema.Properties["metadata"] = OpenAPISchema{
		Ref: "#/components/schemas/ObjectMeta",
	}

	schema.Required = append(schema.Required, "apiVersion", "kind")

	// Add custom fields from the API type
	for _, field := range apiType.Fields {
		fieldSchema := g.generateFieldSchema(field)
		schema.Properties[field.JSONName] = fieldSchema

		if field.Required {
			schema.Required = append(schema.Required, field.JSONName)
		}
	}

	return schema
}

// generateFieldSchema generates an OpenAPI schema for a field.
func (g *OpenAPIGenerator) generateFieldSchema(field *FieldDoc) OpenAPISchema {
	schema := OpenAPISchema{
		Description: field.Description,
	}

	// Map Go types to OpenAPI types
	switch {
	case strings.HasPrefix(field.Type, "[]"):
		schema.Type = "array"
		itemType := strings.TrimPrefix(field.Type, "[]")
		schema.Items = &OpenAPISchema{}
		*schema.Items = g.generateTypeSchema(itemType)

	case strings.HasPrefix(field.Type, "map["):
		schema.Type = "object"
		schema.AdditionalProperties = true

	default:
		schema = g.generateTypeSchema(field.Type)
		schema.Description = field.Description
	}

	return schema
}

// generateTypeSchema generates schema for a specific type.
func (g *OpenAPIGenerator) generateTypeSchema(typeName string) OpenAPISchema {
	switch typeName {
	case "string":
		return OpenAPISchema{Type: "string"}
	case "int", "int32", "int64":
		return OpenAPISchema{Type: "integer", Format: "int64"}
	case "float32", "float64":
		return OpenAPISchema{Type: "number", Format: "float"}
	case "bool":
		return OpenAPISchema{Type: "boolean"}
	case "time.Time", "*time.Time":
		return OpenAPISchema{Type: "string", Format: "date-time"}
	case "metav1.Time", "*metav1.Time":
		return OpenAPISchema{Type: "string", Format: "date-time"}
	case "metav1.Duration", "*metav1.Duration":
		return OpenAPISchema{Type: "string"}
	case "interface{}":
		return OpenAPISchema{Type: "object", AdditionalProperties: true}
	default:
		// For custom types, reference the schema
		if strings.Contains(typeName, ".") {
			return OpenAPISchema{Ref: fmt.Sprintf("#/components/schemas/%s", typeName)}
		}
		return OpenAPISchema{Type: "object"}
	}
}

// generatePathsForResource generates REST API paths for a resource type.
func (g *OpenAPIGenerator) generatePathsForResource(spec *OpenAPISpec, apiType *APIType) {
	resourceName := strings.ToLower(apiType.Kind) + "s" // Pluralize
	group := apiType.Group
	version := apiType.Version

	// Collection path: /apis/{group}/{version}/{resource}
	collectionPath := fmt.Sprintf("/apis/%s/%s/%s", group, version, resourceName)
	
	// Resource path: /apis/{group}/{version}/{resource}/{name}
	resourcePath := fmt.Sprintf("/apis/%s/%s/%s/{name}", group, version, resourceName)

	// Generate collection operations
	spec.Paths[collectionPath] = OpenAPIPath{
		Get: &OpenAPIOperation{
			Summary:     fmt.Sprintf("List %s", apiType.Kind),
			Description: fmt.Sprintf("List all %s resources", apiType.Kind),
			Tags:        []string{apiType.Group},
			Responses: map[string]OpenAPIResponse{
				"200": {
					Description: "Success",
					Content: map[string]OpenAPIMediaType{
						"application/json": {
							Schema: &OpenAPISchema{
								Ref: fmt.Sprintf("#/components/schemas/%sList", apiType.Kind),
							},
						},
					},
				},
			},
		},
		Post: &OpenAPIOperation{
			Summary:     fmt.Sprintf("Create %s", apiType.Kind),
			Description: fmt.Sprintf("Create a new %s resource", apiType.Kind),
			Tags:        []string{apiType.Group},
			RequestBody: &OpenAPIRequestBody{
				Required: true,
				Content: map[string]OpenAPIMediaType{
					"application/json": {
						Schema: &OpenAPISchema{
							Ref: fmt.Sprintf("#/components/schemas/%s", apiType.Kind),
						},
					},
				},
			},
			Responses: map[string]OpenAPIResponse{
				"201": {
					Description: "Created",
					Content: map[string]OpenAPIMediaType{
						"application/json": {
							Schema: &OpenAPISchema{
								Ref: fmt.Sprintf("#/components/schemas/%s", apiType.Kind),
							},
						},
					},
				},
			},
		},
	}

	// Generate resource operations
	spec.Paths[resourcePath] = OpenAPIPath{
		Get: &OpenAPIOperation{
			Summary:     fmt.Sprintf("Get %s", apiType.Kind),
			Description: fmt.Sprintf("Get a specific %s resource", apiType.Kind),
			Tags:        []string{apiType.Group},
			Parameters: []OpenAPIParameter{
				{
					Name:        "name",
					In:          "path",
					Description: fmt.Sprintf("Name of the %s resource", apiType.Kind),
					Required:    true,
					Schema:      &OpenAPISchema{Type: "string"},
				},
			},
			Responses: map[string]OpenAPIResponse{
				"200": {
					Description: "Success",
					Content: map[string]OpenAPIMediaType{
						"application/json": {
							Schema: &OpenAPISchema{
								Ref: fmt.Sprintf("#/components/schemas/%s", apiType.Kind),
							},
						},
					},
				},
			},
		},
		Put: &OpenAPIOperation{
			Summary:     fmt.Sprintf("Update %s", apiType.Kind),
			Description: fmt.Sprintf("Update a %s resource", apiType.Kind),
			Tags:        []string{apiType.Group},
			Parameters: []OpenAPIParameter{
				{
					Name:        "name",
					In:          "path",
					Description: fmt.Sprintf("Name of the %s resource", apiType.Kind),
					Required:    true,
					Schema:      &OpenAPISchema{Type: "string"},
				},
			},
			RequestBody: &OpenAPIRequestBody{
				Required: true,
				Content: map[string]OpenAPIMediaType{
					"application/json": {
						Schema: &OpenAPISchema{
							Ref: fmt.Sprintf("#/components/schemas/%s", apiType.Kind),
						},
					},
				},
			},
			Responses: map[string]OpenAPIResponse{
				"200": {
					Description: "Updated",
					Content: map[string]OpenAPIMediaType{
						"application/json": {
							Schema: &OpenAPISchema{
								Ref: fmt.Sprintf("#/components/schemas/%s", apiType.Kind),
							},
						},
					},
				},
			},
		},
		Delete: &OpenAPIOperation{
			Summary:     fmt.Sprintf("Delete %s", apiType.Kind),
			Description: fmt.Sprintf("Delete a %s resource", apiType.Kind),
			Tags:        []string{apiType.Group},
			Parameters: []OpenAPIParameter{
				{
					Name:        "name",
					In:          "path",
					Description: fmt.Sprintf("Name of the %s resource", apiType.Kind),
					Required:    true,
					Schema:      &OpenAPISchema{Type: "string"},
				},
			},
			Responses: map[string]OpenAPIResponse{
				"200": {
					Description: "Deleted",
				},
			},
		},
	}
}

// writeJSONFile writes an OpenAPI spec to a JSON file.
func (g *OpenAPIGenerator) writeJSONFile(filename string, spec *OpenAPISpec) error {
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// writeYAMLFile writes an OpenAPI spec to a YAML file.
func (g *OpenAPIGenerator) writeYAMLFile(filename string, spec *OpenAPISpec) error {
	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}