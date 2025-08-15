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

	// Nginx configuration
	Nginx *NginxTrafficRouting `json:"nginx,omitempty"`
}

// IstioTrafficRouting defines Istio-specific traffic routing
type IstioTrafficRouting struct {
	VirtualService string `json:"virtualService"`
	DestinationRule string `json:"destinationRule,omitempty"`
}

// NginxTrafficRouting defines Nginx-specific traffic routing  
type NginxTrafficRouting struct {
	Ingress string `json:"ingress"`
	ConfigMap string `json:"configMap,omitempty"`
}

// BlueGreenStrategy defines blue-green deployment
type BlueGreenStrategy struct {
	// PrePromotionAnalysis runs before switching
	PrePromotionAnalysis *AnalysisConfig `json:"prePromotionAnalysis,omitempty"`

	// PostPromotionAnalysis runs after switching
	PostPromotionAnalysis *AnalysisConfig `json:"postPromotionAnalysis,omitempty"`

	// AutoPromotionEnabled enables automatic promotion
	AutoPromotionEnabled bool `json:"autoPromotionEnabled"`

	// ScaleDownDelay before removing old version
	ScaleDownDelay *metav1.Duration `json:"scaleDownDelay,omitempty"`
}

// RollingUpdateStrategy defines rolling update parameters
type RollingUpdateStrategy struct {
	MaxSurge       *intstr.IntOrString `json:"maxSurge,omitempty"`
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// HealthCheckConfig defines health validation
type HealthCheckConfig struct {
	InitialDelay     metav1.Duration `json:"initialDelay"`
	Interval         metav1.Duration `json:"interval"`
	Timeout          metav1.Duration `json:"timeout"`
	SuccessThreshold int32           `json:"successThreshold"`
	FailureThreshold int32           `json:"failureThreshold"`
}

// DeploymentPlan represents an execution plan
type DeploymentPlan struct {
	Strategy     DeploymentStrategy `json:"strategy"`
	Phases       []DeploymentPhase  `json:"phases"`
	Dependencies []Dependency       `json:"dependencies,omitempty"`
}

// DeploymentPhase is a stage in deployment
type DeploymentPhase struct {
	Name      string             `json:"name"`
	Actions   []DeploymentAction `json:"actions"`
	Condition string             `json:"condition,omitempty"`
}

// DeploymentAction is an atomic deployment operation
type DeploymentAction struct {
	Type   ActionType              `json:"type"`
	Target string                  `json:"target"`
	Config map[string]interface{} `json:"config,omitempty"`
}

type ActionType string

const (
	ScaleAction  ActionType = "Scale"
	UpdateAction ActionType = "Update"
	WaitAction   ActionType = "Wait"
	VerifyAction ActionType = "Verify"
)