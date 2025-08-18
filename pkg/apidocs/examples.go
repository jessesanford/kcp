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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// ExampleGenerator generates example YAML files for KCP API resources.
type ExampleGenerator struct {
	apiTypes  map[schema.GroupVersionKind]*APIType
	outputDir string
}

// ExampleResource represents a complete example resource.
type ExampleResource struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   Metadata    `yaml:"metadata"`
	Spec       interface{} `yaml:"spec,omitempty"`
	Status     interface{} `yaml:"status,omitempty"`
}

// Metadata represents Kubernetes object metadata.
type Metadata struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// NewExampleGenerator creates a new example generator.
func NewExampleGenerator(apiTypes map[schema.GroupVersionKind]*APIType, outputDir string) *ExampleGenerator {
	return &ExampleGenerator{
		apiTypes:  apiTypes,
		outputDir: outputDir,
	}
}

// Generate creates example YAML files for all discovered API types.
func (g *ExampleGenerator) Generate() error {
	// Create output directory
	examplesDir := filepath.Join(g.outputDir, "examples")
	if err := os.MkdirAll(examplesDir, 0755); err != nil {
		return fmt.Errorf("failed to create examples directory: %w", err)
	}

	// Generate examples for each API type
	for _, apiType := range g.apiTypes {
		examples := g.generateExamplesForType(apiType)
		
		for i, example := range examples {
			fileName := g.getExampleFileName(apiType, i)
			filePath := filepath.Join(examplesDir, fileName)
			
			if err := g.writeExampleFile(filePath, example); err != nil {
				return fmt.Errorf("failed to write example for %s: %w", apiType.Kind, err)
			}
		}
	}

	// Generate index file
	if err := g.generateExampleIndex(examplesDir); err != nil {
		return fmt.Errorf("failed to generate example index: %w", err)
	}

	return nil
}

// generateExamplesForType generates multiple example variations for an API type.
func (g *ExampleGenerator) generateExamplesForType(apiType *APIType) []ExampleResource {
	var examples []ExampleResource

	// Generate basic example
	basic := g.generateBasicExample(apiType)
	examples = append(examples, basic)

	// Generate complete example with all fields
	complete := g.generateCompleteExample(apiType)
	examples = append(examples, complete)

	// Generate use-case specific examples based on the resource type
	useCaseExamples := g.generateUseCaseExamples(apiType)
	examples = append(examples, useCaseExamples...)

	return examples
}

// generateBasicExample generates a minimal working example.
func (g *ExampleGenerator) generateBasicExample(apiType *APIType) ExampleResource {
	example := ExampleResource{
		APIVersion: fmt.Sprintf("%s/%s", apiType.Group, apiType.Version),
		Kind:       apiType.Kind,
		Metadata: Metadata{
			Name: fmt.Sprintf("example-%s", strings.ToLower(apiType.Kind)),
		},
	}

	// Add basic spec if there are required fields
	spec := g.generateBasicSpec(apiType)
	if spec != nil {
		example.Spec = spec
	}

	return example
}

// generateCompleteExample generates an example with all available fields.
func (g *ExampleGenerator) generateCompleteExample(apiType *APIType) ExampleResource {
	example := ExampleResource{
		APIVersion: fmt.Sprintf("%s/%s", apiType.Group, apiType.Version),
		Kind:       apiType.Kind,
		Metadata: Metadata{
			Name:      fmt.Sprintf("complete-%s", strings.ToLower(apiType.Kind)),
			Namespace: "default",
			Labels: map[string]string{
				"app":     strings.ToLower(apiType.Kind),
				"version": "v1",
				"env":     "example",
			},
			Annotations: map[string]string{
				"example.kcp.io/description": fmt.Sprintf("Complete example of %s", apiType.Kind),
				"example.kcp.io/generated":   "true",
			},
		},
	}

	// Add complete spec
	spec := g.generateCompleteSpec(apiType)
	if spec != nil {
		example.Spec = spec
	}

	return example
}

// generateUseCaseExamples generates specific use-case examples based on the resource type.
func (g *ExampleGenerator) generateUseCaseExamples(apiType *APIType) []ExampleResource {
	var examples []ExampleResource

	switch apiType.Kind {
	case "APIExport":
		examples = append(examples, g.generateAPIExportExamples(apiType)...)
	case "APIBinding":
		examples = append(examples, g.generateAPIBindingExamples(apiType)...)
	case "Workspace":
		examples = append(examples, g.generateWorkspaceExamples(apiType)...)
	case "WorkspaceType":
		examples = append(examples, g.generateWorkspaceTypeExamples(apiType)...)
	case "LogicalCluster":
		examples = append(examples, g.generateLogicalClusterExamples(apiType)...)
	}

	return examples
}

// generateAPIExportExamples generates specific examples for APIExport resources.
func (g *ExampleGenerator) generateAPIExportExamples(apiType *APIType) []ExampleResource {
	examples := []ExampleResource{
		{
			APIVersion: fmt.Sprintf("%s/%s", apiType.Group, apiType.Version),
			Kind:       apiType.Kind,
			Metadata: Metadata{
				Name: "my-api-export",
				Labels: map[string]string{
					"api.kcp.io/export-type": "custom",
				},
			},
			Spec: map[string]interface{}{
				"latestResourceSchemas": []string{
					"myresource.example.com",
				},
				"permissionClaims": []map[string]interface{}{
					{
						"group":    "",
						"resource": "configmaps",
						"verb":     "get",
					},
				},
			},
		},
	}

	return examples
}

// generateAPIBindingExamples generates specific examples for APIBinding resources.
func (g *ExampleGenerator) generateAPIBindingExamples(apiType *APIType) []ExampleResource {
	examples := []ExampleResource{
		{
			APIVersion: fmt.Sprintf("%s/%s", apiType.Group, apiType.Version),
			Kind:       apiType.Kind,
			Metadata: Metadata{
				Name: "kubernetes-binding",
			},
			Spec: map[string]interface{}{
				"reference": map[string]interface{}{
					"export": map[string]interface{}{
						"path": "root:compute",
						"name": "kubernetes",
					},
				},
			},
		},
	}

	return examples
}

// generateWorkspaceExamples generates specific examples for Workspace resources.
func (g *ExampleGenerator) generateWorkspaceExamples(apiType *APIType) []ExampleResource {
	examples := []ExampleResource{
		{
			APIVersion: fmt.Sprintf("%s/%s", apiType.Group, apiType.Version),
			Kind:       apiType.Kind,
			Metadata: Metadata{
				Name: "my-team-workspace",
			},
			Spec: map[string]interface{}{
				"type": map[string]interface{}{
					"name": "universal",
					"path": "root",
				},
			},
		},
	}

	return examples
}

// generateWorkspaceTypeExamples generates specific examples for WorkspaceType resources.
func (g *ExampleGenerator) generateWorkspaceTypeExamples(apiType *APIType) []ExampleResource {
	examples := []ExampleResource{
		{
			APIVersion: fmt.Sprintf("%s/%s", apiType.Group, apiType.Version),
			Kind:       apiType.Kind,
			Metadata: Metadata{
				Name: "custom-workspace-type",
			},
			Spec: map[string]interface{}{
				"defaultAPIBindings": []map[string]interface{}{
					{
						"export": map[string]interface{}{
							"path": "root:compute",
							"name": "kubernetes",
						},
					},
				},
			},
		},
	}

	return examples
}

// generateLogicalClusterExamples generates specific examples for LogicalCluster resources.
func (g *ExampleGenerator) generateLogicalClusterExamples(apiType *APIType) []ExampleResource {
	examples := []ExampleResource{
		{
			APIVersion: fmt.Sprintf("%s/%s", apiType.Group, apiType.Version),
			Kind:       apiType.Kind,
			Metadata: Metadata{
				Name: "logical-cluster-example",
				Annotations: map[string]string{
					"core.kcp.io/logical-cluster-name": "example-cluster",
				},
			},
		},
	}

	return examples
}

// generateBasicSpec generates a minimal spec for basic examples.
func (g *ExampleGenerator) generateBasicSpec(apiType *APIType) interface{} {
	if len(apiType.Fields) == 0 {
		return nil
	}

	spec := make(map[string]interface{})
	hasSpec := false

	// Look for required fields or common fields to populate
	for _, field := range apiType.Fields {
		if field.JSONName == "spec" {
			hasSpec = true
			// Generate nested spec content
			nestedSpec := g.generateFieldExample(field, true)
			if nestedSpec != nil {
				return nestedSpec
			}
		}
	}

	// If no spec field found, return nil
	if !hasSpec {
		return nil
	}

	// Generate a simple spec with basic fields
	spec["# TODO"] = "Add configuration here"
	return spec
}

// generateCompleteSpec generates a complete spec with all fields.
func (g *ExampleGenerator) generateCompleteSpec(apiType *APIType) interface{} {
	if len(apiType.Fields) == 0 {
		return nil
	}

	spec := make(map[string]interface{})
	hasSpec := false

	for _, field := range apiType.Fields {
		if field.JSONName == "spec" {
			hasSpec = true
			nestedSpec := g.generateFieldExample(field, false)
			if nestedSpec != nil {
				return nestedSpec
			}
		}
	}

	if !hasSpec {
		return nil
	}

	return spec
}

// generateFieldExample generates an example value for a field.
func (g *ExampleGenerator) generateFieldExample(field *FieldDoc, minimal bool) interface{} {
	switch {
	case strings.HasPrefix(field.Type, "[]"):
		// Array type
		itemType := strings.TrimPrefix(field.Type, "[]")
		exampleItem := g.generateTypeExample(itemType, minimal)
		return []interface{}{exampleItem}

	case strings.HasPrefix(field.Type, "map["):
		// Map type
		return map[string]interface{}{
			"example-key": "example-value",
		}

	default:
		return g.generateTypeExample(field.Type, minimal)
	}
}

// generateTypeExample generates an example value for a specific type.
func (g *ExampleGenerator) generateTypeExample(typeName string, minimal bool) interface{} {
	switch typeName {
	case "string":
		return "example-value"
	case "int", "int32", "int64":
		return 42
	case "float32", "float64":
		return 3.14
	case "bool":
		return true
	case "time.Time", "*time.Time", "metav1.Time":
		return "2023-01-01T00:00:00Z"
	case "metav1.Duration":
		return "30s"
	default:
		if minimal {
			return map[string]interface{}{}
		}
		return map[string]interface{}{
			"# TODO": fmt.Sprintf("Add %s configuration", typeName),
		}
	}
}

// getExampleFileName generates a filename for an example.
func (g *ExampleGenerator) getExampleFileName(apiType *APIType, index int) string {
	baseName := fmt.Sprintf("%s_%s_%s", 
		strings.ReplaceAll(apiType.Group, ".", "_"),
		apiType.Version,
		strings.ToLower(apiType.Kind))

	suffixes := []string{"basic", "complete", "usecase1", "usecase2"}
	suffix := "basic"
	if index < len(suffixes) {
		suffix = suffixes[index]
	}

	return fmt.Sprintf("%s_%s.yaml", baseName, suffix)
}

// writeExampleFile writes an example resource to a YAML file.
func (g *ExampleGenerator) writeExampleFile(filename string, example ExampleResource) error {
	data, err := yaml.Marshal(example)
	if err != nil {
		return fmt.Errorf("failed to marshal example to YAML: %w", err)
	}

	// Add header comment
	header := fmt.Sprintf("# Example %s resource\n# Generated by tmc-apidocs\n---\n", example.Kind)
	content := header + string(data)

	return os.WriteFile(filename, []byte(content), 0644)
}

// generateExampleIndex generates an index file listing all examples.
func (g *ExampleGenerator) generateExampleIndex(outputDir string) error {
	var content strings.Builder

	content.WriteString("# KCP API Examples\n\n")
	content.WriteString("This directory contains example YAML files for all KCP API resources.\n\n")
	content.WriteString("## Usage\n\n")
	content.WriteString("To use these examples:\n\n")
	content.WriteString("1. Copy the relevant example file\n")
	content.WriteString("2. Modify the values to suit your needs\n")
	content.WriteString("3. Apply using `kubectl apply -f <filename>`\n\n")

	// Group examples by API type
	typeGroups := make(map[string][]string)
	
	for _, apiType := range g.apiTypes {
		typeName := apiType.Kind
		examples := []string{
			g.getExampleFileName(apiType, 0),
			g.getExampleFileName(apiType, 1),
		}
		typeGroups[typeName] = examples
	}

	content.WriteString("## Available Examples\n\n")
	for typeName, examples := range typeGroups {
		content.WriteString(fmt.Sprintf("### %s\n\n", typeName))
		for _, example := range examples {
			content.WriteString(fmt.Sprintf("- [%s](%s)\n", example, example))
		}
		content.WriteString("\n")
	}

	return os.WriteFile(filepath.Join(outputDir, "README.md"), []byte(content.String()), 0644)
}