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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// CanaryDeployment represents a canary deployment configuration for gradual
// rollout of application changes with automated analysis and traffic management.
//
// +crd
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=tmc
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Traffic",type="integer",JSONPath=".spec.trafficPercentage"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type CanaryDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the CanaryDeployment.
	Spec CanaryDeploymentSpec `json:"spec,omitempty"`

	// Status defines the observed state of the CanaryDeployment.
	Status CanaryDeploymentStatus `json:"status,omitempty"`
}

// CanaryDeploymentSpec defines the desired state of a canary deployment.
type CanaryDeploymentSpec struct {
	// TargetRef references the deployment to be updated with canary logic.
	TargetRef corev1.ObjectReference `json:"targetRef"`

	// CanaryVersion specifies the version identifier for the canary release.
	CanaryVersion string `json:"canaryVersion"`

	// StableVersion specifies the version identifier for the stable release.
	StableVersion string `json:"stableVersion"`

	// TrafficPercentage specifies the current percentage of traffic directed to canary (0-100).
	// This field is managed by the controller based on the rollout strategy.
	TrafficPercentage int `json:"trafficPercentage"`

	// Strategy defines the rollout strategy for the canary deployment.
	Strategy CanaryStrategy `json:"strategy"`

	// Analysis defines the metrics analysis configuration for canary evaluation.
	Analysis CanaryAnalysis `json:"analysis"`

	// ProgressDeadlineSeconds specifies the maximum time in seconds for the canary
	// to make progress before it is considered to be failed. Defaults to 1800 seconds.
	// +optional
	ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty"`
}

// CanaryStrategy defines the rollout strategy for canary deployments.
type CanaryStrategy struct {
	// Steps defines the sequence of traffic percentages for gradual rollout.
	// If not specified, defaults to [10, 25, 50, 100].
	// +optional
	Steps []int `json:"steps,omitempty"`

	// StepDuration defines how long to wait at each step before proceeding.
	// Defaults to 5 minutes.
	// +optional
	StepDuration *metav1.Duration `json:"stepDuration,omitempty"`

	// AutoPromotion determines if the canary should automatically promote
	// to the next step when analysis passes. Defaults to true.
	// +optional
	AutoPromotion *bool `json:"autoPromotion,omitempty"`

	// MaxUnavailable specifies the maximum number of pods that can be unavailable
	// during the canary update. This can be an absolute number or a percentage.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// CanaryAnalysis defines the metrics analysis configuration.
type CanaryAnalysis struct {
	// Interval specifies how often to perform the analysis.
	// Defaults to 1 minute.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Threshold specifies the success percentage required for promotion (0-100).
	// Defaults to 95.
	// +optional
	Threshold *int `json:"threshold,omitempty"`

	// MetricQueries defines the list of metric queries to evaluate.
	MetricQueries []MetricQuery `json:"metricQueries"`

	// Webhooks defines optional webhook-based analysis checks.
	// +optional
	Webhooks []WebhookCheck `json:"webhooks,omitempty"`
}

// MetricQuery defines a single metric query for analysis.
type MetricQuery struct {
	// Name identifies the metric query for reference in logs and status.
	Name string `json:"name"`

	// Query specifies the metric query string (Prometheus PromQL format).
	Query string `json:"query"`

	// ThresholdType specifies whether the threshold is a maximum or minimum value.
	// +kubebuilder:validation:Enum=LessThan;GreaterThan
	ThresholdType ThresholdType `json:"thresholdType"`

	// Threshold specifies the numeric threshold value for this metric.
	Threshold float64 `json:"threshold"`

	// Weight specifies the importance of this metric in the overall analysis (1-100).
	// Defaults to 10.
	// +optional
	Weight *int `json:"weight,omitempty"`
}

// ThresholdType defines the comparison type for metric thresholds.
type ThresholdType string

const (
	// LessThan indicates the metric value should be less than the threshold.
	ThresholdTypeLessThan ThresholdType = "LessThan"

	// GreaterThan indicates the metric value should be greater than the threshold.
	ThresholdTypeGreaterThan ThresholdType = "GreaterThan"
)

// WebhookCheck defines a webhook-based analysis check.
type WebhookCheck struct {
	// Name identifies the webhook check for reference.
	Name string `json:"name"`

	// URL specifies the webhook endpoint to call for analysis.
	URL string `json:"url"`

	// TimeoutSeconds specifies the timeout for the webhook call.
	// Defaults to 30 seconds.
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// Headers specifies optional HTTP headers to include in the request.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`
}

// CanaryDeploymentStatus defines the observed state of a canary deployment.
type CanaryDeploymentStatus struct {
	// ObservedGeneration reflects the generation of the most recently observed spec.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase represents the current phase of the canary deployment.
	// +optional
	Phase CanaryPhase `json:"phase,omitempty"`

	// CurrentStep indicates which step in the strategy the canary is currently executing.
	// +optional
	CurrentStep int `json:"currentStep,omitempty"`

	// StepStartTime indicates when the current step began.
	// +optional
	StepStartTime *metav1.Time `json:"stepStartTime,omitempty"`

	// LastAnalysisTime indicates when the last analysis was performed.
	// +optional
	LastAnalysisTime *metav1.Time `json:"lastAnalysisTime,omitempty"`

	// AnalysisResults contains the results of the most recent metric analysis.
	// +optional
	AnalysisResults []AnalysisResult `json:"analysisResults,omitempty"`

	// Message provides human-readable information about the current state.
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the current conditions of the CanaryDeployment.
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`
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
	MetricName string `json:"metricName"`

	// Value contains the observed metric value.
	Value float64 `json:"value"`

	// Threshold contains the threshold that was evaluated against.
	Threshold float64 `json:"threshold"`

	// Passed indicates whether this metric analysis passed.
	Passed bool `json:"passed"`

	// Weight represents the weight of this metric in the overall analysis.
	Weight int `json:"weight"`

	// Timestamp indicates when this analysis was performed.
	Timestamp metav1.Time `json:"timestamp"`
}

// CanaryDeploymentList contains a list of CanaryDeployment.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CanaryDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CanaryDeployment `json:"items"`
}