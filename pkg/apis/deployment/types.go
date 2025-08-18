/*
Copyright 2023 The KCP Authors.

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

package deployment

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// CanaryDeployment represents a canary deployment configuration for gradual
// rollout of application changes with automated analysis and traffic management.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CanaryDeployment struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Spec defines the desired state of the CanaryDeployment.
	Spec CanaryDeploymentSpec

	// Status defines the observed state of the CanaryDeployment.
	Status CanaryDeploymentStatus
}

// CanaryDeploymentSpec defines the desired state of a canary deployment.
type CanaryDeploymentSpec struct {
	// TargetRef references the deployment to be updated with canary logic.
	TargetRef corev1.ObjectReference

	// CanaryVersion specifies the version identifier for the canary release.
	CanaryVersion string

	// StableVersion specifies the version identifier for the stable release.
	StableVersion string

	// TrafficPercentage specifies the current percentage of traffic directed to canary (0-100).
	TrafficPercentage int

	// Strategy defines the rollout strategy for the canary deployment.
	Strategy CanaryStrategy

	// Analysis defines the metrics analysis configuration for canary evaluation.
	Analysis CanaryAnalysis

	// ProgressDeadlineSeconds specifies the maximum time in seconds for the canary
	// to make progress before it is considered to be failed.
	ProgressDeadlineSeconds *int32
}

// CanaryStrategy defines the rollout strategy for canary deployments.
type CanaryStrategy struct {
	// Steps defines the sequence of traffic percentages for gradual rollout.
	Steps []int

	// StepDuration defines how long to wait at each step before proceeding.
	StepDuration *metav1.Duration

	// AutoPromotion determines if the canary should automatically promote.
	AutoPromotion *bool

	// MaxUnavailable specifies the maximum number of pods that can be unavailable.
	MaxUnavailable *intstr.IntOrString
}

// CanaryAnalysis defines the metrics analysis configuration.
type CanaryAnalysis struct {
	// Interval specifies how often to perform the analysis.
	Interval *metav1.Duration

	// Threshold specifies the success percentage required for promotion (0-100).
	Threshold *int

	// MetricQueries defines the list of metric queries to evaluate.
	MetricQueries []MetricQuery

	// Webhooks defines optional webhook-based analysis checks.
	Webhooks []WebhookCheck
}

// MetricQuery defines a single metric query for analysis.
type MetricQuery struct {
	// Name identifies the metric query.
	Name string

	// Query specifies the metric query string.
	Query string

	// ThresholdType specifies whether the threshold is a maximum or minimum value.
	ThresholdType ThresholdType

	// Threshold specifies the numeric threshold value for this metric.
	Threshold float64

	// Weight specifies the importance of this metric in the overall analysis.
	Weight *int
}

// ThresholdType defines the comparison type for metric thresholds.
type ThresholdType string

const (
	// ThresholdTypeLessThan indicates the metric value should be less than the threshold.
	ThresholdTypeLessThan ThresholdType = "LessThan"

	// ThresholdTypeGreaterThan indicates the metric value should be greater than the threshold.
	ThresholdTypeGreaterThan ThresholdType = "GreaterThan"
)

// WebhookCheck defines a webhook-based analysis check.
type WebhookCheck struct {
	// Name identifies the webhook check.
	Name string

	// URL specifies the webhook endpoint.
	URL string

	// TimeoutSeconds specifies the timeout for the webhook call.
	TimeoutSeconds *int32

	// Headers specifies optional HTTP headers.
	Headers map[string]string
}

// CanaryDeploymentStatus defines the observed state of a canary deployment.
type CanaryDeploymentStatus struct {
	// ObservedGeneration reflects the generation of the most recently observed spec.
	ObservedGeneration int64

	// Phase represents the current phase of the canary deployment.
	Phase CanaryPhase

	// CurrentStep indicates which step in the strategy the canary is executing.
	CurrentStep int

	// StepStartTime indicates when the current step began.
	StepStartTime *metav1.Time

	// LastAnalysisTime indicates when the last analysis was performed.
	LastAnalysisTime *metav1.Time

	// AnalysisResults contains the results of the most recent metric analysis.
	AnalysisResults []AnalysisResult

	// Message provides human-readable information about the current state.
	Message string

	// Conditions represent the current conditions of the CanaryDeployment.
	Conditions conditionsv1alpha1.Conditions
}

// CanaryPhase represents the phase of a canary deployment.
type CanaryPhase string

const (
	// CanaryPhasePending indicates the canary deployment is pending initialization.
	CanaryPhasePending CanaryPhase = "Pending"

	// CanaryPhaseProgressing indicates the canary deployment is progressing through steps.
	CanaryPhaseProgressing CanaryPhase = "Progressing"

	// CanaryPhaseAnalyzing indicates the canary deployment is performing analysis.
	CanaryPhaseAnalyzing CanaryPhase = "Analyzing"

	// CanaryPhasePromoting indicates the canary deployment is promoting to the next step.
	CanaryPhasePromoting CanaryPhase = "Promoting"

	// CanaryPhaseSucceeded indicates the canary deployment completed successfully.
	CanaryPhaseSucceeded CanaryPhase = "Succeeded"

	// CanaryPhaseFailed indicates the canary deployment failed analysis or rollout.
	CanaryPhaseFailed CanaryPhase = "Failed"

	// CanaryPhaseRollingBack indicates the canary deployment is rolling back to stable.
	CanaryPhaseRollingBack CanaryPhase = "RollingBack"
)

// AnalysisResult represents the result of a single metric analysis.
type AnalysisResult struct {
	// MetricName identifies which metric query this result corresponds to.
	MetricName string

	// Value contains the observed metric value.
	Value float64

	// Threshold contains the threshold that was evaluated against.
	Threshold float64

	// Passed indicates whether this metric analysis passed.
	Passed bool

	// Weight represents the weight of this metric in the overall analysis.
	Weight int

	// Timestamp indicates when this analysis was performed.
	Timestamp metav1.Time
}

// CanaryDeploymentList contains a list of CanaryDeployment.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CanaryDeploymentList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []CanaryDeployment
}