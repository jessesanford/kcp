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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DocumentationGenerator generates API documentation for KCP resources.
// It scans Go source files for API types, extracts documentation comments,
// and generates documentation in multiple formats (OpenAPI, Markdown, Examples).
type DocumentationGenerator struct {
	// SourcePaths contains the root paths to scan for API types
	SourcePaths []string
	// OutputDir is the base directory for generated documentation
	OutputDir string
	// APITypes holds discovered API type information
	APITypes map[schema.GroupVersionKind]*APIType
	// fileSet is used for parsing Go source files
	fileSet *token.FileSet
}

// APIType represents a Kubernetes API type with documentation metadata.
type APIType struct {
	// Kind is the Kubernetes kind (e.g., "APIExport")
	Kind string
	// Group is the API group (e.g., "apis.kcp.io")
	Group string
	// Version is the API version (e.g., "v1alpha1")
	Version string
	// Description is the main documentation for this type
	Description string
	// Fields contains field documentation
	Fields []*FieldDoc
	// Examples contains example YAML for this type
	Examples []string
	// PackagePath is the Go package path
	PackagePath string
	// FileName is the source file name
	FileName string
}

// FieldDoc represents documentation for a single field in an API type.
type FieldDoc struct {
	// Name is the field name
	Name string
	// Type is the field type (e.g., "string", "[]string")
	Type string
	// JSONName is the JSON/YAML field name
	JSONName string
	// Description is the field documentation
	Description string
	// Required indicates if this field is required
	Required bool
	// Nested contains nested field documentation for complex types
	Nested []*FieldDoc
}

// GenerateOptions contains configuration for documentation generation.
type GenerateOptions struct {
	// Formats specifies which documentation formats to generate
	Formats []string // "openapi", "markdown", "examples"
	// IncludePrivate determines whether to include private/internal APIs
	IncludePrivate bool
	// APIGroups filters generation to specific API groups
	APIGroups []string
}

// NewDocumentationGenerator creates a new documentation generator.
func NewDocumentationGenerator(sourcePaths []string, outputDir string) *DocumentationGenerator {
	return &DocumentationGenerator{
		SourcePaths: sourcePaths,
		OutputDir:   outputDir,
		APITypes:    make(map[schema.GroupVersionKind]*APIType),
		fileSet:     token.NewFileSet(),
	}
}

// Generate scans source files and generates documentation in the specified formats.
func (g *DocumentationGenerator) Generate(opts GenerateOptions) error {
	// Discover API types from source files
	if err := g.discoverAPITypes(opts); err != nil {
		return fmt.Errorf("failed to discover API types: %w", err)
	}

	// Generate documentation in requested formats
	for _, format := range opts.Formats {
		switch format {
		case "openapi":
			if err := g.generateOpenAPI(); err != nil {
				return fmt.Errorf("failed to generate OpenAPI spec: %w", err)
			}
		case "markdown":
			if err := g.generateMarkdown(); err != nil {
				return fmt.Errorf("failed to generate Markdown docs: %w", err)
			}
		case "examples":
			if err := g.generateExamples(); err != nil {
				return fmt.Errorf("failed to generate examples: %w", err)
			}
		default:
			return fmt.Errorf("unsupported format: %s", format)
		}
	}

	return nil
}

// discoverAPITypes scans source paths for API type definitions.
func (g *DocumentationGenerator) discoverAPITypes(opts GenerateOptions) error {
	for _, sourcePath := range g.SourcePaths {
		err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip non-Go files
			if !strings.HasSuffix(path, ".go") {
				return nil
			}

			// Skip test files
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}

			// Skip generated files
			if strings.Contains(path, "zz_generated") {
				return nil
			}

			return g.parseSourceFile(path, opts)
		})

		if err != nil {
			return fmt.Errorf("failed to walk source path %s: %w", sourcePath, err)
		}
	}

	return nil
}

// parseSourceFile parses a Go source file and extracts API type information.
func (g *DocumentationGenerator) parseSourceFile(filename string, opts GenerateOptions) error {
	src, err := parser.ParseFile(g.fileSet, filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	// Extract package information
	packagePath := g.extractPackagePath(filename)
	
	// Skip non-API packages if filtering is enabled
	if len(opts.APIGroups) > 0 && !g.matchesAPIGroups(packagePath, opts.APIGroups) {
		return nil
	}

	// Look for type declarations
	ast.Inspect(src, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		// Only process struct types that look like Kubernetes resources
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		// Check if this looks like a Kubernetes API type
		if !g.isKubernetesAPIType(typeSpec, structType) {
			return true
		}

		// Extract API type information
		apiType := g.extractAPIType(typeSpec, structType, packagePath, filename, src)
		if apiType != nil {
			gvk := schema.GroupVersionKind{
				Group:   apiType.Group,
				Version: apiType.Version,
				Kind:    apiType.Kind,
			}
			g.APITypes[gvk] = apiType
		}

		return true
	})

	return nil
}

// extractPackagePath extracts the Go package path from a file path.
func (g *DocumentationGenerator) extractPackagePath(filename string) string {
	// Find the position of "pkg/" or "sdk/" to determine package path
	parts := strings.Split(filename, "/")
	for i, part := range parts {
		if part == "pkg" || part == "sdk" {
			return strings.Join(parts[i:len(parts)-1], "/")
		}
	}
	return filepath.Dir(filename)
}

// matchesAPIGroups checks if a package path matches any of the specified API groups.
func (g *DocumentationGenerator) matchesAPIGroups(packagePath string, apiGroups []string) bool {
	for _, group := range apiGroups {
		if strings.Contains(packagePath, group) {
			return true
		}
	}
	return false
}

// isKubernetesAPIType determines if a type declaration represents a Kubernetes API type.
func (g *DocumentationGenerator) isKubernetesAPIType(typeSpec *ast.TypeSpec, structType *ast.StructType) bool {
	typeName := typeSpec.Name.Name
	
	// Skip types that don't follow Kubernetes naming conventions
	if strings.HasSuffix(typeName, "List") ||
		strings.HasSuffix(typeName, "Status") ||
		strings.HasSuffix(typeName, "Spec") ||
		strings.HasPrefix(typeName, "runtime") {
		return false
	}

	// Look for TypeMeta field to identify Kubernetes resources
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		fieldName := field.Names[0].Name
		if fieldName == "TypeMeta" || fieldName == "metav1.TypeMeta" {
			return true
		}
	}

	return false
}

// extractAPIType extracts API type information from an AST type declaration.
func (g *DocumentationGenerator) extractAPIType(typeSpec *ast.TypeSpec, structType *ast.StructType, 
	packagePath, filename string, file *ast.File) *APIType {
	
	// Extract group and version from package path
	group, version := g.extractGroupVersion(packagePath)
	
	apiType := &APIType{
		Kind:        typeSpec.Name.Name,
		Group:       group,
		Version:     version,
		PackagePath: packagePath,
		FileName:    filename,
		Fields:      []*FieldDoc{},
	}

	// Extract type documentation from comments
	if typeSpec.Doc != nil {
		apiType.Description = strings.TrimSpace(typeSpec.Doc.Text())
	}

	// Extract field documentation
	extractor := NewFieldExtractor(g.fileSet, file)
	apiType.Fields = extractor.ExtractFields(structType)

	return apiType
}

// extractGroupVersion extracts API group and version from package path.
func (g *DocumentationGenerator) extractGroupVersion(packagePath string) (string, string) {
	parts := strings.Split(packagePath, "/")
	
	var group, version string
	
	// Look for version pattern (v1alpha1, v1beta1, v1, etc.)
	for i, part := range parts {
		if strings.HasPrefix(part, "v1") || strings.HasPrefix(part, "v2") {
			version = part
			// Group is typically the part before version
			if i > 0 {
				group = parts[i-1]
			}
			break
		}
	}

	// Default group extraction for KCP
	if group == "" {
		if strings.Contains(packagePath, "apis") {
			group = "apis.kcp.io"
		} else if strings.Contains(packagePath, "tenancy") {
			group = "tenancy.kcp.io"
		} else if strings.Contains(packagePath, "core") {
			group = "core.kcp.io"
		} else if strings.Contains(packagePath, "cache") {
			group = "cache.kcp.io"
		}
	}

	if version == "" {
		version = "v1alpha1" // Default version
	}

	return group, version
}

// generateOpenAPI generates OpenAPI 3.0 specification.
func (g *DocumentationGenerator) generateOpenAPI() error {
	generator := NewOpenAPIGenerator(g.APITypes, g.OutputDir)
	return generator.Generate()
}

// generateMarkdown generates Markdown documentation.
func (g *DocumentationGenerator) generateMarkdown() error {
	generator := NewMarkdownGenerator(g.APITypes, g.OutputDir)
	return generator.Generate()
}

// generateExamples generates example YAML files.
func (g *DocumentationGenerator) generateExamples() error {
	generator := NewExampleGenerator(g.APITypes, g.OutputDir)
	return generator.Generate()
}