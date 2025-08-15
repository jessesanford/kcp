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
)

// TopologicalOrder represents the result of a topological sort operation.
type TopologicalOrder struct {
	// Order contains the nodes in topological order
	Order []string
	
	// HasCycle indicates whether a cycle was detected
	HasCycle bool
	
	// CyclePath contains nodes forming a cycle (if HasCycle is true)
	CyclePath []string
}

// TopologicalSort performs a topological sort on the dependency graph using Kahn's algorithm.
// Returns a TopologicalOrder containing the sorted nodes or cycle information.
func (g *DependencyGraph) TopologicalSort() *TopologicalOrder {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	// Calculate in-degree for each node
	inDegree := make(map[string]int)
	for nodeID := range g.nodes {
		inDegree[nodeID] = len(g.adjacencyList[nodeID])
	}
	
	// Queue for nodes with in-degree 0
	queue := []string{}
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}
	
	result := []string{}
	
	// Process nodes with in-degree 0
	for len(queue) > 0 {
		// Dequeue a node
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		
		// For each dependent of the current node
		for _, dependent := range g.reverseAdjacencyList[current] {
			// Decrease the in-degree
			inDegree[dependent]--
			
			// If in-degree becomes 0, add to queue
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}
	
	// Check if all nodes were processed
	if len(result) != len(g.nodes) {
		// Cycle detected - find the cycle path
		cyclePath := g.findCyclePathUnsafe()
		return &TopologicalOrder{
			Order:     nil,
			HasCycle:  true,
			CyclePath: cyclePath,
		}
	}
	
	return &TopologicalOrder{
		Order:     result,
		HasCycle:  false,
		CyclePath: nil,
	}
}

// findCyclePathUnsafe finds a cycle in the graph using DFS (internal use, assumes lock held).
func (g *DependencyGraph) findCyclePathUnsafe() []string {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	parent := make(map[string]string)
	
	// Try DFS from each unvisited node
	for nodeID := range g.nodes {
		if !visited[nodeID] {
			if cycle := g.dfsForCycleUnsafe(nodeID, visited, recStack, parent); cycle != nil {
				return cycle
			}
		}
	}
	
	return []string{} // No cycle found (shouldn't happen if we got here)
}

// dfsForCycleUnsafe performs DFS to find a cycle starting from the given node.
func (g *DependencyGraph) dfsForCycleUnsafe(nodeID string, visited, recStack map[string]bool, parent map[string]string) []string {
	visited[nodeID] = true
	recStack[nodeID] = true
	
	// Visit all dependencies
	for _, depID := range g.adjacencyList[nodeID] {
		parent[depID] = nodeID
		
		if !visited[depID] {
			// Continue DFS
			if cycle := g.dfsForCycleUnsafe(depID, visited, recStack, parent); cycle != nil {
				return cycle
			}
		} else if recStack[depID] {
			// Back edge found - construct cycle path
			return g.constructCyclePath(depID, nodeID, parent)
		}
	}
	
	recStack[nodeID] = false
	return nil
}

// constructCyclePath constructs the cycle path from start to end using parent map.
func (g *DependencyGraph) constructCyclePath(start, end string, parent map[string]string) []string {
	cycle := []string{start}
	current := end
	
	// Trace back through parents until we reach the start again
	for current != start {
		cycle = append([]string{current}, cycle...)
		current = parent[current]
	}
	
	return cycle
}

// GetExecutionOrder returns the nodes grouped by execution levels.
// Nodes in the same level can be executed in parallel.
func (g *DependencyGraph) GetExecutionOrder() (*ExecutionPlan, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	// First check for cycles
	topoResult := g.TopologicalSort()
	if topoResult.HasCycle {
		return nil, fmt.Errorf("cannot create execution plan: cycle detected involving nodes %v", topoResult.CyclePath)
	}
	
	// Calculate levels for each node
	levels := make(map[string]int)
	maxLevel := 0
	
	// Initialize all nodes at level 0
	for nodeID := range g.nodes {
		levels[nodeID] = 0
	}
	
	// Calculate the maximum level for each node based on dependencies
	for _, nodeID := range topoResult.Order {
		currentLevel := 0
		
		// Find the maximum level of all dependencies
		for _, depID := range g.adjacencyList[nodeID] {
			if levels[depID]+1 > currentLevel {
				currentLevel = levels[depID] + 1
			}
		}
		
		levels[nodeID] = currentLevel
		if currentLevel > maxLevel {
			maxLevel = currentLevel
		}
	}
	
	// Group nodes by level
	levelGroups := make([][]string, maxLevel+1)
	for nodeID, level := range levels {
		levelGroups[level] = append(levelGroups[level], nodeID)
	}
	
	// Create execution phases
	phases := make([]*ExecutionPhase, len(levelGroups))
	for i, group := range levelGroups {
		phases[i] = &ExecutionPhase{
			Level: i,
			Nodes: group,
		}
	}
	
	return &ExecutionPlan{
		Phases:      phases,
		TotalLevels: maxLevel + 1,
	}, nil
}

// ExecutionPlan represents a plan for executing deployments in dependency order.
type ExecutionPlan struct {
	// Phases contains the execution phases in order
	Phases []*ExecutionPhase
	
	// TotalLevels is the total number of execution levels
	TotalLevels int
}

// ExecutionPhase represents a single phase of execution where all nodes can run in parallel.
type ExecutionPhase struct {
	// Level is the execution level (0-based)
	Level int
	
	// Nodes are the deployment nodes that can be executed in parallel
	Nodes []string
}

// GetNodesAtLevel returns all nodes that should be executed at the given level.
func (plan *ExecutionPlan) GetNodesAtLevel(level int) ([]string, bool) {
	if level < 0 || level >= len(plan.Phases) {
		return nil, false
	}
	
	phase := plan.Phases[level]
	result := make([]string, len(phase.Nodes))
	copy(result, phase.Nodes)
	return result, true
}

// GetTotalNodes returns the total number of nodes in the execution plan.
func (plan *ExecutionPlan) GetTotalNodes() int {
	total := 0
	for _, phase := range plan.Phases {
		total += len(phase.Nodes)
	}
	return total
}

// IsValidExecutionOrder checks if the given order respects all dependencies.
func (g *DependencyGraph) IsValidExecutionOrder(order []string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	// Create position map
	position := make(map[string]int)
	for i, nodeID := range order {
		position[nodeID] = i
	}
	
	// Check that all dependencies come before dependents
	for nodeID := range g.nodes {
		nodePos, nodeExists := position[nodeID]
		if !nodeExists {
			return false // Node missing from order
		}
		
		// Check all dependencies
		for _, depID := range g.adjacencyList[nodeID] {
			depPos, depExists := position[depID]
			if !depExists {
				return false // Dependency missing from order
			}
			
			if depPos >= nodePos {
				return false // Dependency comes after or at same position
			}
		}
	}
	
	return len(order) == len(g.nodes) // Ensure all nodes are included
}