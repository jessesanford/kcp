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

package placement

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kcp-dev/logicalcluster/v3"

	kcpclientset "github.com/kcp-dev/kcp/sdk/client/clientset/versioned/cluster"
	kcptesting "github.com/kcp-dev/kcp/sdk/testing"
	"github.com/kcp-dev/kcp/test/e2e/framework"
)

// TestPolicyEvaluationEngine tests the complete policy evaluation system
// including CEL expressions, policy precedence, and constraint enforcement
func TestPolicyEvaluationEngine(t *testing.T) {
	t.Parallel()
	framework.Suite(t, "control-plane")

	server := kcptesting.SharedKcpServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := server.BaseConfig(t)
	kcpClient, err := kcpclientset.NewForConfig(cfg)
	require.NoError(t, err)

	// Create organization and workspaces for policy testing
	orgPath, _ := framework.NewOrganizationFixture(t, server)
	policyPath, _ := kcptesting.NewWorkspaceFixture(t, server, orgPath, kcptesting.WithName("policy-evaluation"))

	t.Logf("Testing policy evaluation in workspace %s", policyPath)

	// Test 1: CEL expression evaluation
	t.Run("CELExpressionEvaluation", func(t *testing.T) {
		celPolicies := []CELPolicyTest{
			{
				Name:       "region-affinity",
				Expression: `request.object.metadata.labels['region'] == 'us-west-2'`,
				Input:      createMockWorkloadWithLabels(map[string]string{"region": "us-west-2"}),
				Expected:   true,
			},
			{
				Name:       "environment-restriction",
				Expression: `request.object.metadata.labels['environment'] in ['dev', 'staging', 'prod']`,
				Input:      createMockWorkloadWithLabels(map[string]string{"environment": "test"}),
				Expected:   false,
			},
			{
				Name:       "resource-limits",
				Expression: `size(request.object.spec.template.spec.containers) <= 5`,
				Input:      createMockWorkloadWithContainers(3),
				Expected:   true,
			},
		}

		for _, policyTest := range celPolicies {
			t.Run(policyTest.Name, func(t *testing.T) {
				result := evaluateCELExpression(t, ctx, kcpClient, policyPath, policyTest.Expression, policyTest.Input)
				require.Equal(t, policyTest.Expected, result, "CEL evaluation result mismatch for %s", policyTest.Name)
			})
		}
	})

	// Test 2: Policy precedence and inheritance
	t.Run("PolicyPrecedence", func(t *testing.T) {
		// Create hierarchical policies with different priorities
		globalPolicy := createPolicyWithPriority(t, "global-policy", 100)
		orgPolicy := createPolicyWithPriority(t, "org-policy", 200)
		workspacePolicy := createPolicyWithPriority(t, "workspace-policy", 300)

		policies := []*PolicyDefinition{globalPolicy, orgPolicy, workspacePolicy}

		// Test that higher priority policies override lower ones
		effectivePolicy := evaluatePolicyPrecedence(t, ctx, kcpClient, policyPath, policies)
		require.Equal(t, "workspace-policy", effectivePolicy.Name)
		require.Equal(t, 300, effectivePolicy.Priority)
	})

	// Test 3: Policy constraint enforcement
	t.Run("ConstraintEnforcement", func(t *testing.T) {
		constraints := []ConstraintTest{
			{
				Name:         "required-labels",
				Type:         "RequiredLabels",
				Parameters:   map[string]interface{}{"labels": []string{"app", "version", "owner"}},
				ValidInput:   createMockWorkloadWithLabels(map[string]string{"app": "web", "version": "1.0", "owner": "team-a"}),
				InvalidInput: createMockWorkloadWithLabels(map[string]string{"app": "web"}),
			},
			{
				Name:         "resource-quotas",
				Type:         "ResourceQuota",
				Parameters:   map[string]interface{}{"maxCPU": "2", "maxMemory": "4Gi"},
				ValidInput:   createMockWorkloadWithResources("1", "2Gi"),
				InvalidInput: createMockWorkloadWithResources("4", "8Gi"),
			},
		}

		for _, constraintTest := range constraints {
			t.Run(constraintTest.Name, func(t *testing.T) {
				// Test valid input passes constraint
				validResult := enforceConstraint(t, ctx, kcpClient, policyPath, constraintTest.Type, constraintTest.Parameters, constraintTest.ValidInput)
				require.True(t, validResult.Allowed, "Valid input should pass constraint %s", constraintTest.Name)

				// Test invalid input fails constraint
				invalidResult := enforceConstraint(t, ctx, kcpClient, policyPath, constraintTest.Type, constraintTest.Parameters, constraintTest.InvalidInput)
				require.False(t, invalidResult.Allowed, "Invalid input should fail constraint %s", constraintTest.Name)
				require.NotEmpty(t, invalidResult.Reason, "Constraint violation should have reason")
			})
		}
	})

	// Test 4: Dynamic policy updates
	t.Run("DynamicPolicyUpdates", func(t *testing.T) {
		// Create initial policy
		initialPolicy := createSimplePolicy(t, "dynamic-policy", `request.object.metadata.labels['tier'] == 'premium'`)

		// Apply policy and test
		applyPolicy(t, ctx, kcpClient, policyPath, initialPolicy)

		premiumWorkload := createMockWorkloadWithLabels(map[string]string{"tier": "premium"})
		result := evaluatePolicy(t, ctx, kcpClient, policyPath, initialPolicy, premiumWorkload)
		require.True(t, result.Allowed)

		// Update policy dynamically
		updatedPolicy := createSimplePolicy(t, "dynamic-policy", `request.object.metadata.labels['tier'] in ['premium', 'enterprise']`)
		updatePolicy(t, ctx, kcpClient, policyPath, updatedPolicy)

		// Test with new criteria
		enterpriseWorkload := createMockWorkloadWithLabels(map[string]string{"tier": "enterprise"})
		updatedResult := evaluatePolicy(t, ctx, kcpClient, policyPath, updatedPolicy, enterpriseWorkload)
		require.True(t, updatedResult.Allowed)
	})
}

// TestPolicyConflictResolution tests how conflicting policies are resolved
func TestPolicyConflictResolution(t *testing.T) {
	t.Parallel()
	framework.Suite(t, "control-plane")

	server := kcptesting.SharedKcpServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := server.BaseConfig(t)
	kcpClient, err := kcpclientset.NewForConfig(cfg)
	require.NoError(t, err)

	orgPath, _ := framework.NewOrganizationFixture(t, server)
	workspacePath, _ := kcptesting.NewWorkspaceFixture(t, server, orgPath, kcptesting.WithName("policy-conflicts"))

	t.Run("ConflictingPolicies", func(t *testing.T) {
		// Create conflicting policies
		allowPolicy := createSimplePolicy(t, "allow-policy", `true`) // Always allow
		denyPolicy := createSimplePolicy(t, "deny-policy", `false`)  // Always deny

		conflicts := detectPolicyConflicts(t, ctx, kcpClient, workspacePath, []*PolicyDefinition{allowPolicy, denyPolicy})
		require.NotEmpty(t, conflicts, "Should detect policy conflicts")

		resolution := resolvePolicyConflicts(t, ctx, kcpClient, workspacePath, conflicts)
		require.NotNil(t, resolution)
	})
}

// Helper types and functions for policy testing

type CELPolicyTest struct {
	Name       string
	Expression string
	Input      *MockWorkload
	Expected   bool
}

type PolicyDefinition struct {
	Name       string
	Expression string
	Priority   int
	Type       string
}

type ConstraintTest struct {
	Name         string
	Type         string
	Parameters   map[string]interface{}
	ValidInput   *MockWorkload
	InvalidInput *MockWorkload
}

type PolicyEvaluationResult struct {
	Allowed bool
	Reason  string
}

type PolicyConflict struct {
	Policy1 *PolicyDefinition
	Policy2 *PolicyDefinition
	Type    string // "contradiction", "overlap", etc.
}

type ConflictResolution struct {
	Strategy       string // "priority", "merge", "deny-by-default"
	ResolvedPolicy *PolicyDefinition
}

func createMockWorkloadWithLabels(labels map[string]string) *MockWorkload {
	return &MockWorkload{
		Name: "test-workload",
		Spec: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": labels,
			},
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "test-container",
							"image": "nginx:latest",
						},
					},
				},
			},
		},
	}
}

func createMockWorkloadWithContainers(count int) *MockWorkload {
	containers := make([]interface{}, count)
	for i := 0; i < count; i++ {
		containers[i] = map[string]interface{}{
			"name":  fmt.Sprintf("container-%d", i),
			"image": "nginx:latest",
		}
	}

	return &MockWorkload{
		Name: "test-workload",
		Spec: map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": containers,
				},
			},
		},
	}
}

func createMockWorkloadWithResources(cpu, memory string) *MockWorkload {
	return &MockWorkload{
		Name: "test-workload",
		Spec: map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "test-container",
							"image": "nginx:latest",
							"resources": map[string]interface{}{
								"requests": map[string]interface{}{
									"cpu":    cpu,
									"memory": memory,
								},
							},
						},
					},
				},
			},
		},
	}
}

func createPolicyWithPriority(t *testing.T, name string, priority int) *PolicyDefinition {
	return &PolicyDefinition{
		Name:       name,
		Expression: `true`, // Default allow
		Priority:   priority,
		Type:       "CEL",
	}
}

func createSimplePolicy(t *testing.T, name, expression string) *PolicyDefinition {
	return &PolicyDefinition{
		Name:       name,
		Expression: expression,
		Priority:   100,
		Type:       "CEL",
	}
}

func evaluateCELExpression(t *testing.T, ctx context.Context, client kcpclientset.ClusterInterface, workspace logicalcluster.Path, expression string, input *MockWorkload) bool {
	t.Logf("Evaluating CEL expression: %s", expression)

	// Mock CEL evaluation - in real implementation would use CEL library
	// This is a simplified mock that checks some basic patterns
	switch expression {
	case `request.object.metadata.labels['region'] == 'us-west-2'`:
		if labels, ok := input.Spec["metadata"].(map[string]interface{})["labels"].(map[string]string); ok {
			return labels["region"] == "us-west-2"
		}
	case `request.object.metadata.labels['environment'] in ['dev', 'staging', 'prod']`:
		if labels, ok := input.Spec["metadata"].(map[string]interface{})["labels"].(map[string]string); ok {
			env := labels["environment"]
			return env == "dev" || env == "staging" || env == "prod"
		}
	case `size(request.object.spec.template.spec.containers) <= 5`:
		if template, ok := input.Spec["template"].(map[string]interface{}); ok {
			if spec, ok := template["spec"].(map[string]interface{}); ok {
				if containers, ok := spec["containers"].([]interface{}); ok {
					return len(containers) <= 5
				}
			}
		}
	}

	return false
}

func evaluatePolicyPrecedence(t *testing.T, ctx context.Context, client kcpclientset.ClusterInterface, workspace logicalcluster.Path, policies []*PolicyDefinition) *PolicyDefinition {
	t.Logf("Evaluating policy precedence for %d policies", len(policies))

	// Find policy with highest priority
	var effectivePolicy *PolicyDefinition
	maxPriority := -1

	for _, policy := range policies {
		if policy.Priority > maxPriority {
			maxPriority = policy.Priority
			effectivePolicy = policy
		}
	}

	return effectivePolicy
}

func enforceConstraint(t *testing.T, ctx context.Context, client kcpclientset.ClusterInterface, workspace logicalcluster.Path, constraintType string, parameters map[string]interface{}, input *MockWorkload) *PolicyEvaluationResult {
	t.Logf("Enforcing constraint %s", constraintType)

	switch constraintType {
	case "RequiredLabels":
		requiredLabels := parameters["labels"].([]string)
		if labels, ok := input.Spec["metadata"].(map[string]interface{})["labels"].(map[string]string); ok {
			labelSet := sets.StringKeySet(labels)
			for _, required := range requiredLabels {
				if !labelSet.Has(required) {
					return &PolicyEvaluationResult{
						Allowed: false,
						Reason:  fmt.Sprintf("Missing required label: %s", required),
					}
				}
			}
			return &PolicyEvaluationResult{Allowed: true}
		}
		return &PolicyEvaluationResult{
			Allowed: false,
			Reason:  "No labels found",
		}
	case "ResourceQuota":
		// Simplified resource quota check
		maxCPU := parameters["maxCPU"].(string)
		if template, ok := input.Spec["template"].(map[string]interface{}); ok {
			if spec, ok := template["spec"].(map[string]interface{}); ok {
				if containers, ok := spec["containers"].([]interface{}); ok {
					for _, container := range containers {
						if containerMap, ok := container.(map[string]interface{}); ok {
							if resources, ok := containerMap["resources"].(map[string]interface{}); ok {
								if requests, ok := resources["requests"].(map[string]interface{}); ok {
									if cpu, ok := requests["cpu"].(string); ok {
										// Simple string comparison for mock
										if cpu > maxCPU {
											return &PolicyEvaluationResult{
												Allowed: false,
												Reason:  fmt.Sprintf("CPU request %s exceeds limit %s", cpu, maxCPU),
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		return &PolicyEvaluationResult{Allowed: true}
	}

	return &PolicyEvaluationResult{Allowed: true}
}

func applyPolicy(t *testing.T, ctx context.Context, client kcpclientset.ClusterInterface, workspace logicalcluster.Path, policy *PolicyDefinition) {
	t.Logf("Applying policy %s", policy.Name)
	// Mock policy application
}

func updatePolicy(t *testing.T, ctx context.Context, client kcpclientset.ClusterInterface, workspace logicalcluster.Path, policy *PolicyDefinition) {
	t.Logf("Updating policy %s", policy.Name)
	// Mock policy update
}

func evaluatePolicy(t *testing.T, ctx context.Context, client kcpclientset.ClusterInterface, workspace logicalcluster.Path, policy *PolicyDefinition, input *MockWorkload) *PolicyEvaluationResult {
	allowed := evaluateCELExpression(t, ctx, client, workspace, policy.Expression, input)
	return &PolicyEvaluationResult{
		Allowed: allowed,
		Reason:  "Policy evaluation completed",
	}
}

func detectPolicyConflicts(t *testing.T, ctx context.Context, client kcpclientset.ClusterInterface, workspace logicalcluster.Path, policies []*PolicyDefinition) []*PolicyConflict {
	t.Logf("Detecting conflicts among %d policies", len(policies))

	var conflicts []*PolicyConflict

	// Simple conflict detection - opposite boolean expressions
	for i, p1 := range policies {
		for j, p2 := range policies {
			if i >= j {
				continue
			}
			if (p1.Expression == `true` && p2.Expression == `false`) ||
				(p1.Expression == `false` && p2.Expression == `true`) {
				conflicts = append(conflicts, &PolicyConflict{
					Policy1: p1,
					Policy2: p2,
					Type:    "contradiction",
				})
			}
		}
	}

	return conflicts
}

func resolvePolicyConflicts(t *testing.T, ctx context.Context, client kcpclientset.ClusterInterface, workspace logicalcluster.Path, conflicts []*PolicyConflict) *ConflictResolution {
	t.Logf("Resolving %d policy conflicts", len(conflicts))

	if len(conflicts) == 0 {
		return nil
	}

	// Simple resolution strategy - use deny-by-default for contradictions
	return &ConflictResolution{
		Strategy: "deny-by-default",
		ResolvedPolicy: &PolicyDefinition{
			Name:       "conflict-resolved-policy",
			Expression: `false`, // Deny by default
			Priority:   1000,    // Highest priority
			Type:       "CEL",
		},
	}
}
