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
	"errors"
	"fmt"

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

// Validate checks if the deployment strategy configuration is valid
func (ds *DeploymentStrategy) Validate() error {
	if ds.Type == "" {
		return errors.New("strategy type is required")
	}

	switch ds.Type {
	case CanaryStrategyType:
		if ds.Canary == nil {
			return errors.New("canary configuration required for canary strategy")
		}
		return ds.Canary.Validate()
	case BlueGreenStrategyType:
		if ds.BlueGreen == nil {
			return errors.New("blue-green configuration required for blue-green strategy")
		}
		return ds.BlueGreen.Validate()
	case RollingUpdateStrategyType:
		if ds.RollingUpdate == nil {
			return errors.New("rolling update configuration required for rolling update strategy")
		}
		return ds.RollingUpdate.Validate()
	case RecreateStrategyType:
		// Recreate strategy requires no additional configuration
		return nil
	default:
		return fmt.Errorf("unsupported strategy type: %s", ds.Type)
	}
}

// Validate checks if the canary strategy configuration is valid
func (cs *CanaryStrategy) Validate() error {
	if len(cs.Steps) == 0 {
		return errors.New("canary strategy must have at least one step")
	}

	totalWeight := int32(0)
	for i, step := range cs.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("canary step %d validation failed: %w", i, err)
		}
		totalWeight = step.Weight
	}

	// The last step should reach 100% traffic
	if totalWeight != 100 {
		return fmt.Errorf("final canary step must reach 100%% traffic, got %d%%", totalWeight)
	}

	if cs.Analysis != nil {
		if err := cs.Analysis.Validate(); err != nil {
			return fmt.Errorf("canary analysis validation failed: %w", err)
		}
	}

	if cs.TrafficRouting != nil {
		if err := cs.TrafficRouting.Validate(); err != nil {
			return fmt.Errorf("traffic routing validation failed: %w", err)
		}
	}

	return nil
}

// Validate checks if the canary step configuration is valid
func (cs *CanaryStep) Validate() error {
	if cs.Weight < 0 || cs.Weight > 100 {
		return fmt.Errorf("canary step weight must be between 0 and 100, got %d", cs.Weight)
	}

	if cs.Replicas != nil && *cs.Replicas < 0 {
		return fmt.Errorf("canary step replicas must be non-negative, got %d", *cs.Replicas)
	}

	return nil
}

// Validate checks if the blue-green strategy configuration is valid
func (bgs *BlueGreenStrategy) Validate() error {
	if bgs.PrePromotionAnalysis != nil {
		if err := bgs.PrePromotionAnalysis.Validate(); err != nil {
			return fmt.Errorf("pre-promotion analysis validation failed: %w", err)
		}
	}

	if bgs.PostPromotionAnalysis != nil {
		if err := bgs.PostPromotionAnalysis.Validate(); err != nil {
			return fmt.Errorf("post-promotion analysis validation failed: %w", err)
		}
	}

	return nil
}

// Validate checks if the rolling update strategy configuration is valid
func (rus *RollingUpdateStrategy) Validate() error {
	if rus.MaxSurge != nil {
		if err := validateIntOrString(*rus.MaxSurge, "maxSurge"); err != nil {
			return err
		}
	}

	if rus.MaxUnavailable != nil {
		if err := validateIntOrString(*rus.MaxUnavailable, "maxUnavailable"); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks if the analysis configuration is valid
func (ac *AnalysisConfig) Validate() error {
	if len(ac.Metrics) == 0 {
		return errors.New("analysis configuration must have at least one metric")
	}

	for i, metric := range ac.Metrics {
		if err := metric.Validate(); err != nil {
			return fmt.Errorf("metric %d validation failed: %w", i, err)
		}
	}

	if ac.Interval.Duration <= 0 {
		return errors.New("analysis interval must be positive")
	}

	// Basic CEL expression validation (simplified)
	if ac.SuccessCondition != "" {
		if err := validateCELExpression(ac.SuccessCondition); err != nil {
			return fmt.Errorf("invalid success condition: %w", err)
		}
	}

	return nil
}

// Validate checks if the metric configuration is valid
func (mc *MetricConfig) Validate() error {
	if mc.Name == "" {
		return errors.New("metric name is required")
	}

	if mc.Threshold < 0 {
		return errors.New("metric threshold must be non-negative")
	}

	return nil
}

// Validate checks if the traffic routing configuration is valid
func (tr *TrafficRouting) Validate() error {
	configCount := 0

	if tr.Istio != nil {
		configCount++
		if err := tr.Istio.Validate(); err != nil {
			return fmt.Errorf("istio traffic routing validation failed: %w", err)
		}
	}

	if tr.Nginx != nil {
		configCount++
		if err := tr.Nginx.Validate(); err != nil {
			return fmt.Errorf("nginx traffic routing validation failed: %w", err)
		}
	}

	if configCount == 0 {
		return errors.New("traffic routing must specify at least one provider (istio, nginx)")
	}

	if configCount > 1 {
		return errors.New("traffic routing can only specify one provider at a time")
	}

	return nil
}

// Validate checks if the Istio traffic routing configuration is valid
func (itr *IstioTrafficRouting) Validate() error {
	if itr.VirtualService == "" {
		return errors.New("istio virtual service name is required")
	}
	return nil
}

// Validate checks if the Nginx traffic routing configuration is valid
func (ntr *NginxTrafficRouting) Validate() error {
	if ntr.Ingress == "" {
		return errors.New("nginx ingress name is required")
	}
	return nil
}

// Validate checks if the health check configuration is valid
func (hc *HealthCheckConfig) Validate() error {
	if hc.InitialDelay.Duration < 0 {
		return errors.New("health check initial delay must be non-negative")
	}

	if hc.Interval.Duration <= 0 {
		return errors.New("health check interval must be positive")
	}

	if hc.Timeout.Duration <= 0 {
		return errors.New("health check timeout must be positive")
	}

	if hc.SuccessThreshold < 1 {
		return errors.New("health check success threshold must be at least 1")
	}

	if hc.FailureThreshold < 1 {
		return errors.New("health check failure threshold must be at least 1")
	}

	return nil
}

// Validate checks if the deployment plan is valid
func (dp *DeploymentPlan) Validate() error {
	if err := dp.Strategy.Validate(); err != nil {
		return fmt.Errorf("strategy validation failed: %w", err)
	}

	if len(dp.Phases) == 0 {
		return errors.New("deployment plan must have at least one phase")
	}

	for i, phase := range dp.Phases {
		if err := phase.Validate(); err != nil {
			return fmt.Errorf("phase %d validation failed: %w", i, err)
		}
	}

	for i, dependency := range dp.Dependencies {
		if err := dependency.Validate(); err != nil {
			return fmt.Errorf("dependency %d validation failed: %w", i, err)
		}
	}

	return nil
}

// Validate checks if the deployment phase is valid
func (dp *DeploymentPhase) Validate() error {
	if dp.Name == "" {
		return errors.New("deployment phase name is required")
	}

	if len(dp.Actions) == 0 {
		return errors.New("deployment phase must have at least one action")
	}

	for i, action := range dp.Actions {
		if err := action.Validate(); err != nil {
			return fmt.Errorf("action %d validation failed: %w", i, err)
		}
	}

	return nil
}

// Validate checks if the deployment action is valid
func (da *DeploymentAction) Validate() error {
	if da.Type == "" {
		return errors.New("deployment action type is required")
	}

	if da.Target == "" {
		return errors.New("deployment action target is required")
	}

	// Validate action type
	switch da.Type {
	case ScaleAction, UpdateAction, WaitAction, VerifyAction:
		// Valid action types
	default:
		return fmt.Errorf("unsupported action type: %s", da.Type)
	}

	return nil
}

// Helper functions

func validateIntOrString(value intstr.IntOrString, fieldName string) error {
	switch value.Type {
	case intstr.Int:
		if value.IntVal < 0 {
			return fmt.Errorf("%s integer value must be non-negative, got %d", fieldName, value.IntVal)
		}
	case intstr.String:
		if value.StrVal == "" {
			return fmt.Errorf("%s string value cannot be empty", fieldName)
		}
		// Basic percentage validation
		if len(value.StrVal) > 0 && value.StrVal[len(value.StrVal)-1] == '%' {
			// This is a percentage - more detailed validation would require parsing
			if len(value.StrVal) == 1 {
				return fmt.Errorf("%s percentage value cannot be just '%%'", fieldName)
			}
		}
	default:
		return fmt.Errorf("%s has invalid type", fieldName)
	}
	return nil
}

// validateCELExpression performs basic validation of CEL expressions
// In a real implementation, this would use the CEL library to compile and validate
func validateCELExpression(expr string) error {
	if expr == "" {
		return errors.New("CEL expression cannot be empty")
	}

	// Basic syntax checks - in production, use actual CEL compilation
	if len(expr) < 3 {
		return errors.New("CEL expression is too short to be valid")
	}

	// Check for balanced parentheses
	parenCount := 0
	for _, char := range expr {
		if char == '(' {
			parenCount++
		} else if char == ')' {
			parenCount--
			if parenCount < 0 {
				return errors.New("unmatched closing parenthesis in CEL expression")
			}
		}
	}

	if parenCount != 0 {
		return errors.New("unmatched opening parenthesis in CEL expression")
	}

	return nil
}