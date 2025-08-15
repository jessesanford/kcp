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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestDeploymentStrategyValidation(t *testing.T) {
	tests := []struct {
		name     string
		strategy DeploymentStrategy
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid canary strategy",
			strategy: DeploymentStrategy{
				Type: CanaryStrategyType,
				Canary: &CanaryStrategy{
					Steps: []CanaryStep{
						{Weight: 10},
						{Weight: 100},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid blue-green strategy",
			strategy: DeploymentStrategy{
				Type:      BlueGreenStrategyType,
				BlueGreen: &BlueGreenStrategy{},
			},
			wantErr: false,
		},
		{
			name: "valid recreate strategy",
			strategy: DeploymentStrategy{
				Type: RecreateStrategyType,
			},
			wantErr: false,
		},
		{
			name:    "empty strategy type",
			strategy: DeploymentStrategy{},
			wantErr: true,
			errMsg:  "strategy type is required",
		},
		{
			name: "canary without configuration",
			strategy: DeploymentStrategy{
				Type: CanaryStrategyType,
			},
			wantErr: true,
			errMsg:  "canary configuration required",
		},
		{
			name: "unsupported strategy type",
			strategy: DeploymentStrategy{
				Type: StrategyType("Unknown"),
			},
			wantErr: true,
			errMsg:  "unsupported strategy type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.strategy.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCanaryStrategyValidation(t *testing.T) {
	tests := []struct {
		name     string
		strategy CanaryStrategy
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid canary strategy",
			strategy: CanaryStrategy{
				Steps: []CanaryStep{
					{Weight: 10},
					{Weight: 50},
					{Weight: 100},
				},
			},
			wantErr: false,
		},
		{
			name:     "no steps",
			strategy: CanaryStrategy{},
			wantErr:  true,
			errMsg:   "must have at least one step",
		},
		{
			name: "final step not 100%",
			strategy: CanaryStrategy{
				Steps: []CanaryStep{
					{Weight: 10},
					{Weight: 50},
				},
			},
			wantErr: true,
			errMsg:  "final canary step must reach 100%",
		},
		{
			name: "invalid step weight",
			strategy: CanaryStrategy{
				Steps: []CanaryStep{
					{Weight: -10},
					{Weight: 100},
				},
			},
			wantErr: true,
			errMsg:  "weight must be between 0 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.strategy.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCanaryStepValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    CanaryStep
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid step",
			step:    CanaryStep{Weight: 50},
			wantErr: false,
		},
		{
			name:    "weight too low",
			step:    CanaryStep{Weight: -1},
			wantErr: true,
			errMsg:  "weight must be between 0 and 100",
		},
		{
			name:    "weight too high",
			step:    CanaryStep{Weight: 101},
			wantErr: true,
			errMsg:  "weight must be between 0 and 100",
		},
		{
			name: "negative replicas",
			step: CanaryStep{
				Weight:   50,
				Replicas: func() *int32 { r := int32(-1); return &r }(),
			},
			wantErr: true,
			errMsg:  "replicas must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAnalysisConfigValidation(t *testing.T) {
	tests := []struct {
		name     string
		analysis AnalysisConfig
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid analysis",
			analysis: AnalysisConfig{
				Metrics: []MetricConfig{
					{Name: "success-rate", Threshold: 0.99},
				},
				Interval: metav1.Duration{Duration: 30 * time.Second},
			},
			wantErr: false,
		},
		{
			name: "no metrics",
			analysis: AnalysisConfig{
				Interval: metav1.Duration{Duration: 30 * time.Second},
			},
			wantErr: true,
			errMsg:  "must have at least one metric",
		},
		{
			name: "zero interval",
			analysis: AnalysisConfig{
				Metrics: []MetricConfig{
					{Name: "success-rate", Threshold: 0.99},
				},
				Interval: metav1.Duration{Duration: 0},
			},
			wantErr: true,
			errMsg:  "interval must be positive",
		},
		{
			name: "invalid CEL expression",
			analysis: AnalysisConfig{
				Metrics: []MetricConfig{
					{Name: "success-rate", Threshold: 0.99},
				},
				Interval:         metav1.Duration{Duration: 30 * time.Second},
				SuccessCondition: "((",
			},
			wantErr: true,
			errMsg:  "invalid success condition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.analysis.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		metric  MetricConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid metric",
			metric:  MetricConfig{Name: "success-rate", Threshold: 0.99},
			wantErr: false,
		},
		{
			name:    "empty name",
			metric:  MetricConfig{Threshold: 0.99},
			wantErr: true,
			errMsg:  "metric name is required",
		},
		{
			name:    "negative threshold",
			metric:  MetricConfig{Name: "error-rate", Threshold: -0.1},
			wantErr: true,
			errMsg:  "threshold must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.metric.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTrafficRoutingValidation(t *testing.T) {
	tests := []struct {
		name    string
		routing TrafficRouting
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid istio routing",
			routing: TrafficRouting{
				Istio: &IstioTrafficRouting{
					VirtualService: "test-vs",
				},
			},
			wantErr: false,
		},
		{
			name: "valid nginx routing",
			routing: TrafficRouting{
				Nginx: &NginxTrafficRouting{
					Ingress: "test-ingress",
				},
			},
			wantErr: false,
		},
		{
			name:    "no provider specified",
			routing: TrafficRouting{},
			wantErr: true,
			errMsg:  "must specify at least one provider",
		},
		{
			name: "multiple providers",
			routing: TrafficRouting{
				Istio: &IstioTrafficRouting{VirtualService: "test-vs"},
				Nginx: &NginxTrafficRouting{Ingress: "test-ingress"},
			},
			wantErr: true,
			errMsg:  "can only specify one provider at a time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.routing.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHealthCheckConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		healthCheck HealthCheckConfig
		wantErr     bool
		errMsg      string
	}{
		{
			name: "valid health check",
			healthCheck: HealthCheckConfig{
				InitialDelay:     metav1.Duration{Duration: 30 * time.Second},
				Interval:         metav1.Duration{Duration: 10 * time.Second},
				Timeout:          metav1.Duration{Duration: 5 * time.Second},
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
			wantErr: false,
		},
		{
			name: "negative initial delay",
			healthCheck: HealthCheckConfig{
				InitialDelay:     metav1.Duration{Duration: -1 * time.Second},
				Interval:         metav1.Duration{Duration: 10 * time.Second},
				Timeout:          metav1.Duration{Duration: 5 * time.Second},
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
			wantErr: true,
			errMsg:  "initial delay must be non-negative",
		},
		{
			name: "zero interval",
			healthCheck: HealthCheckConfig{
				InitialDelay:     metav1.Duration{Duration: 30 * time.Second},
				Interval:         metav1.Duration{Duration: 0},
				Timeout:          metav1.Duration{Duration: 5 * time.Second},
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
			wantErr: true,
			errMsg:  "interval must be positive",
		},
		{
			name: "zero success threshold",
			healthCheck: HealthCheckConfig{
				InitialDelay:     metav1.Duration{Duration: 30 * time.Second},
				Interval:         metav1.Duration{Duration: 10 * time.Second},
				Timeout:          metav1.Duration{Duration: 5 * time.Second},
				SuccessThreshold: 0,
				FailureThreshold: 3,
			},
			wantErr: true,
			errMsg:  "success threshold must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.healthCheck.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRollingUpdateStrategyValidation(t *testing.T) {
	tests := []struct {
		name     string
		strategy RollingUpdateStrategy
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid with percentages",
			strategy: RollingUpdateStrategy{
				MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
				MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
			},
			wantErr: false,
		},
		{
			name: "valid with integers",
			strategy: RollingUpdateStrategy{
				MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			},
			wantErr: false,
		},
		{
			name: "negative integer value",
			strategy: RollingUpdateStrategy{
				MaxSurge: &intstr.IntOrString{Type: intstr.Int, IntVal: -1},
			},
			wantErr: true,
			errMsg:  "must be non-negative",
		},
		{
			name: "empty string value",
			strategy: RollingUpdateStrategy{
				MaxSurge: &intstr.IntOrString{Type: intstr.String, StrVal: ""},
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.strategy.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeploymentPlanValidation(t *testing.T) {
	tests := []struct {
		name    string
		plan    DeploymentPlan
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid plan",
			plan: DeploymentPlan{
				Strategy: DeploymentStrategy{
					Type: RecreateStrategyType,
				},
				Phases: []DeploymentPhase{
					{
						Name: "deploy",
						Actions: []DeploymentAction{
							{Type: ScaleAction, Target: "app"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid strategy",
			plan: DeploymentPlan{
				Strategy: DeploymentStrategy{},
				Phases: []DeploymentPhase{
					{Name: "deploy", Actions: []DeploymentAction{{Type: ScaleAction, Target: "app"}}},
				},
			},
			wantErr: true,
			errMsg:  "strategy validation failed",
		},
		{
			name: "no phases",
			plan: DeploymentPlan{
				Strategy: DeploymentStrategy{Type: RecreateStrategyType},
			},
			wantErr: true,
			errMsg:  "must have at least one phase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.plan.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCELExpression(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid expression",
			expr:    "success_rate >= 0.99 && error_rate <= 0.01",
			wantErr: false,
		},
		{
			name:    "empty expression",
			expr:    "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "too short",
			expr:    "a",
			wantErr: true,
			errMsg:  "too short to be valid",
		},
		{
			name:    "unmatched opening paren",
			expr:    "success_rate >= (0.99",
			wantErr: true,
			errMsg:  "unmatched opening parenthesis",
		},
		{
			name:    "unmatched closing paren",
			expr:    "success_rate >= 0.99)",
			wantErr: true,
			errMsg:  "unmatched closing parenthesis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCELExpression(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}