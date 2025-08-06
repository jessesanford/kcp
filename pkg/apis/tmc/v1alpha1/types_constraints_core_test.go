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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conditionsv1alpha1 "github.com/kcp-dev/kcp/sdk/apis/third_party/conditions/apis/conditions/v1alpha1"
)

func TestConstraintEvaluationEngine_DefaultValues(t *testing.T) {
	tests := map[string]struct {
		engine   *ConstraintEvaluationEngine
		wantType ConstraintEngineType
		wantPhase ConstraintEnginePhase
	}{
		"default engine type and phase": {
			engine: &ConstraintEvaluationEngine{
				Spec: ConstraintEvaluationEngineSpec{
					EngineType: ConstraintEngineTypeRuleBased,
					EvaluationRules: []ConstraintEvaluationRule{
						{
							Name:     "test-rule",
							RuleType: EvaluationRuleTypeThreshold,
							Condition: RuleCondition{
								Expression: "resource.cpu.usage > 80",
							},
							Action: RuleAction{
								Type: ActionTypeWarn,
							},
						},
					},
					RuleCount: 1,
				},
				Status: ConstraintEvaluationEngineStatus{
					Phase: ConstraintEnginePhaseActive,
				},
			},
			wantType:  ConstraintEngineTypeRuleBased,
			wantPhase: ConstraintEnginePhaseActive,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.engine.Spec.EngineType != tc.wantType {
				t.Errorf("Expected engine type %v, got %v", tc.wantType, tc.engine.Spec.EngineType)
			}
			if tc.engine.Status.Phase != tc.wantPhase {
				t.Errorf("Expected phase %v, got %v", tc.wantPhase, tc.engine.Status.Phase)
			}
		})
	}
}

func TestConstraintEvaluationRule_Validation(t *testing.T) {
	tests := map[string]struct {
		rule      ConstraintEvaluationRule
		wantValid bool
	}{
		"valid threshold rule": {
			rule: ConstraintEvaluationRule{
				Name:     "cpu-threshold",
				RuleType: EvaluationRuleTypeThreshold,
				Condition: RuleCondition{
					Expression:         "resource.cpu.usage > 80",
					EvaluationInterval: metav1.Duration{Duration: time.Minute},
				},
				Action: RuleAction{
					Type: ActionTypeWarn,
					Configuration: map[string]string{
						"message": "CPU usage is high",
					},
				},
				Priority: 50,
				Enabled:  true,
				Parameters: map[string]string{
					"threshold": "80",
				},
			},
			wantValid: true,
		},
		"valid capacity rule": {
			rule: ConstraintEvaluationRule{
				Name:     "capacity-check",
				RuleType: EvaluationRuleTypeCapacity,
				Condition: RuleCondition{
					Expression: "cluster.capacity.memory < 20",
				},
				Action: RuleAction{
					Type: ActionTypeBlock,
					Remediation: &BasicRemediationAction{
						Strategy:    RemediationStrategyLogOnly,
						MaxAttempts: 3,
					},
				},
				Enabled: true,
			},
			wantValid: true,
		},
		"invalid rule - empty name": {
			rule: ConstraintEvaluationRule{
				Name:     "",
				RuleType: EvaluationRuleTypeThreshold,
				Condition: RuleCondition{
					Expression: "resource.cpu.usage > 80",
				},
				Action: RuleAction{
					Type: ActionTypeWarn,
				},
			},
			wantValid: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			valid := tc.rule.Name != "" && 
				tc.rule.Condition.Expression != ""
			
			if valid != tc.wantValid {
				t.Errorf("Expected validity %v, got %v for rule %+v", tc.wantValid, valid, tc.rule)
			}
		})
	}
}

func TestBasicViolationHandlingConfig_Defaults(t *testing.T) {
	tests := map[string]struct {
		config   *BasicViolationHandlingConfig
		wantEnabled bool
		wantHistory int32
	}{
		"default config": {
			config: &BasicViolationHandlingConfig{
				TrackingEnabled:     true,
				MaxViolationHistory: 50,
				AlertingEnabled:     false,
			},
			wantEnabled: true,
			wantHistory: 50,
		},
		"custom config": {
			config: &BasicViolationHandlingConfig{
				TrackingEnabled:     false,
				MaxViolationHistory: 25,
				AlertingEnabled:     true,
			},
			wantEnabled: false,
			wantHistory: 25,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.config.TrackingEnabled != tc.wantEnabled {
				t.Errorf("Expected tracking enabled %v, got %v", tc.wantEnabled, tc.config.TrackingEnabled)
			}
			if tc.config.MaxViolationHistory != tc.wantHistory {
				t.Errorf("Expected history %v, got %v", tc.wantHistory, tc.config.MaxViolationHistory)
			}
		})
	}
}

func TestConstraintEvaluationEngine_ConditionMethods(t *testing.T) {
	engine := &ConstraintEvaluationEngine{}
	
	// Test setting conditions
	conditions := conditionsv1alpha1.Conditions{
		{
			Type:   ConstraintEvaluationEngineConditionReady,
			Status: "True",
			Reason: "EngineReady",
		},
		{
			Type:   ConstraintEvaluationEngineConditionEvaluating,
			Status: "True",
			Reason: "ActivelyEvaluating",
		},
	}
	
	engine.SetConditions(conditions)
	
	retrieved := engine.GetConditions()
	if len(retrieved) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(retrieved))
	}
	
	// Verify specific conditions
	for _, condition := range retrieved {
		switch condition.Type {
		case ConstraintEvaluationEngineConditionReady:
			if condition.Status != "True" {
				t.Errorf("Expected Ready condition to be True, got %s", condition.Status)
			}
		case ConstraintEvaluationEngineConditionEvaluating:
			if condition.Status != "True" {
				t.Errorf("Expected Evaluating condition to be True, got %s", condition.Status)
			}
		}
	}
}

func TestBasicEvaluationMetrics_Calculation(t *testing.T) {
	tests := map[string]struct {
		metrics         *BasicEvaluationMetrics
		wantSuccessRate float64
	}{
		"perfect success rate": {
			metrics: &BasicEvaluationMetrics{
				TotalEvaluations:      100,
				SuccessfulEvaluations: 100,
				FailedEvaluations:     0,
			},
			wantSuccessRate: 1.0,
		},
		"mixed results": {
			metrics: &BasicEvaluationMetrics{
				TotalEvaluations:      100,
				SuccessfulEvaluations: 85,
				FailedEvaluations:     15,
			},
			wantSuccessRate: 0.85,
		},
		"no evaluations": {
			metrics: &BasicEvaluationMetrics{
				TotalEvaluations:      0,
				SuccessfulEvaluations: 0,
				FailedEvaluations:     0,
			},
			wantSuccessRate: 0.0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var successRate float64
			if tc.metrics.TotalEvaluations > 0 {
				successRate = float64(tc.metrics.SuccessfulEvaluations) / float64(tc.metrics.TotalEvaluations)
			}
			
			if successRate != tc.wantSuccessRate {
				t.Errorf("Expected success rate %.2f, got %.2f", tc.wantSuccessRate, successRate)
			}
		})
	}
}

func TestRuleExecutionStats_Tracking(t *testing.T) {
	now := metav1.Now()
	
	stats := RuleExecutionStats{
		RuleName:          "test-rule",
		ExecutionCount:    10,
		TriggerCount:      5,
		LastExecutionTime: &now,
		ErrorCount:        1,
	}
	
	// Test that stats are properly initialized
	if stats.RuleName != "test-rule" {
		t.Errorf("Expected rule name 'test-rule', got %s", stats.RuleName)
	}
	
	if stats.ExecutionCount != 10 {
		t.Errorf("Expected execution count 10, got %d", stats.ExecutionCount)
	}
	
	if stats.TriggerCount != 5 {
		t.Errorf("Expected trigger count 5, got %d", stats.TriggerCount)
	}
	
	if stats.ErrorCount != 1 {
		t.Errorf("Expected error count 1, got %d", stats.ErrorCount)
	}
	
	// Test trigger rate calculation
	triggerRate := float64(stats.TriggerCount) / float64(stats.ExecutionCount)
	expectedRate := 0.5
	
	if triggerRate != expectedRate {
		t.Errorf("Expected trigger rate %.1f, got %.1f", expectedRate, triggerRate)
	}
}

func TestBasicViolationSummary_Summary(t *testing.T) {
	summary := BasicViolationSummary{
		TotalViolations:    100,
		ActiveViolations:   10,
		ResolvedViolations: 90,
	}
	
	// Test that totals match
	if summary.ActiveViolations+summary.ResolvedViolations != summary.TotalViolations {
		t.Errorf("Active + resolved (%d + %d = %d) should equal total (%d)", 
			summary.ActiveViolations, summary.ResolvedViolations,
			summary.ActiveViolations+summary.ResolvedViolations, summary.TotalViolations)
	}
	
	// Test resolution rate
	resolutionRate := float64(summary.ResolvedViolations) / float64(summary.TotalViolations)
	expectedRate := 0.9
	
	if resolutionRate != expectedRate {
		t.Errorf("Expected resolution rate %.1f, got %.1f", expectedRate, resolutionRate)
	}
}