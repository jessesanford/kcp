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
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MarkdownGenerator generates Markdown documentation for KCP APIs.
type MarkdownGenerator struct {
	apiTypes  map[schema.GroupVersionKind]*APIType
	outputDir string
}

// NewMarkdownGenerator creates a new Markdown generator.
func NewMarkdownGenerator(apiTypes map[schema.GroupVersionKind]*APIType, outputDir string) *MarkdownGenerator {
	return &MarkdownGenerator{
		apiTypes:  apiTypes,
		outputDir: outputDir,
	}
}

// Generate creates Markdown documentation for all discovered API types.
func (g *MarkdownGenerator) Generate() error {
	// Create output directory
	markdownDir := filepath.Join(g.outputDir, "markdown")
	if err := os.MkdirAll(markdownDir, 0755); err != nil {
		return fmt.Errorf("failed to create markdown directory: %w", err)
	}

	// Generate index page
	if err := g.generateIndex(markdownDir); err != nil {
		return fmt.Errorf("failed to generate index: %w", err)
	}

	// Group APIs by group and version
	groupedAPIs := g.groupAPIsByGroupVersion()

	// Generate documentation for each group/version
	for groupVersion, apiTypes := range groupedAPIs {
		if err := g.generateGroupVersionDoc(markdownDir, groupVersion, apiTypes); err != nil {
			return fmt.Errorf("failed to generate docs for %s: %w", groupVersion, err)
		}
	}

	// Generate individual resource documentation
	for _, apiType := range g.apiTypes {
		if err := g.generateResourceDoc(markdownDir, apiType); err != nil {
			return fmt.Errorf("failed to generate docs for %s: %w", apiType.Kind, err)
		}
	}

	return nil
}

// generateIndex generates the main index page.
func (g *MarkdownGenerator) generateIndex(outputDir string) error {
	var content strings.Builder

	content.WriteString("# KCP API Reference\n\n")
	content.WriteString("This is the complete API reference for KCP (Kubernetes Control Plane).\n\n")
	content.WriteString("KCP is a prototype of a Kubernetes API server that can host multiple virtual control planes, ")
	content.WriteString("providing strong isolation, multi-tenancy, and scalable workload distribution.\n\n")

	// Group APIs by group and version
	groupedAPIs := g.groupAPIsByGroupVersion()
	
	// Create table of contents
	content.WriteString("## API Groups\n\n")
	
	var groups []string
	for groupVersion := range groupedAPIs {
		groups = append(groups, groupVersion)
	}
	sort.Strings(groups)

	for _, groupVersion := range groups {
		apiTypes := groupedAPIs[groupVersion]
		content.WriteString(fmt.Sprintf("### %s\n\n", groupVersion))
		
		// Sort API types by kind
		sort.Slice(apiTypes, func(i, j int) bool {
			return apiTypes[i].Kind < apiTypes[j].Kind
		})

		for _, apiType := range apiTypes {
			fileName := g.getResourceFileName(apiType)
			content.WriteString(fmt.Sprintf("- [%s](%s) - %s\n", 
				apiType.Kind, fileName, g.getShortDescription(apiType.Description)))
		}
		content.WriteString("\n")
	}

	// Add quick start section
	content.WriteString("## Quick Start\n\n")
	content.WriteString("To get started with KCP APIs:\n\n")
	content.WriteString("1. **Installation**: Follow the [installation guide](../setup/quickstart.md)\n")
	content.WriteString("2. **Authentication**: Set up [authentication](../concepts/authentication/index.md)\n")
	content.WriteString("3. **Workspaces**: Learn about [workspaces](../concepts/workspaces/index.md)\n")
	content.WriteString("4. **API Exports**: Understand [API exports](../concepts/apis/exporting-apis.md)\n\n")

	// Add common operations
	content.WriteString("## Common Operations\n\n")
	content.WriteString("### List Resources\n")
	content.WriteString("```bash\n")
	content.WriteString("kubectl get apiexports\n")
	content.WriteString("kubectl get apibindings\n")
	content.WriteString("kubectl get workspaces\n")
	content.WriteString("```\n\n")

	content.WriteString("### Create a Workspace\n")
	content.WriteString("```yaml\n")
	content.WriteString("apiVersion: tenancy.kcp.io/v1alpha1\n")
	content.WriteString("kind: Workspace\n")
	content.WriteString("metadata:\n")
	content.WriteString("  name: my-workspace\n")
	content.WriteString("```\n\n")

	return os.WriteFile(filepath.Join(outputDir, "index.md"), []byte(content.String()), 0644)
}

// generateGroupVersionDoc generates documentation for a specific group/version.
func (g *MarkdownGenerator) generateGroupVersionDoc(outputDir, groupVersion string, apiTypes []*APIType) error {
	var content strings.Builder

	parts := strings.Split(groupVersion, "/")
	group := parts[0]
	version := parts[1]

	content.WriteString(fmt.Sprintf("# %s API Reference\n\n", groupVersion))
	content.WriteString(fmt.Sprintf("This document describes the %s API group, version %s.\n\n", group, version))

	// Sort API types by kind
	sort.Slice(apiTypes, func(i, j int) bool {
		return apiTypes[i].Kind < apiTypes[j].Kind
	})

	// Create table of contents
	content.WriteString("## Resources\n\n")
	for _, apiType := range apiTypes {
		fileName := g.getResourceFileName(apiType)
		content.WriteString(fmt.Sprintf("- [%s](%s)\n", apiType.Kind, fileName))
	}
	content.WriteString("\n")

	// Add resource summaries
	content.WriteString("## Resource Summaries\n\n")
	for _, apiType := range apiTypes {
		content.WriteString(fmt.Sprintf("### %s\n\n", apiType.Kind))
		if apiType.Description != "" {
			content.WriteString(fmt.Sprintf("%s\n\n", apiType.Description))
		}
		
		// Add a simple field table
		if len(apiType.Fields) > 0 {
			content.WriteString("**Key Fields:**\n\n")
			content.WriteString("| Field | Type | Description |\n")
			content.WriteString("|-------|------|-------------|\n")
			
			// Show first 5 fields
			count := len(apiType.Fields)
			if count > 5 {
				count = 5
			}
			
			for i := 0; i < count; i++ {
				field := apiType.Fields[i]
				desc := g.getShortDescription(field.Description)
				content.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", 
					field.JSONName, field.Type, desc))
			}
			
			if len(apiType.Fields) > 5 {
				fileName := g.getResourceFileName(apiType)
				content.WriteString(fmt.Sprintf("\n[View complete field reference](%s)\n", fileName))
			}
		}
		content.WriteString("\n")
	}

	fileName := strings.ReplaceAll(groupVersion, "/", "_") + ".md"
	return os.WriteFile(filepath.Join(outputDir, fileName), []byte(content.String()), 0644)
}

// generateResourceDoc generates detailed documentation for a single resource.
func (g *MarkdownGenerator) generateResourceDoc(outputDir string, apiType *APIType) error {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("# %s\n\n", apiType.Kind))
	
	// Add metadata
	content.WriteString(fmt.Sprintf("**API Group:** %s  \n", apiType.Group))
	content.WriteString(fmt.Sprintf("**API Version:** %s  \n", apiType.Version))
	content.WriteString(fmt.Sprintf("**Kind:** %s  \n\n", apiType.Kind))

	// Add description
	if apiType.Description != "" {
		content.WriteString("## Description\n\n")
		content.WriteString(fmt.Sprintf("%s\n\n", apiType.Description))
	}

	// Add example
	content.WriteString("## Example\n\n")
	content.WriteString("```yaml\n")
	content.WriteString(fmt.Sprintf("apiVersion: %s/%s\n", apiType.Group, apiType.Version))
	content.WriteString(fmt.Sprintf("kind: %s\n", apiType.Kind))
	content.WriteString("metadata:\n")
	content.WriteString(fmt.Sprintf("  name: example-%s\n", strings.ToLower(apiType.Kind)))
	
	// Add a simple spec if fields are available
	if len(apiType.Fields) > 0 {
		// Check if there's a spec field
		hasSpec := false
		for _, field := range apiType.Fields {
			if field.JSONName == "spec" {
				hasSpec = true
				break
			}
		}
		
		if hasSpec {
			content.WriteString("spec:\n")
			content.WriteString("  # Configuration goes here\n")
		}
	}
	content.WriteString("```\n\n")

	// Add field reference
	if len(apiType.Fields) > 0 {
		content.WriteString("## Fields\n\n")
		content.WriteString("| Field | Type | Required | Description |\n")
		content.WriteString("|-------|------|----------|-------------|\n")
		
		// Sort fields by name
		sortedFields := make([]*FieldDoc, len(apiType.Fields))
		copy(sortedFields, apiType.Fields)
		sort.Slice(sortedFields, func(i, j int) bool {
			return sortedFields[i].JSONName < sortedFields[j].JSONName
		})

		for _, field := range sortedFields {
			required := "No"
			if field.Required {
				required = "**Yes**"
			}
			
			desc := field.Description
			if desc == "" {
				desc = "*No description provided*"
			}
			
			content.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", 
				field.JSONName, g.formatType(field.Type), required, desc))
		}
		content.WriteString("\n")
	}

	// Add usage section
	content.WriteString("## Usage\n\n")
	content.WriteString(fmt.Sprintf("### Create a %s\n\n", apiType.Kind))
	content.WriteString("```bash\n")
	content.WriteString(fmt.Sprintf("kubectl create -f %s.yaml\n", strings.ToLower(apiType.Kind)))
	content.WriteString("```\n\n")

	content.WriteString(fmt.Sprintf("### List %s resources\n\n", apiType.Kind))
	content.WriteString("```bash\n")
	content.WriteString(fmt.Sprintf("kubectl get %s\n", strings.ToLower(apiType.Kind)))
	content.WriteString("```\n\n")

	content.WriteString(fmt.Sprintf("### Get a specific %s\n\n", apiType.Kind))
	content.WriteString("```bash\n")
	content.WriteString(fmt.Sprintf("kubectl get %s example-%s -o yaml\n", 
		strings.ToLower(apiType.Kind), strings.ToLower(apiType.Kind)))
	content.WriteString("```\n\n")

	content.WriteString(fmt.Sprintf("### Delete a %s\n\n", apiType.Kind))
	content.WriteString("```bash\n")
	content.WriteString(fmt.Sprintf("kubectl delete %s example-%s\n", 
		strings.ToLower(apiType.Kind), strings.ToLower(apiType.Kind)))
	content.WriteString("```\n\n")

	// Add related resources if available
	content.WriteString("## Related Resources\n\n")
	content.WriteString("- [API Reference Index](index.md)\n")
	content.WriteString(fmt.Sprintf("- [%s API Group](%s.md)\n", 
		fmt.Sprintf("%s/%s", apiType.Group, apiType.Version),
		strings.ReplaceAll(fmt.Sprintf("%s_%s", apiType.Group, apiType.Version), ".", "_")))

	fileName := g.getResourceFileName(apiType)
	return os.WriteFile(filepath.Join(outputDir, fileName), []byte(content.String()), 0644)
}

// groupAPIsByGroupVersion groups API types by group and version.
func (g *MarkdownGenerator) groupAPIsByGroupVersion() map[string][]*APIType {
	grouped := make(map[string][]*APIType)

	for _, apiType := range g.apiTypes {
		groupVersion := fmt.Sprintf("%s/%s", apiType.Group, apiType.Version)
		grouped[groupVersion] = append(grouped[groupVersion], apiType)
	}

	return grouped
}

// getResourceFileName generates a filename for a resource's markdown documentation.
func (g *MarkdownGenerator) getResourceFileName(apiType *APIType) string {
	return fmt.Sprintf("%s_%s_%s.md", 
		strings.ReplaceAll(apiType.Group, ".", "_"),
		apiType.Version,
		strings.ToLower(apiType.Kind))
}

// getShortDescription extracts the first sentence or line from a description.
func (g *MarkdownGenerator) getShortDescription(description string) string {
	if description == "" {
		return "*No description available*"
	}

	// Split by sentences or lines
	sentences := strings.FieldsFunc(description, func(c rune) bool {
		return c == '.' || c == '\n'
	})

	if len(sentences) > 0 {
		first := strings.TrimSpace(sentences[0])
		if first != "" {
			// Limit length
			if len(first) > 100 {
				return first[:97] + "..."
			}
			return first
		}
	}

	// Fallback to first 100 characters
	if len(description) > 100 {
		return description[:97] + "..."
	}
	
	return description
}

// formatType formats a Go type for display in documentation.
func (g *MarkdownGenerator) formatType(typeName string) string {
	// Format common types for better readability
	switch {
	case strings.HasPrefix(typeName, "[]"):
		itemType := strings.TrimPrefix(typeName, "[]")
		return fmt.Sprintf("Array of %s", g.formatType(itemType))
	case strings.HasPrefix(typeName, "map["):
		return "Map"
	case strings.HasPrefix(typeName, "*"):
		baseType := strings.TrimPrefix(typeName, "*")
		return fmt.Sprintf("Pointer to %s", g.formatType(baseType))
	case typeName == "interface{}":
		return "Any"
	case strings.Contains(typeName, "."):
		// Remove package prefix for cleaner display
		parts := strings.Split(typeName, ".")
		return parts[len(parts)-1]
	default:
		return typeName
	}
}