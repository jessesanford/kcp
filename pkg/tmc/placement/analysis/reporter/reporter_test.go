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
	"encoding/json"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestDefaultReporter_GenerateReport(t *testing.T) {
	formatter, err := NewDefaultFormatter()
	if err != nil {
		t.Fatalf("failed to create formatter: %v", err)
	}
	
	reporter := NewDefaultReporter(formatter)
	ctx := context.Background()
	
	tests := map[string]struct {
		result      *AnalysisResult
		format      ReportFormat
		wantError   bool
		contains    []string
	}{
		"successful analysis with JSON format": {
			result: createTestAnalysisResult("test-workload", AnalysisStatusSuccess),
			format: ReportFormatJSON,
			contains: []string{
				`"workloadRef"`,
				`"clusterAnalyses"`,
				`"recommendedClusters"`,
				`"test-workload"`,
			},
		},
		"successful analysis with text format": {
			result: createTestAnalysisResult("test-workload", AnalysisStatusSuccess),
			format: ReportFormatText,
			contains: []string{
				"TMC Placement Analysis Report",
				"test-workload",
				"Recommended Clusters",
				"Suitable Clusters",
			},
		},
		"invalid result - nil": {
			result:    nil,
			format:    ReportFormatJSON,
			wantError: true,
		},
		"unsupported format": {
			result:    createTestAnalysisResult("test-workload", AnalysisStatusSuccess),
			format:    "unsupported",
			wantError: true,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report, err := reporter.GenerateReport(ctx, tc.result, tc.format)
			
			if tc.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			reportStr := string(report)
			
			for _, expected := range tc.contains {
				if !strings.Contains(reportStr, expected) {
					t.Errorf("report does not contain expected string: %s", expected)
				}
			}
		})
	}
}

func TestDefaultReporter_ValidateResult(t *testing.T) {
	formatter, err := NewDefaultFormatter()
	if err != nil {
		t.Fatalf("failed to create formatter: %v", err)
	}
	
	reporter := NewDefaultReporter(formatter)
	ctx := context.Background()
	
	tests := map[string]struct {
		result    *AnalysisResult
		wantError bool
		errorMsg  string
	}{
		"valid result": {
			result:    createTestAnalysisResult("test-workload", AnalysisStatusSuccess),
			wantError: false,
		},
		"nil result": {
			result:    nil,
			wantError: true,
			errorMsg:  "analysis result is nil",
		},
		"empty workload name": {
			result: &AnalysisResult{
				WorkloadRef: WorkloadReference{
					Name: "",
					Kind: "Deployment",
					APIVersion: "apps/v1",
					UID: "test-uid",
				},
				Timestamp:      metav1.NewTime(time.Now()),
				AnalysisStatus: AnalysisStatusSuccess,
			},
			wantError: true,
			errorMsg:  "workload reference name is empty",
		},
		"invalid analysis status": {
			result: &AnalysisResult{
				WorkloadRef: WorkloadReference{
					Name: "test-workload",
					Kind: "Deployment",
					APIVersion: "apps/v1",
					UID: "test-uid",
				},
				Timestamp:      metav1.NewTime(time.Now()),
				AnalysisStatus: "InvalidStatus",
			},
			wantError: true,
			errorMsg:  "invalid analysis status",
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := reporter.ValidateResult(ctx, tc.result)
			
			if tc.wantError {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				if tc.errorMsg != "" && !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("expected error to contain '%s' but got: %v", tc.errorMsg, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCreateAnalysisResult(t *testing.T) {
	workloadRef := WorkloadReference{
		Name:       "test-workload",
		Namespace:  "test-namespace",
		Kind:       "Deployment",
		APIVersion: "apps/v1",
		UID:        "test-uid",
	}
	
	clusterAnalyses := []ClusterAnalysis{
		{
			ClusterName: "suitable-cluster",
			Suitable:    true,
			Score:       90,
			Reasons:     []string{"good fit"},
		},
		{
			ClusterName: "unsuitable-cluster",
			Suitable:    false,
			Score:       30,
			Reasons:     []string{"insufficient resources"},
			ConstraintsViolated: []string{"memory-constraint"},
		},
	}
	
	result := CreateAnalysisResult(workloadRef, clusterAnalyses)
	
	if result == nil {
		t.Fatal("CreateAnalysisResult returned nil")
	}
	
	if result.WorkloadRef != workloadRef {
		t.Errorf("workload reference mismatch: got %+v, want %+v", result.WorkloadRef, workloadRef)
	}
	
	if len(result.ClusterAnalyses) != 2 {
		t.Errorf("expected 2 cluster analyses, got %d", len(result.ClusterAnalyses))
	}
	
	if len(result.RecommendedClusters) != 1 || result.RecommendedClusters[0] != "suitable-cluster" {
		t.Errorf("expected recommended clusters to be ['suitable-cluster'], got %v", result.RecommendedClusters)
	}
	
	if result.AnalysisStatus != AnalysisStatusSuccess {
		t.Errorf("expected analysis status Success, got %s", result.AnalysisStatus)
	}
}

func TestDefaultFormatter_FormatAnalysisResult(t *testing.T) {
	formatter, err := NewDefaultFormatter()
	if err != nil {
		t.Fatalf("failed to create formatter: %v", err)
	}
	
	ctx := context.Background()
	result := createTestAnalysisResult("test-workload", AnalysisStatusSuccess)
	
	tests := map[string]struct {
		format      ReportFormat
		wantError   bool
		validateFn  func(t *testing.T, report []byte)
	}{
		"JSON format": {
			format:    ReportFormatJSON,
			wantError: false,
			validateFn: func(t *testing.T, report []byte) {
				var parsed AnalysisResult
				if err := json.Unmarshal(report, &parsed); err != nil {
					t.Errorf("failed to parse JSON: %v", err)
				}
				if parsed.WorkloadRef.Name != "test-workload" {
					t.Errorf("expected workload name 'test-workload', got %s", parsed.WorkloadRef.Name)
				}
			},
		},
		"Text format": {
			format:    ReportFormatText,
			wantError: false,
			validateFn: func(t *testing.T, report []byte) {
				reportStr := string(report)
				if !strings.Contains(reportStr, "TMC Placement Analysis Report") {
					t.Error("text report missing header")
				}
			},
		},
		"Unsupported format": {
			format:    "invalid",
			wantError: true,
		},
	}
	
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			report, err := formatter.FormatAnalysisResult(ctx, result, tc.format)
			
			if tc.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if tc.validateFn != nil {
				tc.validateFn(t, report)
			}
		})
	}
}

// Helper function to create a test analysis result
func createTestAnalysisResult(workloadName string, status AnalysisStatus) *AnalysisResult {
	return &AnalysisResult{
		WorkloadRef: WorkloadReference{
			Name:       workloadName,
			Namespace:  "default",
			Kind:       "Deployment",
			APIVersion: "apps/v1",
			UID:        types.UID("test-uid-" + workloadName),
		},
		Timestamp: metav1.NewTime(time.Now()),
		ClusterAnalyses: []ClusterAnalysis{
			{
				ClusterName: "cluster-1",
				Suitable:    status == AnalysisStatusSuccess,
				Score:       90,
				Reasons:     []string{"good fit"},
				ResourceAvailability: ResourceAvailability{
					CPU:       2000,
					Memory:    4096000000,
					NodeCount: 3,
				},
				ConstraintsSatisfied: []string{"location-constraint"},
			},
		},
		RecommendedClusters: func() []string {
			if status == AnalysisStatusSuccess {
				return []string{"cluster-1"}
			}
			return []string{}
		}(),
		ConstraintViolations: []ConstraintViolation{},
		AnalysisStatus:       status,
		Message:              "Test analysis completed",
	}
}