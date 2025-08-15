package cel

import (
	"fmt"
)

// VariableResolver resolves variables for evaluation
type VariableResolver struct {
	defaults map[string]interface{}
}

// NewVariableResolver creates a new variable resolver
func NewVariableResolver() *VariableResolver {
	return &VariableResolver{
		defaults: DefaultVariables(),
	}
}

// Resolve merges provided variables with defaults
func (r *VariableResolver) Resolve(provided map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Start with defaults
	for k, v := range r.defaults {
		result[k] = v
	}
	
	// Overlay provided values
	for k, v := range provided {
		result[k] = r.mergeValue(result[k], v)
	}
	
	return result
}

// mergeValue merges two values, preferring the new value
func (r *VariableResolver) mergeValue(old, new interface{}) interface{} {
	if new == nil {
		return old
	}
	
	// If both are maps, merge them
	oldMap, oldOk := old.(map[string]interface{})
	newMap, newOk := new.(map[string]interface{})
	
	if oldOk && newOk {
		merged := make(map[string]interface{})
		for k, v := range oldMap {
			merged[k] = v
		}
		for k, v := range newMap {
			merged[k] = r.mergeValue(merged[k], v)
		}
		return merged
	}
	
	return new
}

// Validate checks if required variables are present
func (r *VariableResolver) Validate(vars map[string]interface{}, required []string) error {
	for _, req := range required {
		if _, ok := vars[req]; !ok {
			return fmt.Errorf("required variable %s not provided", req)
		}
	}
	return nil
}

// ExtractVariables extracts variable names from an expression
// This is a simple implementation - would use CEL AST in production
func ExtractVariables(expression string) []string {
	vars := []string{}
	// This would parse the expression and extract variable references
	// For now, returning common variables that might be used
	commonVars := []string{"cluster", "workload", "workspace", "user"}
	return append(vars, commonVars...)
}