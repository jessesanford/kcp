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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

// WorkloadAnalysisRun represents a comprehensive analysis run for workload placement validation.
//
// +crd
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type WorkloadAnalysisRun struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec WorkloadAnalysisRunSpec `json:"spec,omitempty"`

	// +optional
	Status WorkloadAnalysisRunStatus `json:"status,omitempty"`
}

// WorkloadAnalysisRunSpec defines the desired state of WorkloadAnalysisRun.
type WorkloadAnalysisRunSpec struct {
	// WorkloadSelector selects the workloads this analysis applies to
	WorkloadSelector WorkloadSelector `json:"workloadSelector"`

	// ClusterSelector defines which clusters to analyze
	ClusterSelector ClusterSelector `json:"clusterSelector"`

	// AnalysisTemplates defines the analysis templates to run
	AnalysisTemplates []AnalysisTemplate `json:"analysisTemplates"`

	// AnalysisSchedule defines when to run analysis
	// +optional
	AnalysisSchedule *AnalysisSchedule `json:"analysisSchedule,omitempty"`

	// FailureThreshold defines when to consider analysis failed
	// +kubebuilder:default=3
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// SuccessThreshold defines successful analysis criteria
	// +kubebuilder:default=1
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`

	// Timeout defines overall analysis timeout
	// +kubebuilder:default="300s"
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`
}

// AnalysisTemplate defines a single analysis template
type AnalysisTemplate struct {
	// Name is the analysis template name
	Name string `json:"name"`

	// AnalysisType defines the type of analysis
	// +kubebuilder:validation:Enum=Prometheus;DataDog;Custom;SLO;Canary;NewRelic;Grafana
	AnalysisType AnalysisType `json:"analysisType"`

	// Query defines the analysis query
	Query string `json:"query"`

	// Interval defines how often to run the analysis
	// +kubebuilder:default="30s"
	// +optional
	Interval metav1.Duration `json:"interval,omitempty"`

	// Timeout defines analysis timeout
	// +kubebuilder:default="60s"
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`

	// SuccessCriteria defines what constitutes success
	SuccessCriteria SuccessCriteria `json:"successCriteria"`

	// Weight defines analysis importance (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=50
	// +optional
	Weight int32 `json:"weight,omitempty"`

	// Provider defines provider-specific configuration
	// +optional
	Provider *AnalysisProvider `json:"provider,omitempty"`
}

// AnalysisType represents the type of analysis
// +kubebuilder:validation:Enum=Prometheus;DataDog;Custom;SLO;Canary;NewRelic;Grafana
type AnalysisType string

const (
	// AnalysisTypePrometheus uses Prometheus metrics
	AnalysisTypePrometheus AnalysisType = "Prometheus"

	// AnalysisTypeDataDog uses DataDog metrics
	AnalysisTypeDataDog AnalysisType = "DataDog"

	// AnalysisTypeCustom uses custom analysis
	AnalysisTypeCustom AnalysisType = "Custom"

	// AnalysisTypeSLO uses SLO-based analysis
	AnalysisTypeSLO AnalysisType = "SLO"

	// AnalysisTypeCanary uses canary analysis
	AnalysisTypeCanary AnalysisType = "Canary"

	// AnalysisTypeNewRelic uses New Relic metrics
	AnalysisTypeNewRelic AnalysisType = "NewRelic"

	// AnalysisTypeGrafana uses Grafana metrics
	AnalysisTypeGrafana AnalysisType = "Grafana"
)

// SuccessCriteria defines success criteria for analysis
type SuccessCriteria struct {
	// Threshold defines the success threshold
	Threshold string `json:"threshold"`

	// Operator defines the comparison operator
	// +kubebuilder:validation:Enum=GreaterThan;LessThan;Equal;GreaterThanOrEqual;LessThanOrEqual;NotEqual
	Operator ComparisonOperator `json:"operator"`

	// Unit defines the measurement unit
	// +optional
	Unit string `json:"unit,omitempty"`

	// TolerancePercentage allows for tolerance in measurements
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	TolerancePercentage *int32 `json:"tolerancePercentage,omitempty"`
}

// ComparisonOperator represents comparison operators
// +kubebuilder:validation:Enum=GreaterThan;LessThan;Equal;GreaterThanOrEqual;LessThanOrEqual;NotEqual
type ComparisonOperator string

const (
	// ComparisonOperatorGreaterThan represents >
	ComparisonOperatorGreaterThan ComparisonOperator = "GreaterThan"

	// ComparisonOperatorLessThan represents <
	ComparisonOperatorLessThan ComparisonOperator = "LessThan"

	// ComparisonOperatorEqual represents =
	ComparisonOperatorEqual ComparisonOperator = "Equal"

	// ComparisonOperatorGreaterThanOrEqual represents >=
	ComparisonOperatorGreaterThanOrEqual ComparisonOperator = "GreaterThanOrEqual"

	// ComparisonOperatorLessThanOrEqual represents <=
	ComparisonOperatorLessThanOrEqual ComparisonOperator = "LessThanOrEqual"

	// ComparisonOperatorNotEqual represents !=
	ComparisonOperatorNotEqual ComparisonOperator = "NotEqual"
)

// AnalysisProvider defines provider-specific configuration
type AnalysisProvider struct {
	// Prometheus defines Prometheus-specific configuration
	// +optional
	Prometheus *PrometheusProvider `json:"prometheus,omitempty"`

	// DataDog defines DataDog-specific configuration
	// +optional
	DataDog *DataDogProvider `json:"dataDog,omitempty"`

	// NewRelic defines New Relic-specific configuration
	// +optional
	NewRelic *NewRelicProvider `json:"newRelic,omitempty"`

	// Grafana defines Grafana-specific configuration
	// +optional
	Grafana *GrafanaProvider `json:"grafana,omitempty"`

	// Custom defines custom provider configuration
	// +optional
	Custom *CustomProvider `json:"custom,omitempty"`
}

// PrometheusProvider defines Prometheus-specific configuration
type PrometheusProvider struct {
	// Address is the Prometheus server address
	Address string `json:"address"`

	// Credentials defines authentication credentials
	// +optional
	Credentials *ProviderCredentials `json:"credentials,omitempty"`

	// Headers defines additional HTTP headers
	// +optional
	Headers map[string]string `json:"headers,omitempty"`
}

// DataDogProvider defines DataDog-specific configuration
type DataDogProvider struct {
	// APIKey is the DataDog API key (reference)
	APIKey string `json:"apiKey"`

	// AppKey is the DataDog application key (reference)
	AppKey string `json:"appKey"`

	// Site defines the DataDog site
	// +kubebuilder:default="datadoghq.com"
	// +optional
	Site string `json:"site,omitempty"`
}

// NewRelicProvider defines New Relic-specific configuration
type NewRelicProvider struct {
	// APIKey is the New Relic API key (reference)
	APIKey string `json:"apiKey"`

	// AccountID is the New Relic account ID
	AccountID string `json:"accountId"`

	// Region defines the New Relic region
	// +kubebuilder:validation:Enum=US;EU
	// +kubebuilder:default="US"
	// +optional
	Region string `json:"region,omitempty"`
}

// GrafanaProvider defines Grafana-specific configuration
type GrafanaProvider struct {
	// Address is the Grafana server address
	Address string `json:"address"`

	// Credentials defines authentication credentials
	// +optional
	Credentials *ProviderCredentials `json:"credentials,omitempty"`

	// OrgID defines the Grafana organization ID
	// +optional
	OrgID *int64 `json:"orgId,omitempty"`
}

// CustomProvider defines custom provider configuration
type CustomProvider struct {
	// Endpoint is the custom provider endpoint
	Endpoint string `json:"endpoint"`

	// Method defines the HTTP method
	// +kubebuilder:validation:Enum=GET;POST;PUT;PATCH
	// +kubebuilder:default="GET"
	// +optional
	Method string `json:"method,omitempty"`

	// Headers defines HTTP headers
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// Body defines the request body template
	// +optional
	Body string `json:"body,omitempty"`

	// Credentials defines authentication credentials
	// +optional
	Credentials *ProviderCredentials `json:"credentials,omitempty"`
}

// ProviderCredentials defines authentication credentials
type ProviderCredentials struct {
	// SecretRef references a secret containing credentials
	// +optional
	SecretRef *CredentialSecretRef `json:"secretRef,omitempty"`

	// BasicAuth defines basic authentication
	// +optional
	BasicAuth *BasicAuthCredentials `json:"basicAuth,omitempty"`

	// BearerToken defines bearer token authentication
	// +optional
	BearerToken *BearerTokenCredentials `json:"bearerToken,omitempty"`
}

// CredentialSecretRef references a secret
type CredentialSecretRef struct {
	// Name is the secret name
	Name string `json:"name"`

	// Namespace is the secret namespace
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Keys defines which keys to use from the secret
	// +optional
	Keys map[string]string `json:"keys,omitempty"`
}

// BasicAuthCredentials defines basic authentication
type BasicAuthCredentials struct {
	// Username is the username
	Username string `json:"username"`

	// PasswordRef references the password
	PasswordRef CredentialSecretRef `json:"passwordRef"`
}

// BearerTokenCredentials defines bearer token authentication
type BearerTokenCredentials struct {
	// TokenRef references the bearer token
	TokenRef CredentialSecretRef `json:"tokenRef"`
}

// AnalysisSchedule defines when to run analysis
type AnalysisSchedule struct {
	// Type defines the schedule type
	// +kubebuilder:validation:Enum=Immediate;Scheduled;Event;Continuous
	// +kubebuilder:default="Immediate"
	// +optional
	Type AnalysisScheduleType `json:"type,omitempty"`

	// CronSchedule defines cron-based scheduling
	// +optional
	CronSchedule *CronSchedule `json:"cronSchedule,omitempty"`

	// EventTrigger defines event-based triggers
	// +optional
	EventTrigger *EventTrigger `json:"eventTrigger,omitempty"`

	// ContinuousSchedule defines continuous analysis
	// +optional
	ContinuousSchedule *ContinuousSchedule `json:"continuousSchedule,omitempty"`
}

// AnalysisScheduleType represents schedule types
// +kubebuilder:validation:Enum=Immediate;Scheduled;Event;Continuous
type AnalysisScheduleType string

const (
	// AnalysisScheduleTypeImmediate runs analysis immediately
	AnalysisScheduleTypeImmediate AnalysisScheduleType = "Immediate"

	// AnalysisScheduleTypeScheduled runs analysis on schedule
	AnalysisScheduleTypeScheduled AnalysisScheduleType = "Scheduled"

	// AnalysisScheduleTypeEvent runs analysis on events
	AnalysisScheduleTypeEvent AnalysisScheduleType = "Event"

	// AnalysisScheduleTypeContinuous runs analysis continuously
	AnalysisScheduleTypeContinuous AnalysisScheduleType = "Continuous"
)

// CronSchedule defines cron-based scheduling
type CronSchedule struct {
	// Schedule is the cron expression
	Schedule string `json:"schedule"`

	// TimeZone defines the timezone
	// +kubebuilder:default="UTC"
	// +optional
	TimeZone string `json:"timeZone,omitempty"`

	// Suspend indicates whether to suspend scheduling
	// +kubebuilder:default=false
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// EventTrigger defines event-based triggers
type EventTrigger struct {
	// Events defines which events trigger analysis
	Events []AnalysisEvent `json:"events"`

	// Debounce defines debounce duration
	// +kubebuilder:default="30s"
	// +optional
	Debounce metav1.Duration `json:"debounce,omitempty"`
}

// AnalysisEvent represents events that trigger analysis
type AnalysisEvent struct {
	// Type defines the event type
	// +kubebuilder:validation:Enum=Deployment;ScaleUp;ScaleDown;ConfigChange;HealthChange
	Type AnalysisEventType `json:"type"`

	// Selector defines event source selector
	// +optional
	Selector map[string]string `json:"selector,omitempty"`
}

// AnalysisEventType represents analysis event types
// +kubebuilder:validation:Enum=Deployment;ScaleUp;ScaleDown;ConfigChange;HealthChange
type AnalysisEventType string

const (
	// AnalysisEventTypeDeployment triggered by deployments
	AnalysisEventTypeDeployment AnalysisEventType = "Deployment"

	// AnalysisEventTypeScaleUp triggered by scale up events
	AnalysisEventTypeScaleUp AnalysisEventType = "ScaleUp"

	// AnalysisEventTypeScaleDown triggered by scale down events
	AnalysisEventTypeScaleDown AnalysisEventType = "ScaleDown"

	// AnalysisEventTypeConfigChange triggered by config changes
	AnalysisEventTypeConfigChange AnalysisEventType = "ConfigChange"

	// AnalysisEventTypeHealthChange triggered by health changes
	AnalysisEventTypeHealthChange AnalysisEventType = "HealthChange"
)

// ContinuousSchedule defines continuous analysis
type ContinuousSchedule struct {
	// Interval defines the analysis interval
	// +kubebuilder:default="300s"
	// +optional
	Interval metav1.Duration `json:"interval,omitempty"`

	// MaxRuns defines maximum number of runs
	// +optional
	MaxRuns *int32 `json:"maxRuns,omitempty"`
}

// WorkloadAnalysisRunStatus represents the observed state of WorkloadAnalysisRun
type WorkloadAnalysisRunStatus struct {
	// Conditions represent the latest available observations of the analysis state
	// +optional
	Conditions conditionsv1alpha1.Conditions `json:"conditions,omitempty"`

	// Phase indicates the current analysis phase
	// +kubebuilder:default="Pending"
	// +optional
	Phase AnalysisPhase `json:"phase,omitempty"`

	// StartTime is when analysis started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// FinishTime is when analysis finished
	// +optional
	FinishTime *metav1.Time `json:"finishTime,omitempty"`

	// RunCount is the number of analysis runs performed
	RunCount int32 `json:"runCount"`

	// SuccessfulRuns is the number of successful runs
	SuccessfulRuns int32 `json:"successfulRuns"`

	// FailedRuns is the number of failed runs
	FailedRuns int32 `json:"failedRuns"`

	// OverallScore represents the overall analysis score (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	OverallScore *int32 `json:"overallScore,omitempty"`

	// AnalysisResults contains the results of analysis templates
	// +optional
	AnalysisResults []AnalysisResult `json:"analysisResults,omitempty"`

	// Message provides additional information about the analysis state
	// +optional
	Message string `json:"message,omitempty"`
}

// AnalysisPhase represents the current phase of analysis
// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed;Inconclusive;Stopped
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

// AnalysisResult represents an analysis result
type AnalysisResult struct {
	// Name is the analysis template name
	Name string `json:"name"`

	// Phase is the analysis phase
	Phase AnalysisPhase `json:"phase"`

	// Score is the analysis score (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Score int32 `json:"score"`

	// StartTime is when analysis started
	StartTime metav1.Time `json:"startTime"`

	// FinishTime is when analysis finished
	// +optional
	FinishTime *metav1.Time `json:"finishTime,omitempty"`

	// Value is the measured value
	// +optional
	Value string `json:"value,omitempty"`

	// Expected is the expected value
	// +optional
	Expected string `json:"expected,omitempty"`

	// Message provides additional information
	// +optional
	Message string `json:"message,omitempty"`

	// Measurements contains detailed measurements
	// +optional
	Measurements []AnalysisMeasurement `json:"measurements,omitempty"`
}

// AnalysisMeasurement represents a single analysis measurement
type AnalysisMeasurement struct {
	// Phase is the measurement phase
	Phase AnalysisPhase `json:"phase"`

	// Value is the measured value
	Value string `json:"value"`

	// StartedAt is when measurement started
	StartedAt metav1.Time `json:"startedAt"`

	// FinishedAt is when measurement finished
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`

	// Message provides additional information
	// +optional
	Message string `json:"message,omitempty"`

	// Metadata contains measurement metadata
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// WorkloadAnalysisRunList is a list of WorkloadAnalysisRun resources
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type WorkloadAnalysisRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []WorkloadAnalysisRun `json:"items"`
}