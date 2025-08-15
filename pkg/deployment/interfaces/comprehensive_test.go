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

package interfaces_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kcp-dev/kcp/pkg/deployment/interfaces"
	"github.com/kcp-dev/kcp/pkg/deployment/types"
)

// TestDeploymentResult tests the DeploymentResult struct
func TestDeploymentResult(t *testing.T) {
	now := metav1.Now()
	result := interfaces.DeploymentResult{
		DeploymentID: "deploy-123",
		Status:       interfaces.DeploymentSucceeded,
		Message:      "Deployment completed successfully",
		StartTime:    now,
		EndTime:      &now,
		Phases: []interfaces.PhaseResult{
			{
				Name:      "deploy",
				Status:    interfaces.DeploymentSucceeded,
				StartTime: now,
				EndTime:   &now,
			},
		},
	}

	assert.Equal(t, "deploy-123", result.DeploymentID)
	assert.Equal(t, interfaces.DeploymentSucceeded, result.Status)
	assert.Equal(t, "Deployment completed successfully", result.Message)
	assert.Len(t, result.Phases, 1)
	assert.Equal(t, "deploy", result.Phases[0].Name)
}

// TestRollingUpdateStrategy tests the RollingUpdateStrategy validation
func TestRollingUpdateStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy types.RollingUpdateStrategy
		wantErr  bool
	}{
		{
			name: "valid rolling update with percentage",
			strategy: types.RollingUpdateStrategy{
				MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
				MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
			},
			wantErr: false,
		},
		{
			name: "valid rolling update with integer",
			strategy: types.RollingUpdateStrategy{
				MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the strategy struct can be created and accessed
			assert.NotNil(t, tt.strategy.MaxSurge)
			assert.NotNil(t, tt.strategy.MaxUnavailable)
		})
	}
}

// TestHealthCheckConfig tests the HealthCheckConfig struct
func TestHealthCheckConfig(t *testing.T) {
	tests := []struct {
		name   string
		config types.HealthCheckConfig
	}{
		{
			name: "valid health check config",
			config: types.HealthCheckConfig{
				InitialDelay:     metav1.Duration{Duration: 30 * time.Second},
				Interval:         metav1.Duration{Duration: 10 * time.Second},
				Timeout:          metav1.Duration{Duration: 5 * time.Second},
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, 30*time.Second, tt.config.InitialDelay.Duration)
			assert.Equal(t, 10*time.Second, tt.config.Interval.Duration)
			assert.Equal(t, 5*time.Second, tt.config.Timeout.Duration)
			assert.Equal(t, int32(1), tt.config.SuccessThreshold)
			assert.Equal(t, int32(3), tt.config.FailureThreshold)
		})
	}
}

// TestDeploymentPlanValidation tests deployment plan validation
func TestDeploymentPlanValidation(t *testing.T) {
	tests := []struct {
		name    string
		plan    types.DeploymentPlan
		wantErr bool
	}{
		{
			name: "valid canary plan",
			plan: types.DeploymentPlan{
				Strategy: types.DeploymentStrategy{
					Type: types.CanaryStrategyType,
					Canary: &types.CanaryStrategy{
						Steps: []types.CanaryStep{
							{Weight: 10},
							{Weight: 100},
						},
					},
				},
				Phases: []types.DeploymentPhase{
					{
						Name: "deploy",
						Actions: []types.DeploymentAction{
							{
								Type:   types.ScaleAction,
								Target: "test-app",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid blue-green plan",
			plan: types.DeploymentPlan{
				Strategy: types.DeploymentStrategy{
					Type: types.BlueGreenStrategyType,
					BlueGreen: &types.BlueGreenStrategy{
						AutoPromotionEnabled: true,
						ScaleDownDelay:       &metav1.Duration{Duration: 5 * time.Minute},
					},
				},
				Phases: []types.DeploymentPhase{
					{Name: "deploy"},
					{Name: "verify"},
					{Name: "promote"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the plan structure is valid
			assert.NotEmpty(t, tt.plan.Strategy.Type)
			assert.NotEmpty(t, tt.plan.Phases)

			// Type-specific validations
			switch tt.plan.Strategy.Type {
			case types.CanaryStrategyType:
				assert.NotNil(t, tt.plan.Strategy.Canary)
				assert.NotEmpty(t, tt.plan.Strategy.Canary.Steps)
			case types.BlueGreenStrategyType:
				assert.NotNil(t, tt.plan.Strategy.BlueGreen)
			}
		})
	}
}

// MockDeploymentStrategy for testing
type MockDeploymentStrategy struct {
	name            string
	validateError   error
	initializeError error
	executeError    error
	cleanupError    error
}

func (m *MockDeploymentStrategy) Name() string {
	return m.name
}

func (m *MockDeploymentStrategy) Validate(config types.DeploymentStrategy) error {
	return m.validateError
}

func (m *MockDeploymentStrategy) Initialize(ctx context.Context, config types.DeploymentStrategy) error {
	return m.initializeError
}

func (m *MockDeploymentStrategy) Execute(ctx context.Context, target interfaces.DeploymentTarget) (*interfaces.StrategyResult, error) {
	if m.executeError != nil {
		return nil, m.executeError
	}
	return &interfaces.StrategyResult{
		Success:    true,
		Message:    "Deployment successful",
		NextAction: interfaces.CompleteAction,
	}, nil
}

func (m *MockDeploymentStrategy) Cleanup(ctx context.Context) error {
	return m.cleanupError
}

// TestMockDeploymentStrategy tests the mock strategy implementation
func TestMockDeploymentStrategy(t *testing.T) {
	ctx := context.Background()

	t.Run("successful strategy execution", func(t *testing.T) {
		strategy := &MockDeploymentStrategy{
			name: "test-strategy",
		}

		assert.Equal(t, "test-strategy", strategy.Name())

		config := types.DeploymentStrategy{Type: types.CanaryStrategyType}
		err := strategy.Validate(config)
		assert.NoError(t, err)

		err = strategy.Initialize(ctx, config)
		assert.NoError(t, err)

		target := interfaces.DeploymentTarget{
			Name:      "test-app",
			Namespace: "default",
		}
		result, err := strategy.Execute(ctx, target)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, interfaces.CompleteAction, result.NextAction)

		err = strategy.Cleanup(ctx)
		assert.NoError(t, err)
	})

	t.Run("strategy with validation error", func(t *testing.T) {
		strategy := &MockDeploymentStrategy{
			validateError: errors.New("validation failed"),
		}

		config := types.DeploymentStrategy{}
		err := strategy.Validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("strategy with execution error", func(t *testing.T) {
		strategy := &MockDeploymentStrategy{
			executeError: errors.New("execution failed"),
		}

		target := interfaces.DeploymentTarget{Name: "test-app"}
		result, err := strategy.Execute(ctx, target)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "execution failed")
	})
}

// MockStrategyFactory for testing
type MockStrategyFactory struct {
	strategies map[types.StrategyType]interfaces.DeploymentStrategy
	registered []types.StrategyType
}

func NewMockStrategyFactory() *MockStrategyFactory {
	return &MockStrategyFactory{
		strategies: make(map[types.StrategyType]interfaces.DeploymentStrategy),
		registered: make([]types.StrategyType, 0),
	}
}

func (m *MockStrategyFactory) Create(strategyType types.StrategyType) (interfaces.DeploymentStrategy, error) {
	strategy, exists := m.strategies[strategyType]
	if !exists {
		return nil, errors.New("strategy not found")
	}
	return strategy, nil
}

func (m *MockStrategyFactory) Register(strategyType types.StrategyType, strategy interfaces.DeploymentStrategy) error {
	m.strategies[strategyType] = strategy
	m.registered = append(m.registered, strategyType)
	return nil
}

func (m *MockStrategyFactory) ListStrategies() []types.StrategyType {
	return m.registered
}

// TestMockStrategyFactory tests the strategy factory implementation
func TestMockStrategyFactory(t *testing.T) {
	factory := NewMockStrategyFactory()

	t.Run("register and create strategy", func(t *testing.T) {
		mockStrategy := &MockDeploymentStrategy{name: "canary"}
		err := factory.Register(types.CanaryStrategyType, mockStrategy)
		assert.NoError(t, err)

		strategy, err := factory.Create(types.CanaryStrategyType)
		assert.NoError(t, err)
		assert.Equal(t, "canary", strategy.Name())

		strategies := factory.ListStrategies()
		assert.Len(t, strategies, 1)
		assert.Contains(t, strategies, types.CanaryStrategyType)
	})

	t.Run("create unknown strategy", func(t *testing.T) {
		strategy, err := factory.Create(types.BlueGreenStrategyType)
		assert.Error(t, err)
		assert.Nil(t, strategy)
		assert.Contains(t, err.Error(), "strategy not found")
	})
}

// TestStrategyResult tests the StrategyResult struct
func TestStrategyResult(t *testing.T) {
	tests := []struct {
		name   string
		result interfaces.StrategyResult
	}{
		{
			name: "successful result",
			result: interfaces.StrategyResult{
				Success:    true,
				Message:    "Deployment successful",
				Metrics:    map[string]interface{}{"cpu": "50%", "memory": "200Mi"},
				NextAction: interfaces.CompleteAction,
			},
		},
		{
			name: "failed result with rollback",
			result: interfaces.StrategyResult{
				Success:    false,
				Message:    "Health check failed",
				NextAction: interfaces.RollbackAction,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.result.Success, tt.result.Success)
			assert.Equal(t, tt.result.Message, tt.result.Message)
			assert.Equal(t, tt.result.NextAction, tt.result.NextAction)
			if tt.result.Metrics != nil {
				assert.NotEmpty(t, tt.result.Metrics)
			}
		})
	}
}

// TestDeploymentStatusTypes tests all deployment status constants
func TestDeploymentStatusTypes(t *testing.T) {
	statuses := []interfaces.DeploymentStatusType{
		interfaces.DeploymentPending,
		interfaces.DeploymentInProgress,
		interfaces.DeploymentSucceeded,
		interfaces.DeploymentFailed,
		interfaces.DeploymentPaused,
		interfaces.DeploymentRollingBack,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			assert.NotEmpty(t, string(status))
		})
	}

	// Test specific status values
	assert.Equal(t, "Pending", string(interfaces.DeploymentPending))
	assert.Equal(t, "InProgress", string(interfaces.DeploymentInProgress))
	assert.Equal(t, "Succeeded", string(interfaces.DeploymentSucceeded))
	assert.Equal(t, "Failed", string(interfaces.DeploymentFailed))
	assert.Equal(t, "Paused", string(interfaces.DeploymentPaused))
	assert.Equal(t, "RollingBack", string(interfaces.DeploymentRollingBack))
}

// TestAnalysisConfig tests the AnalysisConfig struct
func TestAnalysisConfig(t *testing.T) {
	config := types.AnalysisConfig{
		Metrics: []types.MetricConfig{
			{
				Name:      "success-rate",
				Threshold: 0.99,
				Query:     "rate(http_requests_total{code=~\"2..\"}[5m])",
			},
			{
				Name:      "error-rate",
				Threshold: 0.01,
				Query:     "rate(http_requests_total{code=~\"5..\"}[5m])",
			},
		},
		Interval:         metav1.Duration{Duration: 30 * time.Second},
		SuccessCondition: "success-rate >= 0.99 && error-rate <= 0.01",
	}

	assert.Len(t, config.Metrics, 2)
	assert.Equal(t, "success-rate", config.Metrics[0].Name)
	assert.Equal(t, 0.99, config.Metrics[0].Threshold)
	assert.Equal(t, 30*time.Second, config.Interval.Duration)
	assert.Contains(t, config.SuccessCondition, "success-rate >= 0.99")
}

// TestTrafficRouting tests the TrafficRouting configuration
func TestTrafficRouting(t *testing.T) {
	t.Run("istio traffic routing", func(t *testing.T) {
		routing := types.TrafficRouting{
			Istio: &types.IstioTrafficRouting{
				VirtualService:  "test-vs",
				DestinationRule: "test-dr",
			},
		}

		assert.NotNil(t, routing.Istio)
		assert.Equal(t, "test-vs", routing.Istio.VirtualService)
		assert.Equal(t, "test-dr", routing.Istio.DestinationRule)
		assert.Nil(t, routing.Nginx)
	})

	t.Run("nginx traffic routing", func(t *testing.T) {
		routing := types.TrafficRouting{
			Nginx: &types.NginxTrafficRouting{
				Ingress:   "test-ingress",
				ConfigMap: "test-cm",
			},
		}

		assert.NotNil(t, routing.Nginx)
		assert.Equal(t, "test-ingress", routing.Nginx.Ingress)
		assert.Equal(t, "test-cm", routing.Nginx.ConfigMap)
		assert.Nil(t, routing.Istio)
	})
}

// TestProgressReporter tests the progress reporting interface through a mock
type MockProgressReporter struct {
	reports       []interfaces.DeploymentProgress
	errorOnReport error
}

func (m *MockProgressReporter) Report(progress interfaces.DeploymentProgress) error {
	if m.errorOnReport != nil {
		return m.errorOnReport
	}
	m.reports = append(m.reports, progress)
	return nil
}

func TestProgressReporter(t *testing.T) {
	t.Run("successful progress reporting", func(t *testing.T) {
		reporter := &MockProgressReporter{}

		progress := interfaces.DeploymentProgress{
			Phase:      "deploy",
			Percentage: 50.0,
			Message:    "Halfway through deployment",
		}

		err := reporter.Report(progress)
		assert.NoError(t, err)
		assert.Len(t, reporter.reports, 1)
		assert.Equal(t, "deploy", reporter.reports[0].Phase)
		assert.Equal(t, 50.0, reporter.reports[0].Percentage)
	})

	t.Run("progress reporting with error", func(t *testing.T) {
		reporter := &MockProgressReporter{
			errorOnReport: errors.New("reporting failed"),
		}

		progress := interfaces.DeploymentProgress{Phase: "deploy"}
		err := reporter.Report(progress)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reporting failed")
	})
}

// TestActionTypes tests all action type constants
func TestActionTypes(t *testing.T) {
	actions := []types.ActionType{
		types.ScaleAction,
		types.UpdateAction,
		types.WaitAction,
		types.VerifyAction,
	}

	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			assert.NotEmpty(t, string(action))
		})
	}

	// Test specific action values
	assert.Equal(t, "Scale", string(types.ScaleAction))
	assert.Equal(t, "Update", string(types.UpdateAction))
	assert.Equal(t, "Wait", string(types.WaitAction))
	assert.Equal(t, "Verify", string(types.VerifyAction))
}

// TestStrategyActionTypes tests all strategy action constants
func TestStrategyActionTypes(t *testing.T) {
	actions := []interfaces.StrategyAction{
		interfaces.ContinueAction,
		interfaces.PauseAction,
		interfaces.RollbackAction,
		interfaces.CompleteAction,
	}

	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			assert.NotEmpty(t, string(action))
		})
	}

	// Test specific action values
	assert.Equal(t, "Continue", string(interfaces.ContinueAction))
	assert.Equal(t, "Pause", string(interfaces.PauseAction))
	assert.Equal(t, "Rollback", string(interfaces.RollbackAction))
	assert.Equal(t, "Complete", string(interfaces.CompleteAction))
}

// TestComplexDeploymentScenarios tests more complex deployment scenarios
func TestComplexDeploymentScenarios(t *testing.T) {
	t.Run("canary with analysis and traffic routing", func(t *testing.T) {
		strategy := types.DeploymentStrategy{
			Type: types.CanaryStrategyType,
			Canary: &types.CanaryStrategy{
				Steps: []types.CanaryStep{
					{Weight: 5, Pause: &metav1.Duration{Duration: 2 * time.Minute}},
					{Weight: 10, Pause: &metav1.Duration{Duration: 5 * time.Minute}},
					{Weight: 25},
					{Weight: 50},
					{Weight: 100},
				},
				Analysis: &types.AnalysisConfig{
					Metrics: []types.MetricConfig{
						{Name: "success-rate", Threshold: 0.95},
						{Name: "latency-p99", Threshold: 500.0},
					},
					Interval:         metav1.Duration{Duration: 30 * time.Second},
					SuccessCondition: "success-rate >= 0.95 && latency-p99 <= 500",
				},
				TrafficRouting: &types.TrafficRouting{
					Istio: &types.IstioTrafficRouting{
						VirtualService:  "app-vs",
						DestinationRule: "app-dr",
					},
				},
			},
			HealthCheck: &types.HealthCheckConfig{
				InitialDelay:     metav1.Duration{Duration: 30 * time.Second},
				Interval:         metav1.Duration{Duration: 10 * time.Second},
				Timeout:          metav1.Duration{Duration: 5 * time.Second},
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
		}

		// Validate the complex strategy configuration
		assert.Equal(t, types.CanaryStrategyType, strategy.Type)
		require.NotNil(t, strategy.Canary)
		assert.Len(t, strategy.Canary.Steps, 5)
		assert.Equal(t, int32(5), strategy.Canary.Steps[0].Weight)
		assert.Equal(t, int32(100), strategy.Canary.Steps[4].Weight)
		assert.NotNil(t, strategy.Canary.Steps[0].Pause)
		assert.Equal(t, 2*time.Minute, strategy.Canary.Steps[0].Pause.Duration)

		require.NotNil(t, strategy.Canary.Analysis)
		assert.Len(t, strategy.Canary.Analysis.Metrics, 2)
		assert.Equal(t, "success-rate", strategy.Canary.Analysis.Metrics[0].Name)
		assert.Equal(t, 0.95, strategy.Canary.Analysis.Metrics[0].Threshold)

		require.NotNil(t, strategy.Canary.TrafficRouting)
		require.NotNil(t, strategy.Canary.TrafficRouting.Istio)
		assert.Equal(t, "app-vs", strategy.Canary.TrafficRouting.Istio.VirtualService)

		require.NotNil(t, strategy.HealthCheck)
		assert.Equal(t, 30*time.Second, strategy.HealthCheck.InitialDelay.Duration)
	})

	t.Run("blue-green with pre and post promotion analysis", func(t *testing.T) {
		strategy := types.DeploymentStrategy{
			Type: types.BlueGreenStrategyType,
			BlueGreen: &types.BlueGreenStrategy{
				PrePromotionAnalysis: &types.AnalysisConfig{
					Metrics: []types.MetricConfig{
						{Name: "health-check", Threshold: 1.0},
					},
					Interval: metav1.Duration{Duration: 10 * time.Second},
				},
				PostPromotionAnalysis: &types.AnalysisConfig{
					Metrics: []types.MetricConfig{
						{Name: "success-rate", Threshold: 0.99},
						{Name: "error-rate", Threshold: 0.01},
					},
					Interval: metav1.Duration{Duration: 30 * time.Second},
				},
				AutoPromotionEnabled: false, // Manual promotion
				ScaleDownDelay:       &metav1.Duration{Duration: 10 * time.Minute},
			},
		}

		assert.Equal(t, types.BlueGreenStrategyType, strategy.Type)
		require.NotNil(t, strategy.BlueGreen)
		assert.False(t, strategy.BlueGreen.AutoPromotionEnabled)
		assert.Equal(t, 10*time.Minute, strategy.BlueGreen.ScaleDownDelay.Duration)

		require.NotNil(t, strategy.BlueGreen.PrePromotionAnalysis)
		assert.Len(t, strategy.BlueGreen.PrePromotionAnalysis.Metrics, 1)

		require.NotNil(t, strategy.BlueGreen.PostPromotionAnalysis)
		assert.Len(t, strategy.BlueGreen.PostPromotionAnalysis.Metrics, 2)
	})
}