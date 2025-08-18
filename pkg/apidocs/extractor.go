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
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

// FieldExtractor extracts field documentation from Go struct types.
type FieldExtractor struct {
	fileSet *token.FileSet
	file    *ast.File
}

// NewFieldExtractor creates a new field extractor.
func NewFieldExtractor(fileSet *token.FileSet, file *ast.File) *FieldExtractor {
	return &FieldExtractor{
		fileSet: fileSet,
		file:    file,
	}
}

// ExtractFields extracts field documentation from a struct type.
func (e *FieldExtractor) ExtractFields(structType *ast.StructType) []*FieldDoc {
	var fields []*FieldDoc

	for _, field := range structType.Fields.List {
		fieldDocs := e.extractFieldDoc(field)
		fields = append(fields, fieldDocs...)
	}

	return fields
}

// extractFieldDoc extracts documentation for a single field.
func (e *FieldExtractor) extractFieldDoc(field *ast.Field) []*FieldDoc {
	var docs []*FieldDoc

	// Handle anonymous fields (embedded structs)
	if len(field.Names) == 0 {
		// This is an embedded field, extract its type information
		typeName := e.extractTypeName(field.Type)
		doc := &FieldDoc{
			Name:     typeName,
			Type:     typeName,
			JSONName: strings.ToLower(typeName),
		}
		
		if field.Doc != nil {
			doc.Description = strings.TrimSpace(field.Doc.Text())
		}
		
		docs = append(docs, doc)
		return docs
	}

	// Handle named fields
	for _, name := range field.Names {
		// Skip unexported fields
		if !ast.IsExported(name.Name) {
			continue
		}

		doc := &FieldDoc{
			Name: name.Name,
			Type: e.extractTypeName(field.Type),
		}

		// Extract field documentation
		if field.Doc != nil {
			doc.Description = strings.TrimSpace(field.Doc.Text())
		}

		// Extract JSON tag information
		if field.Tag != nil {
			doc.JSONName, doc.Required = e.extractJSONTagInfo(field.Tag.Value, name.Name)
		} else {
			doc.JSONName = e.camelCaseToSnakeCase(name.Name)
		}

		// Extract kubebuilder validation markers
		e.extractValidationInfo(doc, field)

		docs = append(docs, doc)
	}

	return docs
}

// extractTypeName extracts the type name from an AST expression.
func (e *FieldExtractor) extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		pkg := e.extractTypeName(t.X)
		return pkg + "." + t.Sel.Name
	case *ast.ArrayType:
		elemType := e.extractTypeName(t.Elt)
		return "[]" + elemType
	case *ast.MapType:
		keyType := e.extractTypeName(t.Key)
		valueType := e.extractTypeName(t.Value)
		return "map[" + keyType + "]" + valueType
	case *ast.StarExpr:
		return "*" + e.extractTypeName(t.X)
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return "unknown"
	}
}

// extractJSONTagInfo extracts JSON tag information from struct field tags.
func (e *FieldExtractor) extractJSONTagInfo(tagValue, fieldName string) (string, bool) {
	// Remove quotes from tag value
	tagValue = strings.Trim(tagValue, "`")

	// Find JSON tag
	jsonTag := e.extractStructTag(tagValue, "json")
	if jsonTag == "" {
		return e.camelCaseToSnakeCase(fieldName), false
	}

	// Parse JSON tag parts
	parts := strings.Split(jsonTag, ",")
	jsonName := parts[0]

	// Handle special JSON tag values
	if jsonName == "-" {
		return "", false // Field is ignored in JSON
	}
	if jsonName == "" {
		jsonName = e.camelCaseToSnakeCase(fieldName)
	}

	// Check for omitempty
	required := true
	for i := 1; i < len(parts); i++ {
		if parts[i] == "omitempty" {
			required = false
			break
		}
	}

	return jsonName, required
}

// extractStructTag extracts a specific tag value from a struct tag string.
func (e *FieldExtractor) extractStructTag(tagString, tagName string) string {
	// Use regex to find the tag value
	pattern := tagName + `:"([^"]*)"`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(tagString)
	
	if len(matches) > 1 {
		return matches[1]
	}
	
	return ""
}

// camelCaseToSnakeCase converts CamelCase to snake_case.
func (e *FieldExtractor) camelCaseToSnakeCase(str string) string {
	// Handle common abbreviations
	str = strings.ReplaceAll(str, "API", "Api")
	str = strings.ReplaceAll(str, "HTTP", "Http")
	str = strings.ReplaceAll(str, "URL", "Url")
	str = strings.ReplaceAll(str, "ID", "Id")

	// Insert underscores before uppercase letters
	re := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	str = re.ReplaceAllString(str, "${1}_${2}")

	return strings.ToLower(str)
}

// extractValidationInfo extracts kubebuilder validation markers from field comments.
func (e *FieldExtractor) extractValidationInfo(doc *FieldDoc, field *ast.Field) {
	if field.Doc == nil {
		return
	}

	comment := field.Doc.Text()
	
	// Extract common kubebuilder validation markers
	markers := []struct {
		pattern string
		handler func(string)
	}{
		{
			pattern: `\+kubebuilder:validation:Required`,
			handler: func(match string) {
				doc.Required = true
			},
		},
		{
			pattern: `\+kubebuilder:validation:Optional`,
			handler: func(match string) {
				doc.Required = false
			},
		},
		{
			pattern: `\+kubebuilder:validation:MinLength=(\d+)`,
			handler: func(match string) {
				if doc.Description != "" {
					doc.Description += "\n"
				}
				doc.Description += "Minimum length: " + strings.TrimPrefix(match, "+kubebuilder:validation:MinLength=")
			},
		},
		{
			pattern: `\+kubebuilder:validation:MaxLength=(\d+)`,
			handler: func(match string) {
				if doc.Description != "" {
					doc.Description += "\n"
				}
				doc.Description += "Maximum length: " + strings.TrimPrefix(match, "+kubebuilder:validation:MaxLength=")
			},
		},
		{
			pattern: `\+kubebuilder:validation:Pattern=(.+)`,
			handler: func(match string) {
				if doc.Description != "" {
					doc.Description += "\n"
				}
				pattern := strings.TrimPrefix(match, "+kubebuilder:validation:Pattern=")
				doc.Description += "Pattern: " + pattern
			},
		},
		{
			pattern: `\+kubebuilder:validation:Enum=(.+)`,
			handler: func(match string) {
				if doc.Description != "" {
					doc.Description += "\n"
				}
				enum := strings.TrimPrefix(match, "+kubebuilder:validation:Enum=")
				doc.Description += "Valid values: " + enum
			},
		},
	}

	for _, marker := range markers {
		re := regexp.MustCompile(marker.pattern)
		matches := re.FindAllString(comment, -1)
		for _, match := range matches {
			marker.handler(match)
		}
	}
}

// CommentExtractor extracts and processes documentation comments.
type CommentExtractor struct{}

// NewCommentExtractor creates a new comment extractor.
func NewCommentExtractor() *CommentExtractor {
	return &CommentExtractor{}
}

// ExtractTypeComment extracts documentation from type-level comments.
func (c *CommentExtractor) ExtractTypeComment(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}

	text := doc.Text()
	
	// Clean up the comment text
	text = strings.TrimSpace(text)
	
	// Remove common Go doc prefixes
	lines := strings.Split(text, "\n")
	var cleanLines []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip kubebuilder markers
		if strings.HasPrefix(line, "+kubebuilder:") {
			continue
		}
		
		// Skip empty lines at the beginning and end
		if line != "" || len(cleanLines) > 0 {
			cleanLines = append(cleanLines, line)
		}
	}

	// Remove trailing empty lines
	for len(cleanLines) > 0 && cleanLines[len(cleanLines)-1] == "" {
		cleanLines = cleanLines[:len(cleanLines)-1]
	}

	return strings.Join(cleanLines, "\n")
}

// ExtractFieldComment extracts documentation from field-level comments.
func (c *CommentExtractor) ExtractFieldComment(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}

	text := doc.Text()
	text = strings.TrimSpace(text)
	
	// For field comments, keep them more concise
	lines := strings.Split(text, "\n")
	var cleanLines []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip kubebuilder markers in description (they're handled separately)
		if strings.HasPrefix(line, "+kubebuilder:") {
			continue
		}
		
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, " ")
}

// PackageExtractor extracts package-level documentation.
type PackageExtractor struct{}

// NewPackageExtractor creates a new package extractor.
func NewPackageExtractor() *PackageExtractor {
	return &PackageExtractor{}
}

// ExtractPackageDoc extracts package documentation from the file.
func (p *PackageExtractor) ExtractPackageDoc(file *ast.File) string {
	if file.Doc == nil {
		return ""
	}

	text := file.Doc.Text()
	return strings.TrimSpace(text)
}