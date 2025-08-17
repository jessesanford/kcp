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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
)

// CustomFunction defines the signature for custom CEL functions.
type CustomFunction interface {
	// Name returns the function name as it will appear in CEL expressions
	Name() string
}

// hasLabelFunction implements the hasLabel(key, value) function.
type hasLabelFunction struct{}

// NewHasLabelFunction creates a new hasLabel function.
func NewHasLabelFunction() CustomFunction {
	return &hasLabelFunction{}
}

func (f *hasLabelFunction) Name() string {
	return "hasLabel"
}

// inWorkspaceFunction implements the inWorkspace(name) function.
type inWorkspaceFunction struct{}

// NewInWorkspaceFunction creates a new inWorkspace function.
func NewInWorkspaceFunction() CustomFunction {
	return &inWorkspaceFunction{}
}

func (f *inWorkspaceFunction) Name() string {
	return "inWorkspace"
}

// hasCapacityFunction implements the hasCapacity(resource, amount) function.
type hasCapacityFunction struct{}

// NewHasCapacityFunction creates a new hasCapacity function.
func NewHasCapacityFunction() CustomFunction {
	return &hasCapacityFunction{}
}

func (f *hasCapacityFunction) Name() string {
	return "hasCapacity"
}

// matchesSelectorFunction implements the matchesSelector(selector) function.
type matchesSelectorFunction struct{}

// NewMatchesSelectorFunction creates a new matchesSelector function.
func NewMatchesSelectorFunction() CustomFunction {
	return &matchesSelectorFunction{}
}

func (f *matchesSelectorFunction) Name() string {
	return "matchesSelector"
}

// distanceFunction implements the distance(workspace1, workspace2) function.
type distanceFunction struct{}

// NewDistanceFunction creates a new distance function.
func NewDistanceFunction() CustomFunction {
	return &distanceFunction{}
}

func (f *distanceFunction) Name() string {
	return "distance"
}

// calculateDistance calculates distance between workspaces.
func (f *distanceFunction) calculateDistance(workspace1, workspace2 string) float64 {
	// Simplified distance calculation based on workspace names
	// In real implementation, this would use actual network metrics
	if workspace1 == workspace2 {
		return 0.0
	}

	// Simple heuristic based on string similarity
	common := 0
	for i := 0; i < len(workspace1) && i < len(workspace2); i++ {
		if workspace1[i] == workspace2[i] {
			common++
		} else {
			break
		}
	}

	maxLen := len(workspace1)
	if len(workspace2) > maxLen {
		maxLen = len(workspace2)
	}

	// Return a distance metric (lower is closer)
	return float64(maxLen-common) / float64(maxLen) * 100.0
}

// FunctionRegistry manages custom CEL functions.
type FunctionRegistry struct {
	functions map[string]CustomFunction
}

// NewFunctionRegistry creates a new function registry.
func NewFunctionRegistry() *FunctionRegistry {
	return &FunctionRegistry{
		functions: make(map[string]CustomFunction),
	}
}

// Register registers a custom function.
func (r *FunctionRegistry) Register(fn CustomFunction) error {
	name := fn.Name()
	if _, exists := r.functions[name]; exists {
		return fmt.Errorf("function %s already registered", name)
	}
	r.functions[name] = fn
	return nil
}

// Get retrieves a function by name.
func (r *FunctionRegistry) Get(name string) (CustomFunction, bool) {
	fn, exists := r.functions[name]
	return fn, exists
}

// List returns all registered function names.
func (r *FunctionRegistry) List() []string {
	names := make([]string, 0, len(r.functions))
	for name := range r.functions {
		names = append(names, name)
	}
	return names
}

// GetBuiltinFunctions returns all built-in KCP functions.
func GetBuiltinFunctions() []CustomFunction {
	return []CustomFunction{
		NewHasLabelFunction(),
		NewInWorkspaceFunction(),
		NewHasCapacityFunction(),
		NewMatchesSelectorFunction(),
		NewDistanceFunction(),
	}
}

// ValidateFunctionCall validates a function call with given arguments.
func ValidateFunctionCall(fn CustomFunction, args []interface{}) error {
	name := fn.Name()
	
	// Basic validation - in a full implementation, this would be more sophisticated
	switch name {
	case "hasLabel":
		if len(args) != 2 {
			return fmt.Errorf("hasLabel expects 2 arguments, got %d", len(args))
		}
		for i, arg := range args {
			if _, ok := arg.(string); !ok {
				return fmt.Errorf("hasLabel argument %d must be string", i+1)
			}
		}
	case "inWorkspace":
		if len(args) != 1 {
			return fmt.Errorf("inWorkspace expects 1 argument, got %d", len(args))
		}
		if _, ok := args[0].(string); !ok {
			return fmt.Errorf("inWorkspace argument must be string")
		}
	case "hasCapacity":
		if len(args) != 2 {
			return fmt.Errorf("hasCapacity expects 2 arguments, got %d", len(args))
		}
		if _, ok := args[0].(string); !ok {
			return fmt.Errorf("hasCapacity first argument must be string")
		}
		if _, ok := args[1].(string); !ok {
			return fmt.Errorf("hasCapacity second argument must be string")
		}
		// Validate resource quantity format
		if _, err := resource.ParseQuantity(args[1].(string)); err != nil {
			return fmt.Errorf("hasCapacity second argument must be valid resource quantity")
		}
	case "distance":
		if len(args) != 2 {
			return fmt.Errorf("distance expects 2 arguments, got %d", len(args))
		}
		for i, arg := range args {
			if _, ok := arg.(string); !ok {
				return fmt.Errorf("distance argument %d must be string", i+1)
			}
		}
	}

	return nil
}

// Helper functions for working with workspace and resource data

// WorkspaceMatches checks if a workspace matches the given criteria.
func WorkspaceMatches(workspace *WorkspaceContext, selector labels.Selector) bool {
	if workspace == nil || selector == nil {
		return false
	}
	return selector.Matches(labels.Set(workspace.Labels))
}

// HasSufficientCapacity checks if a workspace has sufficient capacity for requirements.
func HasSufficientCapacity(available *ResourceCapacity, required *ResourceRequirements) bool {
	if available == nil || required == nil {
		return false
	}

	// Check CPU
	if available.CPU.Cmp(required.CPU) < 0 {
		return false
	}

	// Check memory
	if available.Memory.Cmp(required.Memory) < 0 {
		return false
	}

	// Check storage
	if available.Storage.Cmp(required.Storage) < 0 {
		return false
	}

	// Check custom resources
	for name, requiredAmount := range required.CustomResources {
		availableAmount, exists := available.CustomResources[name]
		if !exists || availableAmount.Cmp(requiredAmount) < 0 {
			return false
		}
	}

	return true
}

// ParseWorkspaceName parses a workspace name string into a logical cluster name.
func ParseWorkspaceName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("workspace name cannot be empty")
	}

	// Validate workspace name format
	if strings.Contains(name, " ") {
		return "", fmt.Errorf("workspace name cannot contain spaces")
	}

	return name, nil
}