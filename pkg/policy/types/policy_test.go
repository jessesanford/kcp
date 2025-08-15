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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPolicyValidation(t *testing.T) {
	tests := map[string]struct {
		policy   Policy
		expected bool
	}{
		"valid policy with required rule": {
			policy: Policy{
				Name: "data-residency",
				Rules: []PolicyRule{
					{
						Name:       "region-check",
						Expression: "cluster.region in ['us-west', 'us-east']",
						Required:   true,
					},
				},
				Priority: 100,
				Action:   RequireAction,
			},
			expected: true,
		},
		"policy with multiple rules": {
			policy: Policy{
				Name: "cost-optimization",
				Rules: []PolicyRule{
					{
						Name:       "cost-tier",
						Expression: "cluster.costTier == 'spot'",
						Weight:     int32Ptr(10),
					},
					{
						Name:       "region-preference",
						Expression: "cluster.region == workload.preferredRegion",
						Weight:     int32Ptr(20),
					},
				},
				Priority: 50,
				Action:   PreferAction,
			},
			expected: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Basic validation checks
			if tc.policy.Name == "" {
				t.Error("Policy name should not be empty")
			}
			if len(tc.policy.Rules) == 0 {
				t.Error("Policy should have at least one rule")
			}
			for _, rule := range tc.policy.Rules {
				if rule.Expression == "" {
					t.Error("Rule expression should not be empty")
				}
			}
		})
	}
}

func TestPolicySet(t *testing.T) {
	policySet := PolicySet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "placement-policies",
		},
		Policies: []Policy{
			{
				Name:     "region-affinity",
				Priority: 100,
				Action:   PreferAction,
				Rules: []PolicyRule{
					{
						Name:       "same-region",
						Expression: "cluster.region == workload.region",
					},
				},
			},
		},
		DefaultAction:      AllowAction,
		ConflictResolution: HighestPriorityStrategy,
	}

	if policySet.Name != "placement-policies" {
		t.Errorf("Expected policy set name 'placement-policies', got %s", policySet.Name)
	}
	if len(policySet.Policies) != 1 {
		t.Errorf("Expected 1 policy, got %d", len(policySet.Policies))
	}
	if policySet.DefaultAction != AllowAction {
		t.Errorf("Expected default action Allow, got %s", policySet.DefaultAction)
	}
}

func TestExpressionTypes(t *testing.T) {
	expr := Expression{
		Source: "cluster.labels.tier == 'production'",
		Variables: []Variable{
			{
				Name:     "cluster",
				Type:     ObjectType,
				Required: true,
			},
		},
	}

	if expr.Source == "" {
		t.Error("Expression source should not be empty")
	}
	if len(expr.Variables) != 1 {
		t.Errorf("Expected 1 variable, got %d", len(expr.Variables))
	}
	if expr.Variables[0].Type != ObjectType {
		t.Errorf("Expected ObjectType, got %s", expr.Variables[0].Type)
	}
}

func TestPolicyActions(t *testing.T) {
	actions := []PolicyAction{AllowAction, DenyAction, PreferAction, RequireAction}
	expected := []string{"Allow", "Deny", "Prefer", "Require"}

	for i, action := range actions {
		if string(action) != expected[i] {
			t.Errorf("Expected action %s, got %s", expected[i], string(action))
		}
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}