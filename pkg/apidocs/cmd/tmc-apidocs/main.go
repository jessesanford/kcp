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

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kcp-dev/kcp/pkg/apidocs"
)

const (
	programName = "tmc-apidocs"
	version     = "v1.0.0"
)

// Config holds the configuration for the documentation generator.
type Config struct {
	SourcePaths    []string
	OutputDir      string
	Formats        []string
	APIGroups      []string
	IncludePrivate bool
	Verbose        bool
	Version        bool
	Help           bool
}

func main() {
	config := parseFlags()

	if config.Version {
		fmt.Printf("%s %s\n", programName, version)
		os.Exit(0)
	}

	if config.Help {
		printUsage()
		os.Exit(0)
	}

	if err := validateConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Use -help for usage information.\n")
		os.Exit(1)
	}

	if err := runGeneration(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating documentation: %v\n", err)
		os.Exit(1)
	}

	if config.Verbose {
		fmt.Printf("Documentation generated successfully in %s\n", config.OutputDir)
	}
}

// parseFlags parses command-line flags and returns a Config.
func parseFlags() *Config {
	config := &Config{}

	var sourcePaths string
	var formats string
	var apiGroups string

	flag.StringVar(&sourcePaths, "source", "pkg/,sdk/", 
		"Comma-separated list of source paths to scan for API types")
	flag.StringVar(&config.OutputDir, "output", "./docs", 
		"Output directory for generated documentation")
	flag.StringVar(&formats, "format", "openapi,markdown,examples", 
		"Comma-separated list of formats to generate (openapi, markdown, examples)")
	flag.StringVar(&apiGroups, "groups", "", 
		"Comma-separated list of API groups to include (empty = all groups)")
	flag.BoolVar(&config.IncludePrivate, "include-private", false, 
		"Include private/internal APIs in documentation")
	flag.BoolVar(&config.Verbose, "verbose", false, 
		"Enable verbose output")
	flag.BoolVar(&config.Version, "version", false, 
		"Show version information")
	flag.BoolVar(&config.Help, "help", false, 
		"Show help information")

	flag.Parse()

	// Parse comma-separated values
	config.SourcePaths = parseCommaSeparated(sourcePaths)
	config.Formats = parseCommaSeparated(formats)
	config.APIGroups = parseCommaSeparated(apiGroups)

	return config
}

// parseCommaSeparated parses a comma-separated string into a slice.
func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	
	return result
}

// validateConfig validates the configuration.
func validateConfig(config *Config) error {
	if len(config.SourcePaths) == 0 {
		return fmt.Errorf("at least one source path must be specified")
	}

	if config.OutputDir == "" {
		return fmt.Errorf("output directory must be specified")
	}

	if len(config.Formats) == 0 {
		return fmt.Errorf("at least one format must be specified")
	}

	// Validate formats
	validFormats := map[string]bool{
		"openapi":  true,
		"markdown": true,
		"examples": true,
	}

	for _, format := range config.Formats {
		if !validFormats[format] {
			return fmt.Errorf("unsupported format: %s (valid formats: openapi, markdown, examples)", format)
		}
	}

	// Validate source paths exist
	for _, sourcePath := range config.SourcePaths {
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			return fmt.Errorf("source path does not exist: %s", sourcePath)
		}
	}

	return nil
}

// runGeneration runs the documentation generation process.
func runGeneration(config *Config) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if config.Verbose {
		fmt.Printf("Scanning source paths: %v\n", config.SourcePaths)
		fmt.Printf("Output directory: %s\n", config.OutputDir)
		fmt.Printf("Formats: %v\n", config.Formats)
		if len(config.APIGroups) > 0 {
			fmt.Printf("API groups filter: %v\n", config.APIGroups)
		}
	}

	// Convert relative source paths to absolute paths
	absoluteSourcePaths := make([]string, len(config.SourcePaths))
	for i, sourcePath := range config.SourcePaths {
		absPath, err := filepath.Abs(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to convert source path to absolute: %w", err)
		}
		absoluteSourcePaths[i] = absPath
	}

	// Create documentation generator
	generator := apidocs.NewDocumentationGenerator(absoluteSourcePaths, config.OutputDir)

	// Configure generation options
	opts := apidocs.GenerateOptions{
		Formats:        config.Formats,
		IncludePrivate: config.IncludePrivate,
		APIGroups:      config.APIGroups,
	}

	// Generate documentation
	if err := generator.Generate(opts); err != nil {
		return err
	}

	// Print summary if verbose
	if config.Verbose {
		if err := printGenerationSummary(config, generator); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to print summary: %v\n", err)
		}
	}

	return nil
}

// printGenerationSummary prints a summary of what was generated.
func printGenerationSummary(config *Config, generator *apidocs.DocumentationGenerator) error {
	fmt.Printf("\nGeneration Summary:\n")
	fmt.Printf("==================\n")

	// Count API types by group
	groupCounts := make(map[string]int)
	totalTypes := 0

	for _, apiType := range generator.APITypes {
		group := apiType.Group
		if group == "" {
			group = "core"
		}
		groupCounts[group]++
		totalTypes++
	}

	fmt.Printf("Total API types processed: %d\n", totalTypes)
	fmt.Printf("API groups found:\n")
	for group, count := range groupCounts {
		fmt.Printf("  - %s: %d types\n", group, count)
	}

	fmt.Printf("\nGenerated outputs:\n")
	for _, format := range config.Formats {
		outputPath := filepath.Join(config.OutputDir, format)
		fmt.Printf("  - %s: %s/\n", format, outputPath)
	}

	return nil
}

// printUsage prints usage information.
func printUsage() {
	fmt.Printf("%s - Generate API documentation for KCP resources\n\n", programName)
	fmt.Printf("Usage:\n")
	fmt.Printf("  %s [flags]\n\n", programName)
	fmt.Printf("Examples:\n")
	fmt.Printf("  # Generate all formats to ./docs\n")
	fmt.Printf("  %s -source pkg/,sdk/ -output ./docs\n\n", programName)
	fmt.Printf("  # Generate only OpenAPI spec\n")
	fmt.Printf("  %s -format openapi -output ./api-spec\n\n", programName)
	fmt.Printf("  # Generate docs for specific API groups\n")
	fmt.Printf("  %s -groups apis.kcp.io,tenancy.kcp.io -output ./docs\n\n", programName)
	fmt.Printf("  # Include private APIs with verbose output\n")
	fmt.Printf("  %s -include-private -verbose -output ./docs\n\n", programName)
	fmt.Printf("Flags:\n")
	flag.PrintDefaults()
	fmt.Printf("\nSupported formats:\n")
	fmt.Printf("  openapi   - Generate OpenAPI 3.0 specifications (JSON and YAML)\n")
	fmt.Printf("  markdown  - Generate Markdown documentation\n")
	fmt.Printf("  examples  - Generate example YAML files\n")
	fmt.Printf("\nFor more information, visit: https://github.com/kcp-dev/kcp\n")
}