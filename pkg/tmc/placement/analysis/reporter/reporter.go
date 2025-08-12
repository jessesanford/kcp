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
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// AnalysisResult represents the result of placement analysis for a workload.
type AnalysisResult struct {
	// WorkloadRef is the reference to the workload being analyzed
	WorkloadRef WorkloadReference `json:"workloadRef"`
	
	// Timestamp when the analysis was performed
	Timestamp metav1.Time `json:"timestamp"`
	
	// ClusterAnalyses contains the analysis results for each candidate cluster
	ClusterAnalyses []ClusterAnalysis `json:"clusterAnalyses"`
	
	// RecommendedClusters are the clusters recommended for placement
	RecommendedClusters []string `json:"recommendedClusters"`
	
	// ConstraintViolations contains any constraint violations found
	ConstraintViolations []ConstraintViolation `json:"constraintViolations,omitempty"`
	
	// AnalysisStatus indicates the overall status of the analysis
	AnalysisStatus AnalysisStatus `json:"analysisStatus"`
	
	// Message provides additional context about the analysis
	Message string `json:"message,omitempty"`
}

// WorkloadReference identifies a workload in the system.
type WorkloadReference struct {
	// Name of the workload
	Name string `json:"name"`
	
	// Namespace of the workload (empty for cluster-scoped resources)
	Namespace string `json:"namespace,omitempty"`
	
	// Kind of the workload (e.g., Deployment, StatefulSet)
	Kind string `json:"kind"`
	
	// APIVersion of the workload
	APIVersion string `json:"apiVersion"`
	
	// UID uniquely identifies the workload
	UID types.UID `json:"uid"`
}

// ClusterAnalysis represents the analysis result for a specific cluster.
type ClusterAnalysis struct {
	// ClusterName is the name of the cluster
	ClusterName string `json:"clusterName"`
	
	// Suitable indicates if the cluster is suitable for the workload
	Suitable bool `json:"suitable"`
	
	// Score is the overall suitability score (0-100)
	Score int32 `json:"score"`
	
	// Reasons contains the reasons for the suitability determination
	Reasons []string `json:"reasons"`
	
	// ResourceAvailability indicates resource availability in the cluster
	ResourceAvailability ResourceAvailability `json:"resourceAvailability"`
	
	// ConstraintsSatisfied indicates which constraints are satisfied
	ConstraintsSatisfied []string `json:"constraintsSatisfied"`
	
	// ConstraintsViolated indicates which constraints are violated
	ConstraintsViolated []string `json:"constraintsViolated"`
}

// ResourceAvailability represents the resource availability in a cluster.
type ResourceAvailability struct {
	// CPU availability (millicores)
	CPU int64 `json:"cpu"`
	
	// Memory availability (bytes)
	Memory int64 `json:"memory"`
	
	// Storage availability (bytes)
	Storage int64 `json:"storage,omitempty"`
	
	// NodeCount is the number of available nodes
	NodeCount int32 `json:"nodeCount"`
	
	// AvailabilityZones lists the availability zones
	AvailabilityZones []string `json:"availabilityZones,omitempty"`
}

// ConstraintViolation represents a constraint violation.
type ConstraintViolation struct {
	// ConstraintType is the type of constraint that was violated
	ConstraintType string `json:"constraintType"`
	
	// Description describes the violation
	Description string `json:"description"`
	
	// Severity indicates the severity of the violation
	Severity ViolationSeverity `json:"severity"`
	
	// AffectedClusters lists the clusters affected by this violation
	AffectedClusters []string `json:"affectedClusters,omitempty"`
}

// AnalysisStatus represents the overall status of an analysis.
type AnalysisStatus string

const (
	// AnalysisStatusSuccess indicates successful analysis
	AnalysisStatusSuccess AnalysisStatus = "Success"
	
	// AnalysisStatusPartial indicates partial analysis (some clusters failed)
	AnalysisStatusPartial AnalysisStatus = "Partial"
	
	// AnalysisStatusFailed indicates failed analysis
	AnalysisStatusFailed AnalysisStatus = "Failed"
	
	// AnalysisStatusInProgress indicates analysis is in progress
	AnalysisStatusInProgress AnalysisStatus = "InProgress"
)

// ViolationSeverity represents the severity of a constraint violation.
type ViolationSeverity string

const (
	// ViolationSeverityError indicates a hard constraint violation
	ViolationSeverityError ViolationSeverity = "Error"
	
	// ViolationSeverityWarning indicates a soft constraint violation
	ViolationSeverityWarning ViolationSeverity = "Warning"
	
	// ViolationSeverityInfo indicates an informational constraint violation
	ViolationSeverityInfo ViolationSeverity = "Info"
)

// ReportFormat specifies the format for generated reports.
type ReportFormat string

const (
	// ReportFormatText generates human-readable text reports
	ReportFormatText ReportFormat = "text"
	
	// ReportFormatJSON generates JSON reports
	ReportFormatJSON ReportFormat = "json"
	
	// ReportFormatYAML generates YAML reports
	ReportFormatYAML ReportFormat = "yaml"
	
	// ReportFormatMarkdown generates Markdown reports
	ReportFormatMarkdown ReportFormat = "markdown"
)

// Reporter is the interface for generating placement analysis reports.
type Reporter interface {
	// GenerateReport generates a report from analysis results
	GenerateReport(ctx context.Context, result *AnalysisResult, format ReportFormat) ([]byte, error)
	
	// GenerateSummaryReport generates a summary report for multiple workloads
	GenerateSummaryReport(ctx context.Context, results []*AnalysisResult, format ReportFormat) ([]byte, error)
	
	// ValidateResult validates an analysis result for completeness and consistency
	ValidateResult(ctx context.Context, result *AnalysisResult) error
}

// DefaultReporter implements the Reporter interface with default reporting logic.
type DefaultReporter struct {
	formatter Formatter
}

// NewDefaultReporter creates a new DefaultReporter with the specified formatter.
func NewDefaultReporter(formatter Formatter) *DefaultReporter {
	return &DefaultReporter{
		formatter: formatter,
	}
}

// GenerateReport generates a report from analysis results.
func (r *DefaultReporter) GenerateReport(ctx context.Context, result *AnalysisResult, format ReportFormat) ([]byte, error) {
	// Validate the result first (must be done before logging to avoid nil pointer)
	if err := r.ValidateResult(ctx, result); err != nil {
		return nil, fmt.Errorf("invalid analysis result: %w", err)
	}
	
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Generating placement analysis report",
		"workload", result.WorkloadRef.Name,
		"format", format,
		"clusterCount", len(result.ClusterAnalyses))
	
	// Generate the report using the formatter
	report, err := r.formatter.FormatAnalysisResult(ctx, result, format)
	if err != nil {
		return nil, fmt.Errorf("failed to format analysis result: %w", err)
	}
	
	logger.V(4).Info("Successfully generated placement analysis report",
		"workload", result.WorkloadRef.Name,
		"reportSize", len(report))
	
	return report, nil
}

// GenerateSummaryReport generates a summary report for multiple workloads.
func (r *DefaultReporter) GenerateSummaryReport(ctx context.Context, results []*AnalysisResult, format ReportFormat) ([]byte, error) {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Generating summary report", "resultCount", len(results), "format", format)
	
	if len(results) == 0 {
		return nil, fmt.Errorf("no analysis results provided")
	}
	
	// Validate all results
	for i, result := range results {
		if err := r.ValidateResult(ctx, result); err != nil {
			return nil, fmt.Errorf("invalid analysis result at index %d: %w", i, err)
		}
	}
	
	// Generate the summary report using the formatter
	report, err := r.formatter.FormatSummaryReport(ctx, results, format)
	if err != nil {
		return nil, fmt.Errorf("failed to format summary report: %w", err)
	}
	
	logger.V(4).Info("Successfully generated summary report", "reportSize", len(report))
	
	return report, nil
}

// ValidateResult validates an analysis result for completeness and consistency.
func (r *DefaultReporter) ValidateResult(ctx context.Context, result *AnalysisResult) error {
	if result == nil {
		return fmt.Errorf("analysis result is nil")
	}
	
	// Validate workload reference
	if result.WorkloadRef.Name == "" {
		return fmt.Errorf("workload reference name is empty")
	}
	if result.WorkloadRef.Kind == "" {
		return fmt.Errorf("workload reference kind is empty")
	}
	if result.WorkloadRef.APIVersion == "" {
		return fmt.Errorf("workload reference apiVersion is empty")
	}
	if result.WorkloadRef.UID == "" {
		return fmt.Errorf("workload reference UID is empty")
	}
	
	// Validate timestamp
	if result.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is not set")
	}
	
	// Validate analysis status
	switch result.AnalysisStatus {
	case AnalysisStatusSuccess, AnalysisStatusPartial, AnalysisStatusFailed, AnalysisStatusInProgress:
		// Valid statuses
	default:
		return fmt.Errorf("invalid analysis status: %s", result.AnalysisStatus)
	}
	
	// Validate cluster analyses
	if len(result.ClusterAnalyses) == 0 && result.AnalysisStatus != AnalysisStatusFailed {
		return fmt.Errorf("no cluster analyses provided for non-failed analysis")
	}
	
	for i, analysis := range result.ClusterAnalyses {
		if analysis.ClusterName == "" {
			return fmt.Errorf("cluster analysis at index %d has empty cluster name", i)
		}
		if analysis.Score < 0 || analysis.Score > 100 {
			return fmt.Errorf("cluster analysis at index %d has invalid score: %d", i, analysis.Score)
		}
	}
	
	// Validate constraint violations
	for i, violation := range result.ConstraintViolations {
		if violation.ConstraintType == "" {
			return fmt.Errorf("constraint violation at index %d has empty constraint type", i)
		}
		if violation.Description == "" {
			return fmt.Errorf("constraint violation at index %d has empty description", i)
		}
		switch violation.Severity {
		case ViolationSeverityError, ViolationSeverityWarning, ViolationSeverityInfo:
			// Valid severities
		default:
			return fmt.Errorf("constraint violation at index %d has invalid severity: %s", i, violation.Severity)
		}
	}
	
	// Validate recommended clusters exist in cluster analyses
	clusterNames := make(map[string]bool)
	for _, analysis := range result.ClusterAnalyses {
		clusterNames[analysis.ClusterName] = true
	}
	
	for _, recommendedCluster := range result.RecommendedClusters {
		if !clusterNames[recommendedCluster] {
			return fmt.Errorf("recommended cluster %s not found in cluster analyses", recommendedCluster)
		}
	}
	
	return nil
}

// CreateAnalysisResult creates a new AnalysisResult with the specified parameters.
func CreateAnalysisResult(workloadRef WorkloadReference, clusterAnalyses []ClusterAnalysis) *AnalysisResult {
	// Calculate recommended clusters based on analysis
	var recommendedClusters []string
	for _, analysis := range clusterAnalyses {
		if analysis.Suitable {
			recommendedClusters = append(recommendedClusters, analysis.ClusterName)
		}
	}
	
	// Determine overall analysis status
	var status AnalysisStatus
	if len(recommendedClusters) > 0 {
		status = AnalysisStatusSuccess
	} else if len(clusterAnalyses) > 0 {
		status = AnalysisStatusPartial
	} else {
		status = AnalysisStatusFailed
	}
	
	// Collect constraint violations
	var violations []ConstraintViolation
	for _, analysis := range clusterAnalyses {
		for _, constraint := range analysis.ConstraintsViolated {
			violation := ConstraintViolation{
				ConstraintType:   constraint,
				Description:      fmt.Sprintf("Constraint %s violated in cluster %s", constraint, analysis.ClusterName),
				Severity:         ViolationSeverityWarning,
				AffectedClusters: []string{analysis.ClusterName},
			}
			violations = append(violations, violation)
		}
	}
	
	return &AnalysisResult{
		WorkloadRef:          workloadRef,
		Timestamp:           metav1.NewTime(time.Now()),
		ClusterAnalyses:     clusterAnalyses,
		RecommendedClusters: recommendedClusters,
		ConstraintViolations: violations,
		AnalysisStatus:      status,
		Message:             fmt.Sprintf("Analysis completed for workload %s", workloadRef.Name),
	}
}

// PlacementSessionSpec represents the specification of a placement session.
// This is a minimal definition for the reporter to use.
type PlacementSessionSpec struct {
	WorkloadRef WorkloadRef `json:"workloadRef"`
}

// WorkloadRef represents a reference to a workload.
type WorkloadRef struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
}

// PlacementSession represents a placement session object.
// This is a minimal definition for the reporter to use.
type PlacementSession struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PlacementSessionSpec `json:"spec"`
}

// CreateWorkloadReference creates a WorkloadReference from a placement session.
func CreateWorkloadReference(session *PlacementSession) WorkloadReference {
	return WorkloadReference{
		Name:       session.Spec.WorkloadRef.Name,
		Namespace:  session.Spec.WorkloadRef.Namespace,
		Kind:       session.Spec.WorkloadRef.Kind,
		APIVersion: session.Spec.WorkloadRef.APIVersion,
		UID:        session.UID,
	}
}