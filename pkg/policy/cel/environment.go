package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/kcp-dev/kcp/pkg/policy/types"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// createEnvironment creates the base CEL environment
func createEnvironment() (*cel.Env, error) {
	return cel.NewEnv(
		// Standard CEL options
		cel.StdLib(),
		
		// Custom TMC variables
		cel.Variable("cluster", cel.MapType(cel.StringType, cel.AnyType)),
		cel.Variable("workload", cel.MapType(cel.StringType, cel.AnyType)),
		cel.Variable("workspace", cel.StringType),
		cel.Variable("user", cel.StringType),
		
		// Register custom functions
		RegisterCustomFunctions(),
		
		// Optimization options
		cel.OptionalTypes(),
		cel.EagerEvaluation(true),
	)
}

// getCELType converts our variable types to CEL types
func getCELType(varType types.VariableType) *exprpb.Type {
	switch varType {
	case types.VarTypeBool:
		return decls.Bool
	case types.VarTypeInt:
		return decls.Int
	case types.VarTypeFloat:
		return decls.Double
	case types.VarTypeString:
		return decls.String
	case types.VarTypeList:
		return decls.NewListType(decls.Any)
	case types.VarTypeMap:
		return decls.NewMapType(decls.String, decls.Any)
	case types.VarTypeCluster:
		return decls.NewMapType(decls.String, decls.Any)
	case types.VarTypeWorkload:
		return decls.NewMapType(decls.String, decls.Any)
	case types.VarTypeWorkspace:
		return decls.String
	default:
		return decls.Any
	}
}

// createVariableMap creates CEL variable declarations
func createVariableMap() map[string]*exprpb.Type {
	return map[string]*exprpb.Type{
		"cluster.name":       decls.String,
		"cluster.region":     decls.String,
		"cluster.zone":       decls.String,
		"cluster.labels":     decls.NewMapType(decls.String, decls.String),
		"cluster.capacity":   decls.NewMapType(decls.String, decls.Int),
		"cluster.available":  decls.NewMapType(decls.String, decls.Int),
		"workload.name":      decls.String,
		"workload.namespace": decls.String,
		"workload.labels":    decls.NewMapType(decls.String, decls.String),
		"workload.replicas":  decls.Int,
		"workspace":          decls.String,
		"user":               decls.String,
	}
}

// DefaultVariables returns default variable values for testing
func DefaultVariables() map[string]interface{} {
	return map[string]interface{}{
		"cluster": map[string]interface{}{
			"name":   "test-cluster",
			"region": "us-west-2",
			"zone":   "us-west-2a",
			"labels": map[string]string{},
			"capacity": map[string]int{
				"cpu":    100,
				"memory": 1000,
			},
			"available": map[string]int{
				"cpu":    50,
				"memory": 500,
			},
		},
		"workload": map[string]interface{}{
			"name":      "test-workload",
			"namespace": "default",
			"labels":    map[string]string{},
			"replicas":  3,
		},
		"workspace": "root:org:team",
		"user":      "test-user",
	}
}