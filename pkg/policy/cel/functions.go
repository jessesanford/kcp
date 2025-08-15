package cel

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// RegisterCustomFunctions registers TMC-specific CEL functions
func RegisterCustomFunctions() cel.EnvOption {
	return cel.Lib(&customFunctions{})
}

// customFunctions implements cel.Library for TMC-specific functions
type customFunctions struct{}

// CompileOptions returns compile-time function declarations
func (c *customFunctions) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("hasLabel",
			cel.MemberOverload("map_has_label", []*cel.Type{
				cel.MapType(cel.StringType, cel.StringType),
				cel.StringType,
			}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return hasLabel(lhs, rhs)
				})),
		),
		cel.Function("labelValue",
			cel.MemberOverload("map_label_value", []*cel.Type{
				cel.MapType(cel.StringType, cel.StringType),
				cel.StringType,
			}, cel.StringType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return labelValue(lhs, rhs)
				})),
		),
		cel.Function("inRegion",
			cel.MemberOverload("cluster_in_region", []*cel.Type{
				cel.MapType(cel.StringType, cel.AnyType),
				cel.StringType,
			}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return inRegion(lhs, rhs)
				})),
		),
		cel.Function("hasCapacity",
			cel.MemberOverload("cluster_has_capacity", []*cel.Type{
				cel.MapType(cel.StringType, cel.AnyType),
				cel.StringType,
				cel.IntType,
			}, cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return hasCapacity(args[0], args[1], args[2])
				})),
		),
		cel.Function("matchesWorkspace",
			cel.Overload("matches_workspace", []*cel.Type{
				cel.StringType,
				cel.StringType,
			}, cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return matchesWorkspace(lhs, rhs)
				})),
		),
	}
}

// ProgramOptions returns runtime function implementations
func (c *customFunctions) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// hasLabel checks if a map contains a specific label key
func hasLabel(lhs, rhs ref.Val) ref.Val {
	labels, ok := lhs.(types.Mapper)
	if !ok {
		return types.NewErr("left operand must be a map")
	}
	
	key, ok := rhs.(types.String)
	if !ok {
		return types.NewErr("right operand must be a string")
	}
	
	_, found := labels.Find(key)
	return types.Bool(found != types.NoSuchKeyError)
}

// labelValue gets the value of a label key
func labelValue(lhs, rhs ref.Val) ref.Val {
	labels, ok := lhs.(types.Mapper)
	if !ok {
		return types.NewErr("left operand must be a map")
	}
	
	key, ok := rhs.(types.String)
	if !ok {
		return types.NewErr("right operand must be a string")
	}
	
	value, found := labels.Find(key)
	if found != types.NoSuchKeyError {
		return value
	}
	return types.String("")
}

// inRegion checks if a cluster is in a specific region
func inRegion(lhs, rhs ref.Val) ref.Val {
	cluster, ok := lhs.(types.Mapper)
	if !ok {
		return types.NewErr("left operand must be a cluster map")
	}
	
	targetRegion, ok := rhs.(types.String)
	if !ok {
		return types.NewErr("right operand must be a string")
	}
	
	regionVal, found := cluster.Find(types.String("region"))
	if found != types.NoSuchKeyError {
		if region, ok := regionVal.(types.String); ok {
			return types.Bool(region == targetRegion)
		}
	}
	
	return types.Bool(false)
}

// hasCapacity checks if a cluster has sufficient capacity for a resource
func hasCapacity(cluster, resource, amount ref.Val) ref.Val {
	clusterMap, ok := cluster.(types.Mapper)
	if !ok {
		return types.NewErr("first argument must be a cluster map")
	}
	
	resourceName, ok := resource.(types.String)
	if !ok {
		return types.NewErr("second argument must be a string")
	}
	
	requiredAmount, ok := amount.(types.Int)
	if !ok {
		return types.NewErr("third argument must be an integer")
	}
	
	// Get available resources
	availableVal, found := clusterMap.Find(types.String("available"))
	if found != types.NoSuchKeyError {
		if available, ok := availableVal.(types.Mapper); ok {
			resourceVal, found := available.Find(resourceName)
			if found != types.NoSuchKeyError {
				if resourceAmount, ok := resourceVal.(types.Int); ok {
					return types.Bool(resourceAmount >= requiredAmount)
				}
			}
		}
	}
	
	return types.Bool(false)
}

// matchesWorkspace checks if a workspace matches a pattern
func matchesWorkspace(lhs, rhs ref.Val) ref.Val {
	workspace, ok := lhs.(types.String)
	if !ok {
		return types.NewErr("left operand must be a string")
	}
	
	pattern, ok := rhs.(types.String)
	if !ok {
		return types.NewErr("right operand must be a string")
	}
	
	// Simple prefix matching for workspace hierarchies
	// In practice, this could be more sophisticated
	workspaceStr := string(workspace)
	patternStr := string(pattern)
	
	// Check if workspace starts with pattern
	if len(workspaceStr) >= len(patternStr) {
		return types.Bool(workspaceStr[:len(patternStr)] == patternStr)
	}
	
	return types.Bool(false)
}