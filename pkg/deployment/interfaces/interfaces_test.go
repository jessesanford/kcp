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

func TestStrategyTypeValidation(t *testing.T) {
	strategy := types.DeploymentStrategy{
		Type: types.CanaryStrategyType,
		Canary: &types.CanaryStrategy{
			Steps: []types.CanaryStep{
				{Weight: 10},
				{Weight: 50},
				{Weight: 100},
			},
		},
	}

	assert.Equal(t, types.CanaryStrategyType, strategy.Type)
	assert.Len(t, strategy.Canary.Steps, 3)
	assert.Equal(t, int32(10), strategy.Canary.Steps[0].Weight)
	assert.Equal(t, int32(100), strategy.Canary.Steps[2].Weight)
}

func TestDependencyGraphConstruction(t *testing.T) {
	graph := &types.DependencyGraph{
		Nodes: make(map[string]*types.DependencyNode),
		Edges: []types.DependencyEdge{},
	}

	// Add nodes
	graph.Nodes["app1"] = &types.DependencyNode{
		ID:     "app1",
		Status: types.DependencyPending,
	}

	graph.Nodes["app2"] = &types.DependencyNode{
		ID:     "app2",
		Status: types.DependencyPending,
	}

	// Add edge
	graph.Edges = append(graph.Edges, types.DependencyEdge{
		From: "app1",
		To:   "app2",
		Type: types.HardDependency,
	})

	assert.Len(t, graph.Nodes, 2)
	assert.Len(t, graph.Edges, 1)
	assert.Equal(t, "app1", graph.Edges[0].From)
	assert.Equal(t, "app2", graph.Edges[0].To)
	assert.Equal(t, types.HardDependency, graph.Edges[0].Type)
}

func TestBlueGreenStrategyConfiguration(t *testing.T) {
	strategy := types.DeploymentStrategy{
		Type: types.BlueGreenStrategyType,
		BlueGreen: &types.BlueGreenStrategy{
			AutoPromotionEnabled: true,
		},
	}

	assert.Equal(t, types.BlueGreenStrategyType, strategy.Type)
	assert.True(t, strategy.BlueGreen.AutoPromotionEnabled)
	assert.Nil(t, strategy.BlueGreen.PrePromotionAnalysis)
}

func TestDependencyTypes(t *testing.T) {
	tests := []struct {
		name         string
		depType      types.DependencyType
		expectedType types.DependencyType
	}{
		{
			name:         "hard dependency",
			depType:      types.HardDependency,
			expectedType: types.HardDependency,
		},
		{
			name:         "soft dependency",
			depType:      types.SoftDependency,
			expectedType: types.SoftDependency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependency := types.Dependency{
				Name: "test-dep",
				Type: tt.depType,
				Target: types.DependencyTarget{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       "test-service",
				},
			}

			assert.Equal(t, tt.expectedType, dependency.Type)
			assert.Equal(t, "test-dep", dependency.Name)
		})
	}
}

// TestDeploymentTarget tests the DeploymentTarget struct
func TestDeploymentTarget(t *testing.T) {
	target := interfaces.DeploymentTarget{
		Name:       "test-app",
		Namespace:  "default",
		Workspace:  "root:test",
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Labels: map[string]string{
			"app": "test",
			"env": "prod",
		},
	}

	assert.Equal(t, "test-app", target.Name)
	assert.Equal(t, "default", target.Namespace)
	assert.Equal(t, "root:test", target.Workspace)
	assert.Equal(t, "apps/v1", target.APIVersion)
	assert.Equal(t, "Deployment", target.Kind)
	assert.Len(t, target.Labels, 2)
	assert.Equal(t, "test", target.Labels["app"])
	assert.Equal(t, "prod", target.Labels["env"])
}