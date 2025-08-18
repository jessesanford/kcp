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
	"go/parser"
	"go/token"
	"testing"
)

func TestCamelCaseToSnakeCase(t *testing.T) {
	extractor := NewFieldExtractor(nil, nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"APIVersion", "api_version"},
		{"HTTPProxy", "http_proxy"},
		{"URLPath", "url_path"},
		{"IDGenerator", "id_generator"},
		{"SimpleField", "simple_field"},
		{"Field", "field"},
		{"XMLHttpRequest", "xmlhttp_request"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := extractor.camelCaseToSnakeCase(test.input)
			if result != test.expected {
				t.Errorf("camelCaseToSnakeCase(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

func TestExtractJSONTagInfo(t *testing.T) {
	extractor := NewFieldExtractor(nil, nil)

	tests := []struct {
		name      string
		tagValue  string
		fieldName string
		wantJSON  string
		wantReq   bool
	}{
		{
			name:      "basic json tag",
			tagValue:  "`json:\"name\"`",
			fieldName: "Name",
			wantJSON:  "name",
			wantReq:   true,
		},
		{
			name:      "json tag with omitempty",
			tagValue:  "`json:\"description,omitempty\"`",
			fieldName: "Description",
			wantJSON:  "description",
			wantReq:   false,
		},
		{
			name:      "json tag with dash (ignored)",
			tagValue:  "`json:\"-\"`",
			fieldName: "IgnoredField",
			wantJSON:  "",
			wantReq:   false,
		},
		{
			name:      "no json tag",
			tagValue:  "`yaml:\"name\"`",
			fieldName: "FieldName",
			wantJSON:  "field_name",
			wantReq:   false,
		},
		{
			name:      "empty json tag",
			tagValue:  "`json:\"\"`",
			fieldName: "AutoNamed",
			wantJSON:  "auto_named",
			wantReq:   false, // Default behavior when no omitempty is specified
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJSON, gotReq := extractor.extractJSONTagInfo(tt.tagValue, tt.fieldName)
			if gotJSON != tt.wantJSON {
				t.Errorf("extractJSONTagInfo() JSON = %v, want %v", gotJSON, tt.wantJSON)
			}
			if gotReq != tt.wantReq {
				t.Errorf("extractJSONTagInfo() Required = %v, want %v", gotReq, tt.wantReq)
			}
		})
	}
}

func TestExtractStructTag(t *testing.T) {
	extractor := NewFieldExtractor(nil, nil)

	tests := []struct {
		name      string
		tagString string
		tagName   string
		want      string
	}{
		{
			name:      "extract json tag",
			tagString: `json:"name,omitempty" yaml:"name"`,
			tagName:   "json",
			want:      "name,omitempty",
		},
		{
			name:      "extract yaml tag",
			tagString: `json:"name,omitempty" yaml:"name"`,
			tagName:   "yaml",
			want:      "name",
		},
		{
			name:      "tag not found",
			tagString: `json:"name" yaml:"name"`,
			tagName:   "xml",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractor.extractStructTag(tt.tagString, tt.tagName)
			if got != tt.want {
				t.Errorf("extractStructTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractTypeName(t *testing.T) {
	// Create a simple source file for testing
	src := `
package test

import "time"

type TestStruct struct {
	StringField    string
	IntField       int
	SliceField     []string
	MapField       map[string]int
	PointerField   *string
	TimeField      time.Time
	InterfaceField interface{}
}
`

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse test source: %v", err)
	}

	extractor := NewFieldExtractor(fileSet, file)

	// Find the struct type
	var structType *ast.StructType
	ast.Inspect(file, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "TestStruct" {
			if st, ok := ts.Type.(*ast.StructType); ok {
				structType = st
				return false
			}
		}
		return true
	})

	if structType == nil {
		t.Fatal("Could not find TestStruct in parsed file")
	}

	tests := []struct {
		fieldName string
		wantType  string
	}{
		{"StringField", "string"},
		{"IntField", "int"},
		{"SliceField", "[]string"},
		{"MapField", "map[string]int"},
		{"PointerField", "*string"},
		{"TimeField", "time.Time"},
		{"InterfaceField", "interface{}"},
	}

	fieldsByName := make(map[string]*ast.Field)
	for _, field := range structType.Fields.List {
		if len(field.Names) > 0 {
			fieldsByName[field.Names[0].Name] = field
		}
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			field := fieldsByName[tt.fieldName]
			if field == nil {
				t.Fatalf("Field %s not found", tt.fieldName)
			}

			got := extractor.extractTypeName(field.Type)
			if got != tt.wantType {
				t.Errorf("extractTypeName() = %v, want %v", got, tt.wantType)
			}
		})
	}
}

func TestExtractFieldComment(t *testing.T) {
	commentExtractor := NewCommentExtractor()

	tests := []struct {
		name     string
		comment  string
		expected string
	}{
		{
			name:     "simple comment",
			comment:  "// This is a field comment",
			expected: "This is a field comment",
		},
		{
			name:     "multiline comment",
			comment:  "// Line 1\n// Line 2",
			expected: "Line 1 // Line 2", // ast.CommentGroup.Text() preserves // prefixes
		},
		{
			name:     "comment with kubebuilder marker",
			comment:  "// This is a field\n// +kubebuilder:validation:Required",
			expected: "This is a field // +kubebuilder:validation:Required", // Filtering happens in extraction
		},
		{
			name:     "empty comment",
			comment:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock comment group
			var commentGroup *ast.CommentGroup
			if tt.comment != "" {
				commentGroup = &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: tt.comment},
					},
				}
			}

			got := commentExtractor.ExtractFieldComment(commentGroup)
			if got != tt.expected {
				t.Errorf("ExtractFieldComment() = %q, want %q", got, tt.expected)
			}
		})
	}
}