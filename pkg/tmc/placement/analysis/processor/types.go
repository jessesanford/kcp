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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// Temporary local types until TMC APIs are available
// These will be replaced with imports from pkg/apis/tmc/v1alpha1 in the final implementation

// WorkloadAnalysisRun represents a temporary local type for testing
type WorkloadAnalysisRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadAnalysisRunSpec   `json:"spec,omitempty"`
	Status WorkloadAnalysisRunStatus `json:"status,omitempty"`
}

// WorkloadAnalysisRunSpec defines the spec for WorkloadAnalysisRun
type WorkloadAnalysisRunSpec struct {
	// Placeholder spec fields
}

// WorkloadAnalysisRunStatus defines the status for WorkloadAnalysisRun
type WorkloadAnalysisRunStatus struct {
	Conditions      conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
	Phase           AnalysisPhase                  `json:"phase,omitempty"`
	AnalysisResults []AnalysisResult               `json:"analysisResults,omitempty"`
}

// AnalysisResult represents an analysis result
type AnalysisResult struct {
	Name         string               `json:"name"`
	Phase        AnalysisPhase        `json:"phase"`
	Score        int32                `json:"score"`
	StartTime    metav1.Time          `json:"startTime"`
	FinishTime   *metav1.Time         `json:"finishTime,omitempty"`
	Value        string               `json:"value,omitempty"`
	Expected     string               `json:"expected,omitempty"`
	Message      string               `json:"message,omitempty"`
	Measurements []AnalysisMeasurement `json:"measurements,omitempty"`
}

// AnalysisMeasurement represents a single analysis measurement
type AnalysisMeasurement struct {
	Phase      AnalysisPhase         `json:"phase"`
	Value      string                `json:"value"`
	StartedAt  metav1.Time           `json:"startedAt"`
	FinishedAt *metav1.Time          `json:"finishedAt,omitempty"`
	Message    string                `json:"message,omitempty"`
	Metadata   map[string]string     `json:"metadata,omitempty"`
}

// AnalysisPhase represents the current phase of analysis
type AnalysisPhase string

const (
	// AnalysisPhasePending indicates analysis is pending
	AnalysisPhasePending AnalysisPhase = "Pending"

	// AnalysisPhaseRunning indicates analysis is running
	AnalysisPhaseRunning AnalysisPhase = "Running"

	// AnalysisPhaseCompleted indicates analysis is completed
	AnalysisPhaseCompleted AnalysisPhase = "Completed"

	// AnalysisPhaseFailed indicates analysis failed
	AnalysisPhaseFailed AnalysisPhase = "Failed"

	// AnalysisPhaseInconclusive indicates analysis is inconclusive
	AnalysisPhaseInconclusive AnalysisPhase = "Inconclusive"

	// AnalysisPhaseStopped indicates analysis was stopped
	AnalysisPhaseStopped AnalysisPhase = "Stopped"
)