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

package cel

import (
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// KCPFunctions provides KCP-specific CEL functions for TMC policy evaluation.
// These functions enable policy expressions to work with KCP concepts like workspaces,
// clusters, and placement constraints.
type KCPFunctions struct {
	// Custom function implementations can store state if needed
}

// NewKCPFunctions creates a new instance of KCP CEL function provider.
func NewKCPFunctions() *KCPFunctions {
	return &KCPFunctions{}
}

// GetFunctionDeclarations returns CEL function declarations for KCP-specific operations.
// These functions are available in all policy expressions and provide KCP domain logic.
func (k *KCPFunctions) GetFunctionDeclarations() []cel.EnvOption {
	return []cel.EnvOption{
		// Workspace hierarchy functions
		cel.Function("inWorkspace",
			cel.Overload("inWorkspace_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(k.inWorkspace))),

		// Label manipulation functions
		cel.Function("hasLabel",
			cel.Overload("hasLabel_map_string",
				[]*cel.Type{cel.MapType(cel.StringType, cel.StringType), cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(k.hasLabel))),

		cel.Function("labelMatches",
			cel.Overload("labelMatches_map_string_string",
				[]*cel.Type{cel.MapType(cel.StringType, cel.StringType), cel.StringType, cel.StringType},
				cel.BoolType,
				cel.FunctionBinding(k.labelMatches))),

		// Resource capacity functions
		cel.Function("hasCapacity",
			cel.Overload("hasCapacity_map_string_int",
				[]*cel.Type{cel.MapType(cel.StringType, cel.AnyType), cel.StringType, cel.IntType},
				cel.BoolType,
				cel.FunctionBinding(k.hasCapacity))),

		// Geographic/region functions
		cel.Function("inRegion",
			cel.Overload("inRegion_string_list",
				[]*cel.Type{cel.StringType, cel.ListType(cel.StringType)},
				cel.BoolType,
				cel.BinaryBinding(k.inRegion))),

		// Cost optimization functions
		cel.Function("costTier",
			cel.Overload("costTier_map",
				[]*cel.Type{cel.MapType(cel.StringType, cel.AnyType)},
				cel.StringType,
				cel.UnaryBinding(k.costTier))),
	}
}

// inWorkspace checks if a cluster is within a specified workspace hierarchy.
// Supports workspace path matching with hierarchical semantics.
// Example: inWorkspace("root:org:team", "root:org") returns true
func (k *KCPFunctions) inWorkspace(lhs, rhs ref.Val) ref.Val {
	workspace, ok := lhs.(types.String)
	if !ok {
		return types.ValOrErr(workspace, "invalid workspace type")
	}

	pattern, ok := rhs.(types.String)
	if !ok {
		return types.ValOrErr(pattern, "invalid pattern type")
	}

	// Check workspace hierarchy - pattern must be prefix of workspace
	wsStr := string(workspace)
	patStr := string(pattern)

	if strings.HasPrefix(wsStr, patStr) {
		return types.Bool(true)
	}

	return types.Bool(false)
}

// hasLabel checks if a label key exists in the provided label map.
// Returns true if the key exists regardless of value.
// Example: hasLabel(cluster.labels, "environment") 
func (k *KCPFunctions) hasLabel(lhs, rhs ref.Val) ref.Val {
	labels, ok := lhs.(types.Mapper)
	if !ok {
		return types.ValOrErr(labels, "invalid labels type")
	}

	key, ok := rhs.(types.String)
	if !ok {
		return types.ValOrErr(key, "invalid key type")
	}

	_, found := labels.Find(key)
	return types.Bool(found)
}

// labelMatches checks if a label key exists and has the specified value.
// Performs exact string matching on label values.
// Example: labelMatches(cluster.labels, "tier", "production")
func (k *KCPFunctions) labelMatches(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.NewErr("labelMatches requires 3 arguments")
	}

	labels, ok := args[0].(types.Mapper)
	if !ok {
		return types.NewErr("first argument must be map")
	}

	key, ok := args[1].(types.String)
	if !ok {
		return types.NewErr("second argument must be string")
	}

	expected, ok := args[2].(types.String)
	if !ok {
		return types.NewErr("third argument must be string")
	}

	val, found := labels.Find(key)
	if !found {
		return types.Bool(false)
	}

	strVal, ok := val.(types.String)
	if !ok {
		return types.Bool(false)
	}

	return types.Bool(string(strVal) == string(expected))
}

// hasCapacity checks if a cluster has at least the specified amount of a resource.
// Compares numeric resource values for capacity planning.
// Example: hasCapacity(cluster.resources, "cpu", 4)
func (k *KCPFunctions) hasCapacity(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.NewErr("hasCapacity requires 3 arguments")
	}

	resources, ok := args[0].(types.Mapper)
	if !ok {
		return types.NewErr("first argument must be map")
	}

	resourceName, ok := args[1].(types.String)
	if !ok {
		return types.NewErr("second argument must be string")
	}

	requiredAmount, ok := args[2].(types.Int)
	if !ok {
		return types.NewErr("third argument must be int")
	}

	availableVal, found := resources.Find(resourceName)
	if !found {
		return types.Bool(false)
	}

	available, ok := availableVal.(types.Int)
	if !ok {
		return types.Bool(false)
	}

	return types.Bool(available >= requiredAmount)
}

// inRegion checks if a cluster's region is in the allowed regions list.
// Supports region-based placement constraints for compliance.
// Example: inRegion(cluster.region, ["us-west-1", "us-west-2"])
func (k *KCPFunctions) inRegion(lhs, rhs ref.Val) ref.Val {
	region, ok := lhs.(types.String)
	if !ok {
		return types.ValOrErr(region, "invalid region type")
	}

	allowedRegions, ok := rhs.(types.Lister)
	if !ok {
		return types.ValOrErr(allowedRegions, "invalid regions list type")
	}

	// Check if region is in the allowed list
	regionStr := string(region)
	for i := 0; i < int(allowedRegions.Size()); i++ {
		item := allowedRegions.Get(types.IntOf(i))
		if itemStr, ok := item.(types.String); ok {
			if string(itemStr) == regionStr {
				return types.Bool(true)
			}
		}
	}

	return types.Bool(false)
}

// costTier extracts the cost tier classification from cluster metadata.
// Returns standardized cost tier strings for economic optimization.
// Example: costTier(cluster.metadata) returns "spot", "standard", or "premium"
func (k *KCPFunctions) costTier(arg ref.Val) ref.Val {
	metadata, ok := arg.(types.Mapper)
	if !ok {
		return types.ValOrErr(metadata, "invalid metadata type")
	}

	// Look for cost tier in common locations
	if tierVal, found := metadata.Find(types.String("costTier")); found {
		if tier, ok := tierVal.(types.String); ok {
			return tier
		}
	}

	if tierVal, found := metadata.Find(types.String("cost-tier")); found {
		if tier, ok := tierVal.(types.String); ok {
			return tier
		}
	}

	// Default to standard tier if not specified
	return types.String("standard")
}