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

package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// DeploymentStrategy defines how a deployment should be executed
type DeploymentStrategy struct {
	// Type of deployment strategy
	Type StrategyType `json:"type"`

	// Canary configuration
	Canary *CanaryStrategy `json:"canary,omitempty"`

	// BlueGreen configuration
	BlueGreen *BlueGreenStrategy `json:"blueGreen,omitempty"`

	// Rolling update configuration
	RollingUpdate *RollingUpdateStrategy `json:"rollingUpdate,omitempty"`

	// HealthCheck defines health validation
	HealthCheck *HealthCheckConfig `json:"healthCheck,omitempty"`
}

// StrategyType defines the deployment strategy type
type StrategyType string

const (
	CanaryStrategyType        StrategyType = "Canary"
	BlueGreenStrategyType     StrategyType = "BlueGreen"
	RollingUpdateStrategyType StrategyType = "RollingUpdate"
	RecreateStrategyType      StrategyType = "Recreate"
)

// CanaryStrategy defines progressive rollout configuration
type CanaryStrategy struct {
	// Steps define the canary progression
	Steps []CanaryStep `json:"steps"`

	// Analysis configuration for automated promotion
	Analysis *AnalysisConfig `json:"analysis,omitempty"`

	// TrafficRouting configuration
	TrafficRouting *TrafficRouting `json:"trafficRouting,omitempty"`
}

// CanaryStep represents a stage in canary deployment
type CanaryStep struct {
	// Weight is the percentage of traffic
	Weight int32 `json:"weight"`

	// Pause duration before auto-promotion
	Pause *metav1.Duration `json:"pause,omitempty"`

	// Replicas override for this step
	Replicas *int32 `json:"replicas,omitempty"`
}

// AnalysisConfig defines metrics-based promotion
type AnalysisConfig struct {
	// Metrics to evaluate
	Metrics []MetricConfig `json:"metrics"`

	// Interval between analysis runs
	Interval metav1.Duration `json:"interval"`

	// SuccessCondition as a CEL expression
	SuccessCondition string `json:"successCondition,omitempty"`
}

// MetricConfig defines a metric to track
type MetricConfig struct {
	Name      string  `json:"name"`
	Threshold float64 `json:"threshold"`
	Query     string  `json:"query,omitempty"`
}

// TrafficRouting defines traffic management
type TrafficRouting struct {
	// Istio configuration
	Istio *IstioTrafficRouting `json:"istio,omitempty"`

	// SMI configuration
	SMI *SMITrafficRouting `json:"smi,omitempty"`
}

// IstioTrafficRouting for Istio-based traffic management
type IstioTrafficRouting struct {
	VirtualService string `json:"virtualService"`
	DestinationRule string `json:"destinationRule,omitempty"`
}

// SMITrafficRouting for SMI-based traffic management
type SMITrafficRouting struct {
	TrafficSplitName string `json:"trafficSplitName"`
}

// BlueGreenStrategy defines blue-green deployment
type BlueGreenStrategy struct {
	// ActiveService name
	ActiveService string `json:"activeService"`

	// PreviewService name
	PreviewService string `json:"previewService"`

	// AutoPromotionEnabled allows automatic promotion
	AutoPromotionEnabled bool `json:"autoPromotionEnabled"`

	// AutoPromotionSeconds before automatic promotion
	AutoPromotionSeconds int32 `json:"autoPromotionSeconds,omitempty"`

	// ScaleDownDelaySeconds before scaling down old version
	ScaleDownDelaySeconds int32 `json:"scaleDownDelaySeconds,omitempty"`

	// PrePromotionAnalysis to run before promotion
	PrePromotionAnalysis *AnalysisConfig `json:"prePromotionAnalysis,omitempty"`

	// PostPromotionAnalysis to run after promotion
	PostPromotionAnalysis *AnalysisConfig `json:"postPromotionAnalysis,omitempty"`
}

// RollingUpdateStrategy defines rolling update configuration
type RollingUpdateStrategy struct {
	// MaxUnavailable during update
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// MaxSurge during update
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`
}

// HealthCheckConfig defines health validation during deployment
type HealthCheckConfig struct {
	// InitialDelaySeconds before first check
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`

	// PeriodSeconds between checks
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// TimeoutSeconds for each check
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// SuccessThreshold required for healthy status
	SuccessThreshold int32 `json:"successThreshold,omitempty"`

	// FailureThreshold before considering unhealthy
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// HTTPGet probe configuration
	HTTPGet *HTTPGetAction `json:"httpGet,omitempty"`

	// TCPSocket probe configuration
	TCPSocket *TCPSocketAction `json:"tcpSocket,omitempty"`

	// Exec probe configuration
	Exec *ExecAction `json:"exec,omitempty"`
}

// HTTPGetAction describes an HTTP GET probe
type HTTPGetAction struct {
	Path   string `json:"path"`
	Port   int32  `json:"port"`
	Scheme string `json:"scheme,omitempty"`
}

// TCPSocketAction describes a TCP socket probe
type TCPSocketAction struct {
	Port int32 `json:"port"`
}

// ExecAction describes a command-based probe
type ExecAction struct {
	Command []string `json:"command"`
}

// DeploymentResult represents the outcome of a deployment
type DeploymentResult struct {
	// Status of the deployment
	Status DeploymentStatus `json:"status"`

	// Message with details
	Message string `json:"message,omitempty"`

	// Metrics collected during deployment
	Metrics map[string]float64 `json:"metrics,omitempty"`

	// StartTime of deployment
	StartTime metav1.Time `json:"startTime"`

	// EndTime of deployment
	EndTime *metav1.Time `json:"endTime,omitempty"`
}

// DeploymentStatus represents deployment state
type DeploymentStatus string

const (
	DeploymentPending    DeploymentStatus = "Pending"
	DeploymentProgressing DeploymentStatus = "Progressing"
	DeploymentSucceeded   DeploymentStatus = "Succeeded"
	DeploymentFailed      DeploymentStatus = "Failed"
	DeploymentPaused      DeploymentStatus = "Paused"
	DeploymentAborted     DeploymentStatus = "Aborted"
)