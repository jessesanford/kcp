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
	"sort"
	"sync"

	"github.com/kcp-dev/logicalcluster/v3"
)

// DeploymentNode represents a deployment in the dependency graph.
// It contains the deployment metadata and its relationships.
type DeploymentNode struct {
	// ID is the unique identifier for the deployment
	ID string
	
	// Name is the deployment name
	Name string
	
	// Workspace is the logical cluster where the deployment exists
	Workspace logicalcluster.Name
	
	// Dependencies are the deployments this deployment depends on
	Dependencies []string
	
	// Dependents are the deployments that depend on this deployment
	Dependents []string
	
	// Status represents the current state of the deployment
	Status DeploymentStatus
	
	// Metadata stores additional deployment metadata
	Metadata map[string]string
}

// DeploymentStatus represents the current state of a deployment
type DeploymentStatus string

const (
	// StatusPending indicates the deployment is waiting to be processed
	StatusPending DeploymentStatus = "Pending"
	
	// StatusInProgress indicates the deployment is currently being processed
	StatusInProgress DeploymentStatus = "InProgress"
	
	// StatusCompleted indicates the deployment has completed successfully
	StatusCompleted DeploymentStatus = "Completed"
	
	// StatusFailed indicates the deployment has failed
	StatusFailed DeploymentStatus = "Failed"
	
	// StatusBlocked indicates the deployment is blocked by dependencies
	StatusBlocked DeploymentStatus = "Blocked"
)

// DependencyGraph represents a directed acyclic graph of deployment dependencies.
// It provides thread-safe operations for managing deployment relationships.
type DependencyGraph struct {
	// mu protects the graph data structures
	mu sync.RWMutex
	
	// nodes stores all deployment nodes indexed by their ID
	nodes map[string]*DeploymentNode
	
	// adjacencyList represents the directed edges (dependencies)
	// adjacencyList[from] contains all nodes that 'from' depends on
	adjacencyList map[string][]string
	
	// reverseAdjacencyList represents reverse edges (dependents)
	// reverseAdjacencyList[to] contains all nodes that depend on 'to'
	reverseAdjacencyList map[string][]string
}

// NewDependencyGraph creates a new empty dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes:                make(map[string]*DeploymentNode),
		adjacencyList:        make(map[string][]string),
		reverseAdjacencyList: make(map[string][]string),
	}
}

// AddNode adds a deployment node to the graph.
// Returns an error if a node with the same ID already exists.
func (g *DependencyGraph) AddNode(node *DeploymentNode) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}
	
	if node.ID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}
	
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if _, exists := g.nodes[node.ID]; exists {
		return fmt.Errorf("node with ID %s already exists", node.ID)
	}
	
	// Initialize slices if nil
	if node.Dependencies == nil {
		node.Dependencies = []string{}
	}
	if node.Dependents == nil {
		node.Dependents = []string{}
	}
	if node.Metadata == nil {
		node.Metadata = make(map[string]string)
	}
	
	g.nodes[node.ID] = node
	g.adjacencyList[node.ID] = make([]string, 0)
	g.reverseAdjacencyList[node.ID] = make([]string, 0)
	
	return nil
}

// RemoveNode removes a deployment node and all its edges from the graph.
func (g *DependencyGraph) RemoveNode(nodeID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if _, exists := g.nodes[nodeID]; !exists {
		return fmt.Errorf("node with ID %s does not exist", nodeID)
	}
	
	// Remove all edges involving this node
	for _, depID := range g.adjacencyList[nodeID] {
		g.removeEdgeUnsafe(nodeID, depID)
	}
	
	for _, depID := range g.reverseAdjacencyList[nodeID] {
		g.removeEdgeUnsafe(depID, nodeID)
	}
	
	// Remove the node and its adjacency lists
	delete(g.nodes, nodeID)
	delete(g.adjacencyList, nodeID)
	delete(g.reverseAdjacencyList, nodeID)
	
	return nil
}

// AddEdge adds a dependency edge from 'from' node to 'to' node.
// This means 'from' depends on 'to'.
func (g *DependencyGraph) AddEdge(from, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	return g.addEdgeUnsafe(from, to)
}

// addEdgeUnsafe adds an edge without acquiring the mutex (internal use).
func (g *DependencyGraph) addEdgeUnsafe(from, to string) error {
	if from == to {
		return fmt.Errorf("self-dependency not allowed: %s", from)
	}
	
	fromNode, fromExists := g.nodes[from]
	toNode, toExists := g.nodes[to]
	
	if !fromExists {
		return fmt.Errorf("node with ID %s does not exist", from)
	}
	
	if !toExists {
		return fmt.Errorf("node with ID %s does not exist", to)
	}
	
	// Check if edge already exists
	for _, dep := range g.adjacencyList[from] {
		if dep == to {
			return nil // Edge already exists, no need to add
		}
	}
	
	// Add edge to adjacency lists
	g.adjacencyList[from] = append(g.adjacencyList[from], to)
	g.reverseAdjacencyList[to] = append(g.reverseAdjacencyList[to], from)
	
	// Update node dependency information
	fromNode.Dependencies = append(fromNode.Dependencies, to)
	toNode.Dependents = append(toNode.Dependents, from)
	
	// Sort for consistent ordering
	sort.Strings(fromNode.Dependencies)
	sort.Strings(toNode.Dependents)
	
	return nil
}

// RemoveEdge removes a dependency edge between two nodes.
func (g *DependencyGraph) RemoveEdge(from, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	return g.removeEdgeUnsafe(from, to)
}

// removeEdgeUnsafe removes an edge without acquiring the mutex (internal use).
func (g *DependencyGraph) removeEdgeUnsafe(from, to string) error {
	fromNode, fromExists := g.nodes[from]
	toNode, toExists := g.nodes[to]
	
	if !fromExists || !toExists {
		return nil // If nodes don't exist, edge doesn't exist
	}
	
	// Remove from adjacency lists
	g.adjacencyList[from] = removeFromSlice(g.adjacencyList[from], to)
	g.reverseAdjacencyList[to] = removeFromSlice(g.reverseAdjacencyList[to], from)
	
	// Update node dependency information
	fromNode.Dependencies = removeFromSlice(fromNode.Dependencies, to)
	toNode.Dependents = removeFromSlice(toNode.Dependents, from)
	
	return nil
}

// GetNode returns a deployment node by its ID.
func (g *DependencyGraph) GetNode(nodeID string) (*DeploymentNode, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	node, exists := g.nodes[nodeID]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent external modification
	return g.copyNode(node), true
}

// GetAllNodes returns all nodes in the graph.
func (g *DependencyGraph) GetAllNodes() []*DeploymentNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	nodes := make([]*DeploymentNode, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, g.copyNode(node))
	}
	
	return nodes
}

// GetDependencies returns the direct dependencies of a node.
func (g *DependencyGraph) GetDependencies(nodeID string) ([]string, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	deps, exists := g.adjacencyList[nodeID]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent external modification
	result := make([]string, len(deps))
	copy(result, deps)
	return result, true
}

// GetDependents returns the nodes that depend on the given node.
func (g *DependencyGraph) GetDependents(nodeID string) ([]string, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	deps, exists := g.reverseAdjacencyList[nodeID]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent external modification
	result := make([]string, len(deps))
	copy(result, deps)
	return result, true
}

// NodeCount returns the total number of nodes in the graph.
func (g *DependencyGraph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	return len(g.nodes)
}

// EdgeCount returns the total number of edges in the graph.
func (g *DependencyGraph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	count := 0
	for _, deps := range g.adjacencyList {
		count += len(deps)
	}
	return count
}

// UpdateNodeStatus updates the status of a deployment node.
func (g *DependencyGraph) UpdateNodeStatus(nodeID string, status DeploymentStatus) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	node, exists := g.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node with ID %s does not exist", nodeID)
	}
	
	node.Status = status
	return nil
}

// copyNode creates a deep copy of a deployment node.
func (g *DependencyGraph) copyNode(node *DeploymentNode) *DeploymentNode {
	if node == nil {
		return nil
	}
	
	copy := &DeploymentNode{
		ID:        node.ID,
		Name:      node.Name,
		Workspace: node.Workspace,
		Status:    node.Status,
		Metadata:  make(map[string]string),
	}
	
	// Copy dependencies
	if node.Dependencies != nil {
		copy.Dependencies = make([]string, len(node.Dependencies))
		copy(copy.Dependencies, node.Dependencies)
	}
	
	// Copy dependents
	if node.Dependents != nil {
		copy.Dependents = make([]string, len(node.Dependents))
		copy(copy.Dependents, node.Dependents)
	}
	
	// Copy metadata
	for k, v := range node.Metadata {
		copy.Metadata[k] = v
	}
	
	return copy
}

// removeFromSlice removes a string from a slice and returns the modified slice.
func removeFromSlice(slice []string, item string) []string {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}