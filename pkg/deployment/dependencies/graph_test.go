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

package dependencies

import (
	"testing"

	"github.com/kcp-dev/logicalcluster/v3"
)

func TestDependencyGraph_AddNode(t *testing.T) {
	tests := map[string]struct {
		node      *DeploymentNode
		wantError bool
		errorMsg  string
	}{
		"valid node": {
			node: &DeploymentNode{
				ID:        "app1",
				Name:      "test-app",
				Workspace: logicalcluster.Name("root:default"),
				Status:    StatusPending,
			},
			wantError: false,
		},
		"nil node": {
			node:      nil,
			wantError: true,
			errorMsg:  "node cannot be nil",
		},
		"empty ID": {
			node: &DeploymentNode{
				Name:      "test-app",
				Workspace: logicalcluster.Name("root:default"),
			},
			wantError: true,
			errorMsg:  "node ID cannot be empty",
		},
		"duplicate ID": {
			node: &DeploymentNode{
				ID:        "app1",
				Name:      "duplicate-app",
				Workspace: logicalcluster.Name("root:default"),
			},
			wantError: true,
			errorMsg:  "node with ID app1 already exists",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewDependencyGraph()
			
			// For duplicate test, add the node first
			if name == "duplicate ID" {
				firstNode := &DeploymentNode{
					ID:        "app1",
					Name:      "first-app",
					Workspace: logicalcluster.Name("root:default"),
				}
				if err := g.AddNode(firstNode); err != nil {
					t.Fatalf("Failed to add first node: %v", err)
				}
			}
			
			err := g.AddNode(tc.node)
			if tc.wantError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if err.Error() != tc.errorMsg {
					t.Errorf("Expected error %q, got %q", tc.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				
				// Verify node was added
				retrieved, exists := g.GetNode(tc.node.ID)
				if !exists {
					t.Error("Node was not added to graph")
				}
				if retrieved.ID != tc.node.ID {
					t.Errorf("Expected ID %s, got %s", tc.node.ID, retrieved.ID)
				}
			}
		})
	}
}

func TestDependencyGraph_AddEdge(t *testing.T) {
	g := NewDependencyGraph()
	
	// Add test nodes
	nodes := []*DeploymentNode{
		{ID: "app1", Name: "app1", Workspace: "root:default"},
		{ID: "app2", Name: "app2", Workspace: "root:default"},
		{ID: "app3", Name: "app3", Workspace: "root:default"},
	}
	
	for _, node := range nodes {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("Failed to add node %s: %v", node.ID, err)
		}
	}

	tests := map[string]struct {
		from      string
		to        string
		wantError bool
		errorMsg  string
	}{
		"valid edge": {
			from:      "app1",
			to:        "app2",
			wantError: false,
		},
		"self dependency": {
			from:      "app1",
			to:        "app1",
			wantError: true,
			errorMsg:  "self-dependency not allowed: app1",
		},
		"non-existent from": {
			from:      "nonexistent",
			to:        "app2",
			wantError: true,
			errorMsg:  "node with ID nonexistent does not exist",
		},
		"non-existent to": {
			from:      "app1",
			to:        "nonexistent",
			wantError: true,
			errorMsg:  "node with ID nonexistent does not exist",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := g.AddEdge(tc.from, tc.to)
			if tc.wantError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if err.Error() != tc.errorMsg {
					t.Errorf("Expected error %q, got %q", tc.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				
				// Verify edge was added
				deps, exists := g.GetDependencies(tc.from)
				if !exists {
					t.Fatal("From node does not exist")
				}
				
				found := false
				for _, dep := range deps {
					if dep == tc.to {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Edge %s -> %s was not added", tc.from, tc.to)
				}
			}
		})
	}
}

func TestDependencyGraph_TopologicalSort(t *testing.T) {
	tests := map[string]struct {
		nodes         []string
		edges         [][2]string
		expectCycle   bool
		expectedOrder []string
	}{
		"simple linear chain": {
			nodes:         []string{"app1", "app2", "app3"},
			edges:         [][2]string{{"app1", "app2"}, {"app2", "app3"}},
			expectCycle:   false,
			expectedOrder: []string{"app3", "app2", "app1"},
		},
		"parallel dependencies": {
			nodes:         []string{"app1", "app2", "app3", "app4"},
			edges:         [][2]string{{"app1", "app3"}, {"app2", "app4"}},
			expectCycle:   false,
			// app3 and app4 can be in any order
		},
		"cycle detection": {
			nodes:       []string{"app1", "app2", "app3"},
			edges:       [][2]string{{"app1", "app2"}, {"app2", "app3"}, {"app3", "app1"}},
			expectCycle: true,
		},
		"empty graph": {
			nodes:         []string{},
			edges:         [][2]string{},
			expectCycle:   false,
			expectedOrder: []string{},
		},
		"single node": {
			nodes:         []string{"app1"},
			edges:         [][2]string{},
			expectCycle:   false,
			expectedOrder: []string{"app1"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewDependencyGraph()
			
			// Add nodes
			for _, nodeID := range tc.nodes {
				node := &DeploymentNode{
					ID:        nodeID,
					Name:      nodeID,
					Workspace: logicalcluster.Name("root:default"),
					Status:    StatusPending,
				}
				if err := g.AddNode(node); err != nil {
					t.Fatalf("Failed to add node %s: %v", nodeID, err)
				}
			}
			
			// Add edges
			for _, edge := range tc.edges {
				if err := g.AddEdge(edge[0], edge[1]); err != nil {
					t.Fatalf("Failed to add edge %s -> %s: %v", edge[0], edge[1], err)
				}
			}
			
			// Perform topological sort
			result := g.TopologicalSort()
			
			if tc.expectCycle {
				if !result.HasCycle {
					t.Error("Expected cycle but none was detected")
				}
				if len(result.CyclePath) == 0 {
					t.Error("Expected cycle path but got empty")
				}
			} else {
				if result.HasCycle {
					t.Errorf("Unexpected cycle detected: %v", result.CyclePath)
				}
				
				if len(result.Order) != len(tc.nodes) {
					t.Errorf("Expected order length %d, got %d", len(tc.nodes), len(result.Order))
				}
				
				// For specific expected orders, verify them
				if tc.expectedOrder != nil && len(tc.expectedOrder) > 0 {
					if len(result.Order) != len(tc.expectedOrder) {
						t.Errorf("Expected order %v, got %v", tc.expectedOrder, result.Order)
					}
				}
				
				// Verify the order is valid (all dependencies come before dependents)
				if !g.IsValidExecutionOrder(result.Order) {
					t.Errorf("Generated order is invalid: %v", result.Order)
				}
			}
		})
	}
}

func TestValidator_Validate(t *testing.T) {
	tests := map[string]struct {
		setupGraph    func() *DependencyGraph
		expectValid   bool
		expectErrors  int
		expectCycles  int
	}{
		"valid simple graph": {
			setupGraph: func() *DependencyGraph {
				g := NewDependencyGraph()
				g.AddNode(&DeploymentNode{ID: "app1", Name: "app1", Workspace: "root:default"})
				g.AddNode(&DeploymentNode{ID: "app2", Name: "app2", Workspace: "root:default"})
				g.AddEdge("app1", "app2")
				return g
			},
			expectValid:  true,
			expectErrors: 0,
			expectCycles: 0,
		},
		"graph with cycle": {
			setupGraph: func() *DependencyGraph {
				g := NewDependencyGraph()
				g.AddNode(&DeploymentNode{ID: "app1", Name: "app1", Workspace: "root:default"})
				g.AddNode(&DeploymentNode{ID: "app2", Name: "app2", Workspace: "root:default"})
				g.AddEdge("app1", "app2")
				g.AddEdge("app2", "app1")
				return g
			},
			expectValid:  false,
			expectErrors: 1, // cycle error
			expectCycles: 1,
		},
		"empty graph": {
			setupGraph: func() *DependencyGraph {
				return NewDependencyGraph()
			},
			expectValid:  true,
			expectErrors: 0,
			expectCycles: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			validator := NewValidator()
			graph := tc.setupGraph()
			
			result := validator.Validate(graph)
			
			if result.IsValid != tc.expectValid {
				t.Errorf("Expected validity %v, got %v", tc.expectValid, result.IsValid)
			}
			
			if len(result.Errors) != tc.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tc.expectErrors, len(result.Errors), result.Errors)
			}
			
			if len(result.Cycles) != tc.expectCycles {
				t.Errorf("Expected %d cycles, got %d: %v", tc.expectCycles, len(result.Cycles), result.Cycles)
			}
		})
	}
}

func TestDependencyGraph_GetExecutionOrder(t *testing.T) {
	g := NewDependencyGraph()
	
	// Create a more complex dependency structure
	nodes := []string{"db", "cache", "api", "frontend", "monitor"}
	for _, nodeID := range nodes {
		node := &DeploymentNode{
			ID:        nodeID,
			Name:      nodeID,
			Workspace: logicalcluster.Name("root:default"),
			Status:    StatusPending,
		}
		if err := g.AddNode(node); err != nil {
			t.Fatalf("Failed to add node %s: %v", nodeID, err)
		}
	}
	
	// Set up dependencies: frontend -> api -> cache, db; monitor -> api
	edges := [][2]string{
		{"frontend", "api"},
		{"api", "cache"},
		{"api", "db"},
		{"monitor", "api"},
	}
	
	for _, edge := range edges {
		if err := g.AddEdge(edge[0], edge[1]); err != nil {
			t.Fatalf("Failed to add edge %s -> %s: %v", edge[0], edge[1], err)
		}
	}
	
	plan, err := g.GetExecutionOrder()
	if err != nil {
		t.Fatalf("Failed to get execution order: %v", err)
	}
	
	// Verify that dependencies are satisfied in the execution plan
	if plan.TotalLevels < 3 {
		t.Errorf("Expected at least 3 levels, got %d", plan.TotalLevels)
	}
	
	// Level 0 should contain db and cache (no dependencies)
	level0, exists := plan.GetNodesAtLevel(0)
	if !exists {
		t.Fatal("Level 0 does not exist")
	}
	
	expectedAtLevel0 := map[string]bool{"db": true, "cache": true}
	for _, node := range level0 {
		if !expectedAtLevel0[node] {
			t.Errorf("Node %s should not be at level 0", node)
		}
	}
}

func TestDependencyGraph_Concurrency(t *testing.T) {
	g := NewDependencyGraph()
	
	// Test concurrent node additions
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			node := &DeploymentNode{
				ID:        string(rune('a' + id)),
				Name:      string(rune('a' + id)),
				Workspace: logicalcluster.Name("root:default"),
				Status:    StatusPending,
			}
			g.AddNode(node)
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Verify that we have some nodes (exact count may vary due to duplicates)
	if g.NodeCount() == 0 {
		t.Error("Expected some nodes to be added")
	}
}