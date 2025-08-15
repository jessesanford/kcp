package interfaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/kcp/pkg/placement/interfaces"
	"github.com/kcp-dev/kcp/sdk/apis/core"
)

func TestPlacementDecisionValidation(t *testing.T) {
	timestamp := metav1.Now()
	tests := map[string]struct {
		decision *interfaces.PlacementDecision
		validate func(*testing.T, *interfaces.PlacementDecision)
	}{
		"valid placement decision": {
			decision: &interfaces.PlacementDecision{
				TargetClusters: []interfaces.ScoredTarget{
					{
						ClusterTarget: interfaces.ClusterTarget{
							Name:      "cluster-1",
							Workspace: core.LogicalCluster("root:org:prod"),
							Ready:     true,
						},
						Score: 100,
					},
				},
				DecisionID: "test-decision-123",
				Timestamp:  timestamp,
			},
			validate: func(t *testing.T, decision *interfaces.PlacementDecision) {
				assert.NotNil(t, decision)
				assert.Len(t, decision.TargetClusters, 1)
				assert.Equal(t, "cluster-1", decision.TargetClusters[0].Name)
				assert.Equal(t, int32(100), decision.TargetClusters[0].Score)
				assert.Equal(t, "test-decision-123", decision.DecisionID)
			},
		},
		"empty decision": {
			decision: &interfaces.PlacementDecision{
				TargetClusters: []interfaces.ScoredTarget{},
				Timestamp:      timestamp,
			},
			validate: func(t *testing.T, decision *interfaces.PlacementDecision) {
				assert.NotNil(t, decision)
				assert.Len(t, decision.TargetClusters, 0)
			},
		},
		"decision with policy evaluations": {
			decision: &interfaces.PlacementDecision{
				TargetClusters: []interfaces.ScoredTarget{
					{
						ClusterTarget: interfaces.ClusterTarget{
							Name: "cluster-1",
						},
						Score: 85,
					},
				},
				PolicyEvaluations: []interfaces.PolicyResult{
					{
						PolicyName: "data-residency",
						Passed:     true,
						Score:      90,
						Message:    "Cluster meets data residency requirements",
					},
				},
				Timestamp: timestamp,
			},
			validate: func(t *testing.T, decision *interfaces.PlacementDecision) {
				require.Len(t, decision.PolicyEvaluations, 1)
				assert.Equal(t, "data-residency", decision.PolicyEvaluations[0].PolicyName)
				assert.True(t, decision.PolicyEvaluations[0].Passed)
				assert.Equal(t, int32(90), decision.PolicyEvaluations[0].Score)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.validate(t, tc.decision)
		})
	}
}

func TestClusterTargetFields(t *testing.T) {
	tests := map[string]struct {
		target   interfaces.ClusterTarget
		validate func(*testing.T, interfaces.ClusterTarget)
	}{
		"basic cluster target": {
			target: interfaces.ClusterTarget{
				Name:      "test-cluster",
				Workspace: core.LogicalCluster("root:org:test"),
				Ready:     true,
				Labels: map[string]string{
					"region": "us-west",
					"zone":   "us-west-1a",
				},
				Capacity: interfaces.ResourceCapacity{
					CPU:    "4",
					Memory: "8Gi",
					Pods:   110,
				},
			},
			validate: func(t *testing.T, target interfaces.ClusterTarget) {
				assert.Equal(t, "test-cluster", target.Name)
				assert.Equal(t, core.LogicalCluster("root:org:test"), target.Workspace)
				assert.True(t, target.Ready)
				assert.Equal(t, "us-west", target.Labels["region"])
				assert.Equal(t, "us-west-1a", target.Labels["zone"])
				assert.Equal(t, "4", target.Capacity.CPU)
				assert.Equal(t, "8Gi", target.Capacity.Memory)
				assert.Equal(t, int32(110), target.Capacity.Pods)
			},
		},
		"cluster with annotations": {
			target: interfaces.ClusterTarget{
				Name: "annotated-cluster",
				Annotations: map[string]string{
					"cluster.kcp.io/cost-center": "engineering",
					"cluster.kcp.io/owner":       "team-alpha",
				},
			},
			validate: func(t *testing.T, target interfaces.ClusterTarget) {
				assert.Equal(t, "engineering", target.Annotations["cluster.kcp.io/cost-center"])
				assert.Equal(t, "team-alpha", target.Annotations["cluster.kcp.io/owner"])
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.validate(t, tc.target)
		})
	}
}

func TestScoredTarget(t *testing.T) {
	scored := interfaces.ScoredTarget{
		ClusterTarget: interfaces.ClusterTarget{
			Name:      "high-score-cluster",
			Workspace: core.LogicalCluster("root:prod"),
			Ready:     true,
		},
		Score: 95,
		Reasons: []string{
			"Excellent CPU availability",
			"Low network latency",
			"Matches data residency policy",
		},
	}

	assert.Equal(t, "high-score-cluster", scored.Name)
	assert.Equal(t, int32(95), scored.Score)
	assert.Len(t, scored.Reasons, 3)
	assert.Contains(t, scored.Reasons, "Excellent CPU availability")
}

func TestPlacementPolicy(t *testing.T) {
	tests := map[string]struct {
		policy   interfaces.PlacementPolicy
		validate func(*testing.T, interfaces.PlacementPolicy)
	}{
		"basic policy": {
			policy: interfaces.PlacementPolicy{
				Name:     "data-residency",
				Priority: 100,
				Required: true,
				Rules: []interfaces.PolicyRule{
					{
						Expression: "cluster.region == 'us-west'",
						Type:       interfaces.PolicyRuleConstraint,
						Weight:     100,
					},
				},
			},
			validate: func(t *testing.T, policy interfaces.PlacementPolicy) {
				assert.Equal(t, "data-residency", policy.Name)
				assert.Equal(t, int32(100), policy.Priority)
				assert.True(t, policy.Required)
				require.Len(t, policy.Rules, 1)
				assert.Equal(t, "cluster.region == 'us-west'", policy.Rules[0].Expression)
				assert.Equal(t, interfaces.PolicyRuleConstraint, policy.Rules[0].Type)
			},
		},
		"preference policy": {
			policy: interfaces.PlacementPolicy{
				Name:     "cost-optimization",
				Priority: 50,
				Required: false,
				Rules: []interfaces.PolicyRule{
					{
						Expression: "cluster.labels['cost-tier'] == 'low'",
						Type:       interfaces.PolicyRulePreference,
						Weight:     80,
					},
					{
						Expression: "cluster.capacity.cpu > '2'",
						Type:       interfaces.PolicyRulePreference,
						Weight:     60,
					},
				},
			},
			validate: func(t *testing.T, policy interfaces.PlacementPolicy) {
				assert.Equal(t, "cost-optimization", policy.Name)
				assert.False(t, policy.Required)
				require.Len(t, policy.Rules, 2)
				assert.Equal(t, interfaces.PolicyRulePreference, policy.Rules[0].Type)
				assert.Equal(t, int32(80), policy.Rules[0].Weight)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.validate(t, tc.policy)
		})
	}
}

func TestSchedulingResult(t *testing.T) {
	result := interfaces.SchedulingResult{
		Algorithm:         "binpack",
		Duration:          time.Millisecond * 150,
		Iterations:        3,
		ClustersEvaluated: 25,
	}

	assert.Equal(t, "binpack", result.Algorithm)
	assert.Equal(t, time.Millisecond*150, result.Duration)
	assert.Equal(t, 3, result.Iterations)
	assert.Equal(t, 25, result.ClustersEvaluated)
}

func TestWorkspaceInfo(t *testing.T) {
	parent := core.LogicalCluster("root:org")
	workspace := interfaces.WorkspaceInfo{
		Name:   core.LogicalCluster("root:org:team-a"),
		Parent: &parent,
		Labels: map[string]string{
			"team":        "alpha",
			"environment": "production",
		},
		Ready: true,
	}

	assert.Equal(t, core.LogicalCluster("root:org:team-a"), workspace.Name)
	assert.Equal(t, core.LogicalCluster("root:org"), *workspace.Parent)
	assert.Equal(t, "alpha", workspace.Labels["team"])
	assert.True(t, workspace.Ready)
}

func TestPolicyResult(t *testing.T) {
	result := interfaces.PolicyResult{
		PolicyName: "security-compliance",
		Passed:     true,
		Score:      88,
		Message:    "All security checks passed",
		RuleResults: []interfaces.RuleResult{
			{
				Expression: "cluster.labels['security-scan'] == 'passed'",
				Passed:     true,
				Score:      90,
			},
			{
				Expression: "cluster.labels['compliance-status'] == 'verified'",
				Passed:     true,
				Score:      85,
			},
		},
	}

	assert.Equal(t, "security-compliance", result.PolicyName)
	assert.True(t, result.Passed)
	assert.Equal(t, int32(88), result.Score)
	assert.Equal(t, "All security checks passed", result.Message)
	require.Len(t, result.RuleResults, 2)
	assert.True(t, result.RuleResults[0].Passed)
	assert.Equal(t, int32(90), result.RuleResults[0].Score)
}

func TestSchedulerOptions(t *testing.T) {
	options := interfaces.SchedulerOptions{
		Strategy: "binpack",
		Weights: map[string]float64{
			"cpu":    0.7,
			"memory": 0.6,
			"cost":   0.4,
		},
		Constraints: []string{
			"region == us-west",
			"availability-zone != us-west-1c",
		},
		MaxClustersPerPlacement: 5,
		EnablePreemption:        true,
		AffinityRules: []interfaces.AffinityRule{
			{
				Type: interfaces.AffinityTypeRepulsion,
				Selector: map[string]string{
					"app": "database",
				},
				Weight:   80,
				Required: false,
			},
		},
	}

	assert.Equal(t, "binpack", options.Strategy)
	assert.Equal(t, 0.7, options.Weights["cpu"])
	assert.Equal(t, 0.6, options.Weights["memory"])
	assert.Len(t, options.Constraints, 2)
	assert.Equal(t, 5, options.MaxClustersPerPlacement)
	assert.True(t, options.EnablePreemption)
	require.Len(t, options.AffinityRules, 1)
	assert.Equal(t, interfaces.AffinityTypeRepulsion, options.AffinityRules[0].Type)
}

func TestAffinityRule(t *testing.T) {
	rule := interfaces.AffinityRule{
		Type: interfaces.AffinityTypeAttraction,
		Selector: map[string]string{
			"component": "frontend",
			"tier":      "web",
		},
		Weight:   90,
		Required: true,
	}

	assert.Equal(t, interfaces.AffinityTypeAttraction, rule.Type)
	assert.Equal(t, "frontend", rule.Selector["component"])
	assert.Equal(t, "web", rule.Selector["tier"])
	assert.Equal(t, int32(90), rule.Weight)
	assert.True(t, rule.Required)
}

func TestPlacementContext(t *testing.T) {
	timestamp := metav1.Now()
	context := interfaces.PlacementContext{
		User:      "alice@example.com",
		Groups:    []string{"developers", "team-alpha"},
		RequestID: "req-12345",
		Timestamp: timestamp,
		Variables: map[string]interface{}{
			"deployment_priority": "high",
			"cost_limit":          100.0,
		},
	}

	assert.Equal(t, "alice@example.com", context.User)
	assert.Contains(t, context.Groups, "developers")
	assert.Contains(t, context.Groups, "team-alpha")
	assert.Equal(t, "req-12345", context.RequestID)
	assert.Equal(t, timestamp, context.Timestamp)
	assert.Equal(t, "high", context.Variables["deployment_priority"])
	assert.Equal(t, 100.0, context.Variables["cost_limit"])
}

func TestEngineStatus(t *testing.T) {
	status := interfaces.EngineStatus{
		Ready:                 true,
		ActivePlacements:      3,
		TotalPlacements:       1250,
		AverageProcessingTime: time.Millisecond * 200,
		LastError:             "",
		ComponentStatuses: map[string]interfaces.ComponentStatus{
			"workspace-discovery": {
				Name:         "workspace-discovery",
				Ready:        true,
				Message:      "All workspace connections healthy",
				LastActivity: time.Now(),
			},
			"policy-evaluator": {
				Name:         "policy-evaluator",
				Ready:        true,
				Message:      "CEL evaluator operational",
				LastActivity: time.Now(),
			},
		},
		Timestamp: time.Now(),
	}

	assert.True(t, status.Ready)
	assert.Equal(t, int32(3), status.ActivePlacements)
	assert.Equal(t, int64(1250), status.TotalPlacements)
	assert.Equal(t, time.Millisecond*200, status.AverageProcessingTime)
	assert.Empty(t, status.LastError)
	assert.Len(t, status.ComponentStatuses, 2)
	assert.True(t, status.ComponentStatuses["workspace-discovery"].Ready)
	assert.Equal(t, "CEL evaluator operational", status.ComponentStatuses["policy-evaluator"].Message)
}