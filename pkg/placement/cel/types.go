/*
Copyright 2023 The KCP Authors.

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

package cel

import (
	"context"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kcp-dev/logicalcluster/v3"
)

// CELEvaluator provides the main interface for evaluating CEL expressions
// in the context of placement and workspace management.
type CELEvaluator interface {
	// CompileExpression compiles a CEL expression and returns a compiled expression
	// that can be efficiently evaluated multiple times
	CompileExpression(expr string) (*CompiledExpression, error)
	
	// EvaluatePlacement evaluates a placement expression against a placement context
	// Returns true if the workspace matches the criteria
	EvaluatePlacement(ctx context.Context, expr *CompiledExpression, placement *PlacementContext) (bool, error)
	
	// EvaluateWithVariables evaluates an expression with custom variable bindings
	// This is the most flexible evaluation method for advanced use cases
	EvaluateWithVariables(ctx context.Context, expr *CompiledExpression, vars map[string]interface{}) (interface{}, error)
	
	// RegisterCustomFunction registers a custom function for use in CEL expressions
	RegisterCustomFunction(name string, fn CustomFunction) error
	
	// GetEnvironment returns the CEL environment for advanced operations
	GetEnvironment() *cel.Env
}

// CompiledExpression represents a pre-compiled CEL expression ready for evaluation.
type CompiledExpression struct {
	// Expression is the original expression string
	Expression string
	
	// Program is the compiled CEL program
	Program cel.Program
	
	// CompiledAt tracks when this expression was compiled
	CompiledAt time.Time
	
	// Hash is the hash of the expression for caching
	Hash string
}

// PlacementContext provides the context information available to CEL expressions
// during placement evaluation.
type PlacementContext struct {
	// Workspace contains information about the target workspace
	Workspace *WorkspaceContext
	
	// Request contains information about the placement request
	Request *RequestContext
	
	// Resources contains resource capacity and utilization information
	Resources *ResourceContext
	
	// Variables contains custom variables for this evaluation
	Variables map[string]interface{}
}

// WorkspaceContext provides workspace-specific information for CEL evaluation.
type WorkspaceContext struct {
	// Name is the logical cluster name of the workspace
	Name logicalcluster.Name
	
	// Labels are the labels associated with this workspace
	Labels map[string]string
	
	// Annotations are the annotations associated with this workspace
	Annotations map[string]string
	
	// Ready indicates if the workspace is ready to accept placements
	Ready bool
	
	// LastHeartbeat is when the workspace last reported its status
	LastHeartbeat time.Time
	
	// Region is the geographical region of the workspace (if available)
	Region string
	
	// Zone is the availability zone of the workspace (if available)
	Zone string
}

// RequestContext provides placement request information for CEL evaluation.
type RequestContext struct {
	// Name is the name of the placement request
	Name string
	
	// Namespace is the namespace of the placement request
	Namespace string
	
	// SourceWorkspace is the workspace where the request originated
	SourceWorkspace logicalcluster.Name
	
	// Labels are the labels from the placement request
	Labels map[string]string
	
	// Requirements are the resource requirements
	Requirements *ResourceRequirements
	
	// Priority is the scheduling priority
	Priority int32
	
	// CreatedAt is when the request was created
	CreatedAt time.Time
}

// ResourceContext provides resource information for CEL evaluation.
type ResourceContext struct {
	// TotalCapacity is the total resource capacity
	TotalCapacity *ResourceCapacity
	
	// AvailableCapacity is the available resource capacity
	AvailableCapacity *ResourceCapacity
	
	// CurrentUtilization is the current resource utilization
	CurrentUtilization *ResourceUtilization
	
	// ReservedResources are resources that are reserved but not yet allocated
	ReservedResources *ResourceCapacity
}

// ResourceRequirements specifies the resource requirements for a placement.
type ResourceRequirements struct {
	// CPU is the CPU requirement
	CPU resource.Quantity
	
	// Memory is the memory requirement  
	Memory resource.Quantity
	
	// Storage is the storage requirement
	Storage resource.Quantity
	
	// CustomResources contains requirements for custom resources
	CustomResources map[string]resource.Quantity
}

// ResourceCapacity represents the resource capacity or availability.
type ResourceCapacity struct {
	// CPU is the CPU capacity
	CPU resource.Quantity
	
	// Memory is the memory capacity
	Memory resource.Quantity
	
	// Storage is the storage capacity
	Storage resource.Quantity
	
	// CustomResources contains capacity for custom resources
	CustomResources map[string]resource.Quantity
	
	// LastUpdated is when this information was last updated
	LastUpdated time.Time
}

// ResourceUtilization represents current resource utilization.
type ResourceUtilization struct {
	// CPU is the current CPU usage
	CPU resource.Quantity
	
	// Memory is the current memory usage
	Memory resource.Quantity
	
	// Storage is the current storage usage
	Storage resource.Quantity
	
	// CustomResources contains usage for custom resources
	CustomResources map[string]resource.Quantity
}


// EvaluationOptions provides options for customizing evaluation behavior.
type EvaluationOptions struct {
	// Timeout sets the maximum duration for evaluation
	Timeout time.Duration
	
	// MaxCost limits the computational cost of expressions
	MaxCost uint64
	
	// EnableDebug enables debug logging for expression evaluation
	EnableDebug bool
	
	// Variables provides default variables for all evaluations
	Variables map[string]interface{}
}

// ExpressionCache provides caching for compiled expressions.
type ExpressionCache interface {
	// Get retrieves a compiled expression from the cache
	Get(hash string) (*CompiledExpression, bool)
	
	// Set stores a compiled expression in the cache
	Set(hash string, expr *CompiledExpression)
	
	// Delete removes an expression from the cache
	Delete(hash string)
	
	// Clear removes all expressions from the cache
	Clear()
	
	// Size returns the number of cached expressions
	Size() int
}

// ValidationResult represents the result of expression validation.
type ValidationResult struct {
	// Valid indicates if the expression is valid
	Valid bool
	
	// Errors contains any validation errors
	Errors []string
	
	// Warnings contains any validation warnings
	Warnings []string
	
	// ReturnType is the expected return type of the expression
	ReturnType *cel.Type
}

// EvaluationResult represents the result of an expression evaluation.
type EvaluationResult struct {
	// Value is the result value
	Value interface{}
	
	// Type is the CEL type of the result
	Type types.Type
	
	// Duration is how long the evaluation took
	Duration time.Duration
	
	// Cost is the computational cost of the evaluation
	Cost uint64
}

// LabelSelector represents a label selector for workspace matching.
type LabelSelector struct {
	// MatchLabels is a map of key-value pairs for exact matching
	MatchLabels map[string]string
	
	// MatchExpressions is a list of label selector requirements
	MatchExpressions []metav1.LabelSelectorRequirement
}

// Distance represents the distance between workspaces (for network latency, etc.).
type Distance struct {
	// From is the source workspace
	From logicalcluster.Name
	
	// To is the target workspace
	To logicalcluster.Name
	
	// NetworkDistance is the network latency or distance metric
	NetworkDistance float64
	
	// GeographicalDistance is the physical distance (if available)
	GeographicalDistance float64
	
	// LastMeasured is when this distance was last measured
	LastMeasured time.Time
}