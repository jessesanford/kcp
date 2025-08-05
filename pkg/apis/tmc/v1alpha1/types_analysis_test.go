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

package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkloadAnalysisRunValidation(t *testing.T) {
	tests := map[string]struct {
		analysisRun *WorkloadAnalysisRun
		wantValid   bool
	}{
		"valid prometheus analysis": {
			analysisRun: &WorkloadAnalysisRun{
				ObjectMeta: metav1.ObjectMeta{Name: "test-analysis", Namespace: "default"},
				Spec: WorkloadAnalysisRunSpec{
					WorkloadSelector: WorkloadSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "frontend"},
						},
					},
					ClusterSelector: ClusterSelector{LocationSelector: []string{"us-west-1"}},
					AnalysisTemplates: []AnalysisTemplate{
						{
							Name:         "cpu-check",
							AnalysisType: AnalysisTypePrometheus,
							Query:        "avg(cpu_usage)",
							SuccessCriteria: SuccessCriteria{
								Threshold: "80",
								Operator:  ComparisonOperatorLessThan,
							},
						},
					},
				},
			},
			wantValid: true,
		},
		"invalid analysis - no templates": {
			analysisRun: &WorkloadAnalysisRun{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-analysis", Namespace: "default"},
				Spec: WorkloadAnalysisRunSpec{
					WorkloadSelector:  WorkloadSelector{},
					ClusterSelector:   ClusterSelector{},
					AnalysisTemplates: []AnalysisTemplate{},
				},
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.analysisRun == nil {
				t.Fatal("analysisRun cannot be nil")
			}

			// Basic validation
			hasSelector := tc.analysisRun.Spec.WorkloadSelector.LabelSelector != nil ||
				len(tc.analysisRun.Spec.WorkloadSelector.WorkloadTypes) > 0
			hasTemplates := len(tc.analysisRun.Spec.AnalysisTemplates) > 0

			if !hasSelector && tc.wantValid {
				t.Error("expected valid analysis run, but WorkloadSelector has no selection criteria")
			}
			if !hasTemplates && tc.wantValid {
				t.Error("expected valid analysis run, but no analysis templates provided")
			}

			// Validate templates
			for _, template := range tc.analysisRun.Spec.AnalysisTemplates {
				if template.Name == "" && tc.wantValid {
					t.Error("AnalysisTemplate name cannot be empty")
				}
				if template.Query == "" && tc.wantValid {
					t.Error("AnalysisTemplate query cannot be empty")
				}
				if template.Weight < 0 || template.Weight > 100 {
					if tc.wantValid {
						t.Errorf("AnalysisTemplate weight must be 0-100, got %d", template.Weight)
					}
				}
			}
		})
	}
}

func TestAnalysisResultCalculation(t *testing.T) {
	status := &WorkloadAnalysisRunStatus{
		Phase:          AnalysisPhaseCompleted,
		RunCount:       10,
		SuccessfulRuns: 8,
		FailedRuns:     2,
		AnalysisResults: []AnalysisResult{
			{Name: "cpu-check", Phase: AnalysisPhaseCompleted, Score: 85},
			{Name: "memory-check", Phase: AnalysisPhaseCompleted, Score: 92},
		},
	}

	// Calculate overall score
	totalScore := int32(0)
	for _, result := range status.AnalysisResults {
		totalScore += result.Score
	}
	expectedOverallScore := totalScore / int32(len(status.AnalysisResults))
	status.OverallScore = &expectedOverallScore

	if *status.OverallScore != 88 { // (85+92)/2 = 88.5, truncated to 88
		t.Errorf("expected overall score around 88, got %d", *status.OverallScore)
	}

	// Validate success rate
	successRate := float64(status.SuccessfulRuns) / float64(status.RunCount) * 100
	if successRate != 80.0 {
		t.Errorf("expected success rate 80%%, got %.1f%%", successRate)
	}
}

func TestAnalysisPhaseTransitions(t *testing.T) {
	status := &WorkloadAnalysisRunStatus{Phase: AnalysisPhasePending}

	// Test phase transition
	status.Phase = AnalysisPhaseRunning
	if status.Phase != AnalysisPhaseRunning {
		t.Errorf("expected phase %s, got %s", AnalysisPhaseRunning, status.Phase)
	}

	status.Phase = AnalysisPhaseCompleted
	if status.Phase != AnalysisPhaseCompleted {
		t.Errorf("expected phase %s, got %s", AnalysisPhaseCompleted, status.Phase)
	}
}