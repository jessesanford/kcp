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

package processor

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewAnalysisDataProcessor(t *testing.T) {
	tests := map[string]struct {
		config   *ProcessorConfig
		wantNil  bool
	}{
		"creates processor with default config when nil": {
			config:  nil,
			wantNil: false,
		},
		"creates processor with custom config": {
			config: &ProcessorConfig{
				MaxConcurrentProcessing: 5,
				ProcessingTimeout:       2 * time.Minute,
				RetryAttempts:          2,
				RetryBackoff:           15 * time.Second,
			},
			wantNil: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			processor := NewAnalysisDataProcessor(tc.config)
			
			if (processor == nil) != tc.wantNil {
				t.Errorf("NewAnalysisDataProcessor() = %v, want nil: %v", processor, tc.wantNil)
			}
			
			if processor != nil {
				if processor.metrics == nil {
					t.Error("processor.metrics should not be nil")
				}
				if processor.config == nil {
					t.Error("processor.config should not be nil")
				}
			}
		})
	}
}

func TestAnalysisDataProcessor_ProcessAnalysis(t *testing.T) {
	processor := NewAnalysisDataProcessor(nil)
	ctx := context.Background()

	tests := map[string]struct {
		analysis    *WorkloadAnalysisRun
		wantError   bool
		wantScore   int32
	}{
		"processes valid analysis successfully": {
			analysis: &WorkloadAnalysisRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-analysis",
					Namespace: "default",
				},
				Status: WorkloadAnalysisRunStatus{
					Phase: AnalysisPhaseCompleted,
					AnalysisResults: []AnalysisResult{
						{
							Name:  "test-result-1",
							Phase: AnalysisPhaseCompleted,
							Score: 80,
							Measurements: []AnalysisMeasurement{
								{
									Phase:     AnalysisPhaseCompleted,
									Value:     "0.8",
									StartedAt: metav1.Now(),
								},
							},
						},
					},
				},
			},
			wantError: false,
			wantScore: 50, // Expected normalized score
		},
		"fails with nil analysis": {
			analysis:  nil,
			wantError: true,
		},
		"fails with empty analysis name": {
			analysis: &WorkloadAnalysisRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "default",
				},
				Status: WorkloadAnalysisRunStatus{
					Phase: AnalysisPhaseCompleted,
				},
			},
			wantError: true,
		},
		"fails with empty phase": {
			analysis: &WorkloadAnalysisRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-analysis",
					Namespace: "default",
				},
				Status: WorkloadAnalysisRunStatus{
					Phase: "",
				},
			},
			wantError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := processor.ProcessAnalysis(ctx, tc.analysis)
			
			if (err != nil) != tc.wantError {
				t.Errorf("ProcessAnalysis() error = %v, wantError %v", err, tc.wantError)
				return
			}
			
			if !tc.wantError {
				if result == nil {
					t.Error("expected non-nil result for successful processing")
					return
				}
				
				if result.AnalysisName != tc.analysis.Name {
					t.Errorf("ProcessingResult.AnalysisName = %v, want %v", result.AnalysisName, tc.analysis.Name)
				}
				
				if result.Score != tc.wantScore {
					t.Errorf("ProcessingResult.Score = %v, want %v", result.Score, tc.wantScore)
				}
				
				if result.ProcessedAt.IsZero() {
					t.Error("ProcessingResult.ProcessedAt should not be zero")
				}
			}
		})
	}
}

func TestAnalysisDataProcessor_ProcessBatch(t *testing.T) {
	processor := NewAnalysisDataProcessor(&ProcessorConfig{
		MaxConcurrentProcessing: 2,
		ProcessingTimeout:       1 * time.Minute,
	})
	ctx := context.Background()

	tests := map[string]struct {
		analyses    []*WorkloadAnalysisRun
		wantError   bool
		wantResults int
	}{
		"processes empty batch successfully": {
			analyses:    []*WorkloadAnalysisRun{},
			wantError:   false,
			wantResults: 0,
		},
		"processes multiple analyses concurrently": {
			analyses: []*WorkloadAnalysisRun{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "analysis-1", Namespace: "default"},
					Status:     WorkloadAnalysisRunStatus{Phase: AnalysisPhaseCompleted},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "analysis-2", Namespace: "default"},
					Status:     WorkloadAnalysisRunStatus{Phase: AnalysisPhaseCompleted},
				},
			},
			wantError:   false,
			wantResults: 2,
		},
		"handles mixed valid and invalid analyses": {
			analyses: []*WorkloadAnalysisRun{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "analysis-1", Namespace: "default"},
					Status:     WorkloadAnalysisRunStatus{Phase: AnalysisPhaseCompleted},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "", Namespace: "default"}, // Invalid
					Status:     WorkloadAnalysisRunStatus{Phase: AnalysisPhaseCompleted},
				},
			},
			wantError:   true,
			wantResults: 2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			results, err := processor.ProcessBatch(ctx, tc.analyses)
			
			if (err != nil) != tc.wantError {
				t.Errorf("ProcessBatch() error = %v, wantError %v", err, tc.wantError)
				return
			}
			
			if len(results) != tc.wantResults {
				t.Errorf("ProcessBatch() returned %d results, want %d", len(results), tc.wantResults)
			}
		})
	}
}

func TestAnalysisDataProcessor_GetMetrics(t *testing.T) {
	processor := NewAnalysisDataProcessor(nil)
	
	// Get initial metrics
	initialMetrics := processor.GetMetrics()
	
	if initialMetrics.ProcessedAnalyses != 0 {
		t.Errorf("initial ProcessedAnalyses = %d, want 0", initialMetrics.ProcessedAnalyses)
	}
	
	if initialMetrics.FailedAnalyses != 0 {
		t.Errorf("initial FailedAnalyses = %d, want 0", initialMetrics.FailedAnalyses)
	}
	
	// Update metrics manually to test
	processor.updateMetrics(100*time.Millisecond, true)
	processor.updateMetrics(200*time.Millisecond, false)
	
	updatedMetrics := processor.GetMetrics()
	
	if updatedMetrics.ProcessedAnalyses != 1 {
		t.Errorf("ProcessedAnalyses = %d, want 1", updatedMetrics.ProcessedAnalyses)
	}
	
	if updatedMetrics.FailedAnalyses != 1 {
		t.Errorf("FailedAnalyses = %d, want 1", updatedMetrics.FailedAnalyses)
	}
	
	if updatedMetrics.AverageProcessingTime == 0 {
		t.Error("AverageProcessingTime should not be zero after updates")
	}
}


// Benchmark tests for performance validation

func BenchmarkAnalysisDataProcessor_ProcessAnalysis(b *testing.B) {
	processor := NewAnalysisDataProcessor(nil)
	ctx := context.Background()
	
	analysis := &WorkloadAnalysisRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bench-analysis",
			Namespace: "default",
		},
		Status: WorkloadAnalysisRunStatus{
			Phase: AnalysisPhaseCompleted,
			AnalysisResults: []AnalysisResult{
				{
					Name:  "bench-result",
					Phase: AnalysisPhaseCompleted,
					Score: 75,
					Measurements: []AnalysisMeasurement{
						{
							Phase:     AnalysisPhaseCompleted,
							Value:     "0.75",
							StartedAt: metav1.Now(),
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.ProcessAnalysis(ctx, analysis)
		if err != nil {
			b.Fatalf("ProcessAnalysis failed: %v", err)
		}
	}
}

