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
	"fmt"
	"strings"
)

// ValidationResult represents the result of dependency validation.
type ValidationResult struct {
	// IsValid indicates whether the dependency configuration is valid
	IsValid bool
	
	// Errors contains validation error messages
	Errors []string
	
	// Warnings contains validation warning messages
	Warnings []string
	
	// Cycles contains detected dependency cycles
	Cycles [][]string
}

// Validator provides comprehensive dependency validation capabilities.
type Validator struct {
	// AllowSelfReference controls whether self-references are allowed
	AllowSelfReference bool
	
	// MaxDependencyDepth limits the maximum depth of dependency chains
	MaxDependencyDepth int
	
	// RequiredFields specifies which node fields must be populated
	RequiredFields []string
}

// NewValidator creates a new dependency validator with default settings.
func NewValidator() *Validator {
	return &Validator{
		AllowSelfReference: false,
		MaxDependencyDepth: 50, // Reasonable default to prevent deep chains
		RequiredFields:     []string{"ID", "Name"},
	}
}

// Validate performs comprehensive validation of the dependency graph.
func (v *Validator) Validate(graph *DependencyGraph) *ValidationResult {
	result := &ValidationResult{
		IsValid:  true,
		Errors:   []string{},
		Warnings: []string{},
		Cycles:   [][]string{},
	}
	
	// Validate graph structure
	v.validateGraphStructure(graph, result)
	
	// Check for cycles
	v.detectCycles(graph, result)
	
	// Validate dependency depth
	v.validateDependencyDepth(graph, result)
	
	// Validate node completeness
	v.validateNodeCompleteness(graph, result)
	
	// Set overall validity
	result.IsValid = len(result.Errors) == 0
	
	return result
}

// validateGraphStructure validates basic graph structural integrity.
func (v *Validator) validateGraphStructure(graph *DependencyGraph, result *ValidationResult) {
	graph.mu.RLock()
	defer graph.mu.RUnlock()
	
	// Check for empty graphs
	if len(graph.nodes) == 0 {
		result.Warnings = append(result.Warnings, "Graph is empty")
		return
	}
	
	// Validate each node's dependencies exist
	for nodeID, node := range graph.nodes {
		// Check self-references
		for _, depID := range node.Dependencies {
			if depID == nodeID {
				if !v.AllowSelfReference {
					result.Errors = append(result.Errors, 
						fmt.Sprintf("Node %s has self-dependency which is not allowed", nodeID))
				} else {
					result.Warnings = append(result.Warnings, 
						fmt.Sprintf("Node %s has self-dependency", nodeID))
				}
			}
			
			// Check if dependency exists
			if _, exists := graph.nodes[depID]; !exists {
				result.Errors = append(result.Errors, 
					fmt.Sprintf("Node %s depends on non-existent node %s", nodeID, depID))
			}
		}
		
		// Validate adjacency list consistency
		if adjList, exists := graph.adjacencyList[nodeID]; exists {
			if len(adjList) != len(node.Dependencies) {
				result.Errors = append(result.Errors, 
					fmt.Sprintf("Node %s has inconsistent dependency list length", nodeID))
			}
		}
	}
}

// detectCycles identifies all cycles in the dependency graph.
func (v *Validator) detectCycles(graph *DependencyGraph, result *ValidationResult) {
	graph.mu.RLock()
	defer graph.mu.RUnlock()
	
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	parent := make(map[string]string)
	
	// Check for cycles starting from each node
	for nodeID := range graph.nodes {
		if !visited[nodeID] {
			cycles := v.findAllCyclesFromNode(graph, nodeID, visited, recStack, parent)
			result.Cycles = append(result.Cycles, cycles...)
		}
	}
	
	// Add cycle errors
	for _, cycle := range result.Cycles {
		result.Errors = append(result.Errors, 
			fmt.Sprintf("Circular dependency detected: %s", strings.Join(cycle, " -> ")))
	}
}

// findAllCyclesFromNode finds all cycles reachable from a given node.
func (v *Validator) findAllCyclesFromNode(graph *DependencyGraph, nodeID string, 
	visited, recStack map[string]bool, parent map[string]string) [][]string {
	
	visited[nodeID] = true
	recStack[nodeID] = true
	cycles := [][]string{}
	
	// Visit all dependencies
	for _, depID := range graph.adjacencyList[nodeID] {
		parent[depID] = nodeID
		
		if !visited[depID] {
			// Continue DFS
			subCycles := v.findAllCyclesFromNode(graph, depID, visited, recStack, parent)
			cycles = append(cycles, subCycles...)
		} else if recStack[depID] {
			// Back edge found - construct cycle
			cycle := v.constructCyclePath(depID, nodeID, parent)
			cycles = append(cycles, cycle)
		}
	}
	
	recStack[nodeID] = false
	return cycles
}

// constructCyclePath builds the cycle path from start to end.
func (v *Validator) constructCyclePath(start, end string, parent map[string]string) []string {
	cycle := []string{start}
	current := end
	
	for current != start && current != "" {
		cycle = append([]string{current}, cycle...)
		current = parent[current]
	}
	
	// Complete the cycle
	cycle = append(cycle, start)
	return cycle
}

// validateDependencyDepth checks for excessively deep dependency chains.
func (v *Validator) validateDependencyDepth(graph *DependencyGraph, result *ValidationResult) {
	graph.mu.RLock()
	defer graph.mu.RUnlock()
	
	for nodeID := range graph.nodes {
		depth := v.calculateMaxDepth(graph, nodeID, make(map[string]bool))
		if depth > v.MaxDependencyDepth {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("Node %s has dependency chain depth %d (max: %d)", 
					nodeID, depth, v.MaxDependencyDepth))
		}
	}
}

// calculateMaxDepth calculates the maximum dependency depth for a node.
func (v *Validator) calculateMaxDepth(graph *DependencyGraph, nodeID string, visiting map[string]bool) int {
	// Prevent infinite recursion in case of cycles
	if visiting[nodeID] {
		return 0
	}
	
	visiting[nodeID] = true
	defer func() { visiting[nodeID] = false }()
	
	maxDepth := 0
	for _, depID := range graph.adjacencyList[nodeID] {
		depth := v.calculateMaxDepth(graph, depID, visiting)
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	
	return maxDepth + 1
}

// validateNodeCompleteness ensures all nodes have required fields.
func (v *Validator) validateNodeCompleteness(graph *DependencyGraph, result *ValidationResult) {
	graph.mu.RLock()
	defer graph.mu.RUnlock()
	
	for nodeID, node := range graph.nodes {
		for _, field := range v.RequiredFields {
			switch field {
			case "ID":
				if node.ID == "" {
					result.Errors = append(result.Errors, 
						fmt.Sprintf("Node %s is missing required field: ID", nodeID))
				}
			case "Name":
				if node.Name == "" {
					result.Errors = append(result.Errors, 
						fmt.Sprintf("Node %s is missing required field: Name", nodeID))
				}
			case "Workspace":
				if string(node.Workspace) == "" {
					result.Errors = append(result.Errors, 
						fmt.Sprintf("Node %s is missing required field: Workspace", nodeID))
				}
			}
		}
	}
}

// ValidateNodeAddition checks if adding a node would create issues.
func (v *Validator) ValidateNodeAddition(graph *DependencyGraph, node *DeploymentNode) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}
	
	if node.ID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}
	
	// Check if node already exists
	if _, exists := graph.GetNode(node.ID); exists {
		return fmt.Errorf("node with ID %s already exists", node.ID)
	}
	
	// Validate required fields
	for _, field := range v.RequiredFields {
		switch field {
		case "Name":
			if node.Name == "" {
				return fmt.Errorf("node is missing required field: Name")
			}
		}
	}
	
	return nil
}

// ValidateEdgeAddition checks if adding an edge would create a cycle.
func (v *Validator) ValidateEdgeAddition(graph *DependencyGraph, from, to string) error {
	// Check if nodes exist
	if _, exists := graph.GetNode(from); !exists {
		return fmt.Errorf("source node %s does not exist", from)
	}
	
	if _, exists := graph.GetNode(to); !exists {
		return fmt.Errorf("target node %s does not exist", to)
	}
	
	// Check for self-reference
	if from == to && !v.AllowSelfReference {
		return fmt.Errorf("self-dependency not allowed: %s -> %s", from, to)
	}
	
	// Create a temporary copy and test the edge
	// This is a simplified approach - in practice, we'd do a more efficient check
	graph.mu.RLock()
	defer graph.mu.RUnlock()
	
	// Check if adding this edge would create a path from 'to' back to 'from'
	if v.hasPath(graph, to, from, make(map[string]bool)) {
		return fmt.Errorf("adding edge %s -> %s would create a cycle", from, to)
	}
	
	return nil
}

// hasPath checks if there's a path from source to target using DFS.
func (v *Validator) hasPath(graph *DependencyGraph, source, target string, visited map[string]bool) bool {
	if source == target {
		return true
	}
	
	if visited[source] {
		return false
	}
	
	visited[source] = true
	
	// Check all dependencies of the source
	for _, depID := range graph.adjacencyList[source] {
		if v.hasPath(graph, depID, target, visited) {
			return true
		}
	}
	
	return false
}