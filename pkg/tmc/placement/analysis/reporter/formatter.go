/*
Copyright 2024 The KCP Authors.

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

package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

// Formatter is the interface for formatting analysis results into different output formats.
type Formatter interface {
	// FormatAnalysisResult formats a single analysis result
	FormatAnalysisResult(ctx context.Context, result *AnalysisResult, format ReportFormat) ([]byte, error)
	
	// FormatSummaryReport formats a summary of multiple analysis results
	FormatSummaryReport(ctx context.Context, results []*AnalysisResult, format ReportFormat) ([]byte, error)
}

// DefaultFormatter implements the Formatter interface with built-in templates.
type DefaultFormatter struct {
	textTemplate     *template.Template
	markdownTemplate *template.Template
}

// NewDefaultFormatter creates a new DefaultFormatter with built-in templates.
func NewDefaultFormatter() (*DefaultFormatter, error) {
	// Template functions
	funcMap := template.FuncMap{
		"join": strings.Join,
	}
	
	textTmpl, err := template.New("text").Funcs(funcMap).Parse(textReportTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse text template: %w", err)
	}
	
	markdownTmpl, err := template.New("markdown").Funcs(funcMap).Parse(markdownReportTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown template: %w", err)
	}
	
	return &DefaultFormatter{
		textTemplate:     textTmpl,
		markdownTemplate: markdownTmpl,
	}, nil
}

// FormatAnalysisResult formats a single analysis result.
func (f *DefaultFormatter) FormatAnalysisResult(ctx context.Context, result *AnalysisResult, format ReportFormat) ([]byte, error) {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Formatting analysis result", "format", format, "workload", result.WorkloadRef.Name)
	
	switch format {
	case ReportFormatJSON:
		return json.MarshalIndent(result, "", "  ")
	case ReportFormatYAML:
		return yaml.Marshal(result)
	case ReportFormatText:
		return f.formatText(result, f.textTemplate)
	case ReportFormatMarkdown:
		return f.formatText(result, f.markdownTemplate)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// FormatSummaryReport formats a summary of multiple analysis results.
func (f *DefaultFormatter) FormatSummaryReport(ctx context.Context, results []*AnalysisResult, format ReportFormat) ([]byte, error) {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Formatting summary report", "format", format, "resultCount", len(results))
	
	summary := f.createSummary(results)
	
	switch format {
	case ReportFormatJSON:
		return json.MarshalIndent(summary, "", "  ")
	case ReportFormatYAML:
		return yaml.Marshal(summary)
	case ReportFormatText:
		return f.formatSummaryText(summary)
	case ReportFormatMarkdown:
		return f.formatSummaryMarkdown(summary)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// formatText formats a result using the specified template.
func (f *DefaultFormatter) formatText(result *AnalysisResult, tmpl *template.Template) ([]byte, error) {
	var buf bytes.Buffer
	
	// Prepare template data
	data := struct {
		*AnalysisResult
		FormattedTimestamp string
		SuitableClusters   []ClusterAnalysis
		UnsuitableClusters []ClusterAnalysis
	}{
		AnalysisResult:     result,
		FormattedTimestamp: result.Timestamp.Format(time.RFC3339),
	}
	
	// Separate suitable and unsuitable clusters
	for _, analysis := range result.ClusterAnalyses {
		if analysis.Suitable {
			data.SuitableClusters = append(data.SuitableClusters, analysis)
		} else {
			data.UnsuitableClusters = append(data.UnsuitableClusters, analysis)
		}
	}
	
	// Sort clusters by score (descending)
	sort.Slice(data.SuitableClusters, func(i, j int) bool {
		return data.SuitableClusters[i].Score > data.SuitableClusters[j].Score
	})
	sort.Slice(data.UnsuitableClusters, func(i, j int) bool {
		return data.UnsuitableClusters[i].Score > data.UnsuitableClusters[j].Score
	})
	
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	
	return buf.Bytes(), nil
}

// SummaryData represents aggregated data for summary reports.
type SummaryData struct {
	TotalWorkloads      int                     `json:"totalWorkloads"`
	SuccessfulAnalyses  int                     `json:"successfulAnalyses"`
	PartialAnalyses     int                     `json:"partialAnalyses"`
	FailedAnalyses      int                     `json:"failedAnalyses"`
	TotalClusters       int                     `json:"totalClusters"`
	GenerationTime      time.Time               `json:"generationTime"`
	WorkloadSummaries   []WorkloadSummary       `json:"workloadSummaries"`
	ClusterUtilization  map[string]int          `json:"clusterUtilization"`
	CommonViolations    []ConstraintViolation   `json:"commonViolations"`
}

// WorkloadSummary represents a summary for a single workload.
type WorkloadSummary struct {
	WorkloadRef           WorkloadReference `json:"workloadRef"`
	Status                AnalysisStatus    `json:"status"`
	RecommendedCount      int               `json:"recommendedCount"`
	ConstraintViolations  int               `json:"constraintViolations"`
	TopCluster            string            `json:"topCluster,omitempty"`
	TopClusterScore       int32             `json:"topClusterScore,omitempty"`
}

// createSummary creates a summary from multiple analysis results.
func (f *DefaultFormatter) createSummary(results []*AnalysisResult) *SummaryData {
	summary := &SummaryData{
		TotalWorkloads:     len(results),
		GenerationTime:     time.Now(),
		ClusterUtilization: make(map[string]int),
	}
	
	clusterSet := make(map[string]bool)
	violationMap := make(map[string]*ConstraintViolation)
	
	for _, result := range results {
		// Count analysis statuses
		switch result.AnalysisStatus {
		case AnalysisStatusSuccess:
			summary.SuccessfulAnalyses++
		case AnalysisStatusPartial:
			summary.PartialAnalyses++
		case AnalysisStatusFailed:
			summary.FailedAnalyses++
		}
		
		// Create workload summary
		workloadSummary := WorkloadSummary{
			WorkloadRef:          result.WorkloadRef,
			Status:               result.AnalysisStatus,
			RecommendedCount:     len(result.RecommendedClusters),
			ConstraintViolations: len(result.ConstraintViolations),
		}
		
		// Find top cluster
		var topScore int32 = -1
		for _, analysis := range result.ClusterAnalyses {
			clusterSet[analysis.ClusterName] = true
			
			// Track cluster utilization
			if analysis.Suitable {
				summary.ClusterUtilization[analysis.ClusterName]++
			}
			
			// Find top scoring cluster
			if analysis.Score > topScore {
				topScore = analysis.Score
				workloadSummary.TopCluster = analysis.ClusterName
				workloadSummary.TopClusterScore = analysis.Score
			}
		}
		
		summary.WorkloadSummaries = append(summary.WorkloadSummaries, workloadSummary)
		
		// Aggregate constraint violations
		for _, violation := range result.ConstraintViolations {
			key := fmt.Sprintf("%s:%s", violation.ConstraintType, violation.Severity)
			if existing, exists := violationMap[key]; exists {
				// Merge affected clusters
				clusterMap := make(map[string]bool)
				for _, cluster := range existing.AffectedClusters {
					clusterMap[cluster] = true
				}
				for _, cluster := range violation.AffectedClusters {
					clusterMap[cluster] = true
				}
				
				var mergedClusters []string
				for cluster := range clusterMap {
					mergedClusters = append(mergedClusters, cluster)
				}
				existing.AffectedClusters = mergedClusters
			} else {
				violationMap[key] = &ConstraintViolation{
					ConstraintType:   violation.ConstraintType,
					Description:      violation.Description,
					Severity:         violation.Severity,
					AffectedClusters: violation.AffectedClusters,
				}
			}
		}
	}
	
	summary.TotalClusters = len(clusterSet)
	
	// Convert violation map to slice
	for _, violation := range violationMap {
		summary.CommonViolations = append(summary.CommonViolations, *violation)
	}
	
	return summary
}

// formatSummaryText formats a summary as plain text.
func (f *DefaultFormatter) formatSummaryText(summary *SummaryData) ([]byte, error) {
	var buf bytes.Buffer
	
	// Header
	buf.WriteString("TMC Placement Analysis Summary Report\n")
	buf.WriteString(strings.Repeat("=", 40) + "\n\n")
	
	// Overview
	buf.WriteString(fmt.Sprintf("Generated: %s\n", summary.GenerationTime.Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("Total Workloads: %d\n", summary.TotalWorkloads))
	buf.WriteString(fmt.Sprintf("Total Clusters: %d\n", summary.TotalClusters))
	buf.WriteString("\n")
	
	// Analysis Status Summary
	buf.WriteString("Analysis Status Summary:\n")
	buf.WriteString(fmt.Sprintf("  Successful: %d\n", summary.SuccessfulAnalyses))
	buf.WriteString(fmt.Sprintf("  Partial:    %d\n", summary.PartialAnalyses))
	buf.WriteString(fmt.Sprintf("  Failed:     %d\n", summary.FailedAnalyses))
	buf.WriteString("\n")
	
	// Cluster Utilization
	buf.WriteString("Cluster Utilization:\n")
	for cluster, count := range summary.ClusterUtilization {
		buf.WriteString(fmt.Sprintf("  %s: %d workloads\n", cluster, count))
	}
	buf.WriteString("\n")
	
	// Common Violations
	if len(summary.CommonViolations) > 0 {
		buf.WriteString("Common Constraint Violations:\n")
		for _, violation := range summary.CommonViolations {
			buf.WriteString(fmt.Sprintf("  %s (%s): %s\n",
				violation.ConstraintType, violation.Severity, violation.Description))
		}
		buf.WriteString("\n")
	}
	
	return buf.Bytes(), nil
}

// formatSummaryMarkdown formats a summary as markdown.
func (f *DefaultFormatter) formatSummaryMarkdown(summary *SummaryData) ([]byte, error) {
	var buf bytes.Buffer
	
	// Header
	buf.WriteString("# TMC Placement Analysis Summary Report\n\n")
	
	// Overview
	buf.WriteString("## Overview\n\n")
	buf.WriteString(fmt.Sprintf("- **Generated:** %s\n", summary.GenerationTime.Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("- **Total Workloads:** %d\n", summary.TotalWorkloads))
	buf.WriteString(fmt.Sprintf("- **Total Clusters:** %d\n", summary.TotalClusters))
	buf.WriteString("\n")
	
	// Analysis Status Summary
	buf.WriteString("## Analysis Status Summary\n\n")
	buf.WriteString("| Status | Count |\n")
	buf.WriteString("|--------|-------|\n")
	buf.WriteString(fmt.Sprintf("| Successful | %d |\n", summary.SuccessfulAnalyses))
	buf.WriteString(fmt.Sprintf("| Partial | %d |\n", summary.PartialAnalyses))
	buf.WriteString(fmt.Sprintf("| Failed | %d |\n", summary.FailedAnalyses))
	buf.WriteString("\n")
	
	// Cluster Utilization
	buf.WriteString("## Cluster Utilization\n\n")
	buf.WriteString("| Cluster | Workload Count |\n")
	buf.WriteString("|---------|----------------|\n")
	for cluster, count := range summary.ClusterUtilization {
		buf.WriteString(fmt.Sprintf("| %s | %d |\n", cluster, count))
	}
	buf.WriteString("\n")
	
	// Common Violations
	if len(summary.CommonViolations) > 0 {
		buf.WriteString("## Common Constraint Violations\n\n")
		buf.WriteString("| Constraint Type | Severity | Description |\n")
		buf.WriteString("|----------------|----------|-------------|\n")
		for _, violation := range summary.CommonViolations {
			buf.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
				violation.ConstraintType, violation.Severity, violation.Description))
		}
		buf.WriteString("\n")
	}
	
	return buf.Bytes(), nil
}

// Text report template
const textReportTemplate = `TMC Placement Analysis Report
=============================

Workload: {{.WorkloadRef.Name}} ({{.WorkloadRef.Kind}})
Namespace: {{if .WorkloadRef.Namespace}}{{.WorkloadRef.Namespace}}{{else}}<cluster-scoped>{{end}}
Analysis Time: {{.FormattedTimestamp}}
Status: {{.AnalysisStatus}}

{{if .Message}}
Message: {{.Message}}
{{end}}

Recommended Clusters ({{len .RecommendedClusters}}):
{{range .RecommendedClusters}}  - {{.}}
{{end}}
{{if not .RecommendedClusters}}  None
{{end}}

Suitable Clusters:
{{range .SuitableClusters}}  - {{.ClusterName}} (Score: {{.Score}})
    CPU: {{.ResourceAvailability.CPU}}m, Memory: {{.ResourceAvailability.Memory}} bytes
    Nodes: {{.ResourceAvailability.NodeCount}}
    Reasons: {{join .Reasons ", "}}
{{end}}
{{if not .SuitableClusters}}  None
{{end}}

Unsuitable Clusters:
{{range .UnsuitableClusters}}  - {{.ClusterName}} (Score: {{.Score}})
    CPU: {{.ResourceAvailability.CPU}}m, Memory: {{.ResourceAvailability.Memory}} bytes
    Nodes: {{.ResourceAvailability.NodeCount}}
    Reasons: {{join .Reasons ", "}}
    Violations: {{join .ConstraintsViolated ", "}}
{{end}}
{{if not .UnsuitableClusters}}  None
{{end}}

{{if .ConstraintViolations}}
Constraint Violations:
{{range .ConstraintViolations}}  - {{.ConstraintType}} ({{.Severity}}): {{.Description}}
{{end}}
{{end}}`

// Markdown report template
const markdownReportTemplate = `# TMC Placement Analysis Report

## Workload Information
- **Name:** {{.WorkloadRef.Name}}
- **Kind:** {{.WorkloadRef.Kind}}
- **Namespace:** {{if .WorkloadRef.Namespace}}{{.WorkloadRef.Namespace}}{{else}}_cluster-scoped_{{end}}
- **Analysis Time:** {{.FormattedTimestamp}}
- **Status:** {{.AnalysisStatus}}

{{if .Message}}
**Message:** {{.Message}}
{{end}}

## Recommended Clusters ({{len .RecommendedClusters}})

{{if .RecommendedClusters}}
{{range .RecommendedClusters}}
- {{.}}
{{end}}
{{else}}
_None_
{{end}}

## Cluster Analysis

### Suitable Clusters

{{if .SuitableClusters}}
| Cluster | Score | CPU | Memory | Nodes | Reasons |
|---------|-------|-----|--------|-------|---------|
{{range .SuitableClusters}}| {{.ClusterName}} | {{.Score}} | {{.ResourceAvailability.CPU}}m | {{.ResourceAvailability.Memory}} bytes | {{.ResourceAvailability.NodeCount}} | {{join .Reasons ", "}} |
{{end}}
{{else}}
_No suitable clusters found_
{{end}}

### Unsuitable Clusters

{{if .UnsuitableClusters}}
| Cluster | Score | CPU | Memory | Nodes | Violations |
|---------|-------|-----|--------|-------|------------|
{{range .UnsuitableClusters}}| {{.ClusterName}} | {{.Score}} | {{.ResourceAvailability.CPU}}m | {{.ResourceAvailability.Memory}} bytes | {{.ResourceAvailability.NodeCount}} | {{join .ConstraintsViolated ", "}} |
{{end}}
{{else}}
_No unsuitable clusters_
{{end}}

{{if .ConstraintViolations}}
## Constraint Violations

| Constraint Type | Severity | Description |
|----------------|----------|-------------|
{{range .ConstraintViolations}}| {{.ConstraintType}} | {{.Severity}} | {{.Description}} |
{{end}}
{{end}}`