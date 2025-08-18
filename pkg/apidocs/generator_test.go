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
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewDocumentationGenerator(t *testing.T) {
	sourcePaths := []string{"/path/to/source"}
	outputDir := "/path/to/output"

	generator := NewDocumentationGenerator(sourcePaths, outputDir)

	if generator == nil {
		t.Fatal("Expected non-nil generator")
	}

	if len(generator.SourcePaths) != 1 || generator.SourcePaths[0] != "/path/to/source" {
		t.Errorf("Expected source path '/path/to/source', got %v", generator.SourcePaths)
	}

	if generator.OutputDir != "/path/to/output" {
		t.Errorf("Expected output dir '/path/to/output', got %s", generator.OutputDir)
	}

	if generator.APITypes == nil {
		t.Error("Expected non-nil APITypes map")
	}

	if generator.fileSet == nil {
		t.Error("Expected non-nil fileSet")
	}
}

func TestExtractGroupVersion(t *testing.T) {
	generator := NewDocumentationGenerator(nil, "")

	tests := []struct {
		name        string
		packagePath string
		wantGroup   string
		wantVersion string
	}{
		{
			name:        "apis package with v1alpha1",
			packagePath: "sdk/apis/apis/v1alpha1",
			wantGroup:   "apis", // Gets "apis" from part before version
			wantVersion: "v1alpha1",
		},
		{
			name:        "tenancy package with v1alpha1",
			packagePath: "sdk/apis/tenancy/v1alpha1",
			wantGroup:   "tenancy", // Gets "tenancy" from part before version
			wantVersion: "v1alpha1",
		},
		{
			name:        "core package with v1alpha1",
			packagePath: "sdk/apis/core/v1alpha1",
			wantGroup:   "core", // Gets "core" from part before version
			wantVersion: "v1alpha1",
		},
		{
			name:        "cache package with v1alpha1",
			packagePath: "sdk/apis/cache/v1alpha1",
			wantGroup:   "cache", // Gets "cache" from part before version
			wantVersion: "v1alpha1",
		},
		{
			name:        "unknown package defaults",
			packagePath: "unknown/package/v1beta1",
			wantGroup:   "package", // Will extract "package" before version
			wantVersion: "v1beta1",
		},
		{
			name:        "no version defaults",
			packagePath: "some/package/path",
			wantGroup:   "",
			wantVersion: "v1alpha1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, version := generator.extractGroupVersion(tt.packagePath)
			if group != tt.wantGroup {
				t.Errorf("extractGroupVersion() group = %v, want %v", group, tt.wantGroup)
			}
			if version != tt.wantVersion {
				t.Errorf("extractGroupVersion() version = %v, want %v", version, tt.wantVersion)
			}
		})
	}
}

func TestGenerateWithMockAPITypes(t *testing.T) {
	// Create temporary output directory
	tempDir, err := os.MkdirTemp("", "apidocs_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create generator with mock data
	generator := NewDocumentationGenerator([]string{}, tempDir)
	
	// Add mock API type
	gvk := schema.GroupVersionKind{
		Group:   "apis.kcp.io",
		Version: "v1alpha1",
		Kind:    "MockResource",
	}
	
	generator.APITypes[gvk] = &APIType{
		Kind:        "MockResource",
		Group:       "apis.kcp.io",
		Version:     "v1alpha1",
		Description: "A mock resource for testing",
		Fields: []*FieldDoc{
			{
				Name:        "Spec",
				Type:        "MockSpec",
				JSONName:    "spec",
				Description: "The specification for the mock resource",
				Required:    true,
			},
		},
		PackagePath: "test/package",
		FileName:    "mock.go",
	}

	// Test generation with different formats
	formats := []string{"openapi", "markdown", "examples"}
	opts := GenerateOptions{
		Formats:        formats,
		IncludePrivate: false,
	}

	err = generator.Generate(opts)
	if err != nil {
		t.Fatalf("Failed to generate documentation: %v", err)
	}

	// Verify output directories were created
	for _, format := range formats {
		formatDir := filepath.Join(tempDir, format)
		if _, err := os.Stat(formatDir); os.IsNotExist(err) {
			t.Errorf("Expected %s directory to be created", format)
		}
	}

	// Verify some output files exist
	openAPIFile := filepath.Join(tempDir, "openapi", "kcp-api-combined.json")
	if _, err := os.Stat(openAPIFile); os.IsNotExist(err) {
		t.Error("Expected combined OpenAPI JSON file to be created")
	}

	markdownIndex := filepath.Join(tempDir, "markdown", "index.md")
	if _, err := os.Stat(markdownIndex); os.IsNotExist(err) {
		t.Error("Expected Markdown index file to be created")
	}

	examplesIndex := filepath.Join(tempDir, "examples", "README.md")
	if _, err := os.Stat(examplesIndex); os.IsNotExist(err) {
		t.Error("Expected examples README file to be created")
	}
}

func TestMatchesAPIGroups(t *testing.T) {
	generator := NewDocumentationGenerator(nil, "")

	tests := []struct {
		name        string
		packagePath string
		apiGroups   []string
		want        bool
	}{
		{
			name:        "matches single group",
			packagePath: "sdk/apis/tenancy/v1alpha1",
			apiGroups:   []string{"tenancy"},
			want:        true,
		},
		{
			name:        "matches multiple groups",
			packagePath: "sdk/apis/apis/v1alpha1",
			apiGroups:   []string{"tenancy", "apis"},
			want:        true,
		},
		{
			name:        "no match",
			packagePath: "sdk/core/v1alpha1", // No "apis" or "tenancy" in path
			apiGroups:   []string{"tenancy", "apis"},
			want:        false,
		},
		{
			name:        "empty groups matches all",
			packagePath: "any/path",
			apiGroups:   []string{},
			want:        false, // Empty slice means no filtering
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generator.matchesAPIGroups(tt.packagePath, tt.apiGroups); got != tt.want {
				t.Errorf("matchesAPIGroups() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractPackagePath(t *testing.T) {
	generator := NewDocumentationGenerator(nil, "")

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "pkg path",
			filename: "/workspaces/kcp/pkg/apis/types.go",
			want:     "pkg/apis",
		},
		{
			name:     "sdk path",
			filename: "/workspaces/kcp/sdk/apis/core/v1alpha1/types.go",
			want:     "sdk/apis/core/v1alpha1",
		},
		{
			name:     "no pkg or sdk",
			filename: "/some/other/path/file.go",
			want:     "/some/other/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generator.extractPackagePath(tt.filename); got != tt.want {
				t.Errorf("extractPackagePath() = %v, want %v", got, tt.want)
			}
		})
	}
}