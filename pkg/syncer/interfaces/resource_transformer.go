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

package interfaces

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kcp-dev/logicalcluster/v3"
)

// ResourceTransformer handles resource transformations during synchronization between
// logical and physical clusters. It provides workspace-aware transformations that
// ensure resources are properly adapted for their target environment while maintaining
// consistency and compliance with KCP patterns.
type ResourceTransformer interface {
	// TransformForDownstream transforms a resource from logical cluster format to
	// physical cluster format. This includes adding required annotations, labels,
	// and modifications needed for the resource to function in the target cluster.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - workspace: Logical cluster the resource originates from
	//   - targetCluster: Logical cluster representing the physical cluster target
	//   - resource: Resource to transform
	//
	// Returns transformed resource or error if transformation fails.
	TransformForDownstream(ctx context.Context, workspace logicalcluster.Name, targetCluster logicalcluster.Name, resource *unstructured.Unstructured) (*unstructured.Unstructured, error)

	// TransformForUpstream transforms a resource from physical cluster format to
	// logical cluster format. This includes removing physical cluster-specific
	// annotations and adapting the resource for logical cluster storage.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - sourceCluster: Logical cluster representing the physical cluster source
	//   - workspace: Logical cluster where the resource will be stored
	//   - resource: Resource to transform
	//
	// Returns transformed resource or error if transformation fails.
	TransformForUpstream(ctx context.Context, sourceCluster logicalcluster.Name, workspace logicalcluster.Name, resource *unstructured.Unstructured) (*unstructured.Unstructured, error)

	// ShouldTransform determines if a resource requires transformation based on
	// its type, annotations, or other characteristics.
	//
	// Parameters:
	//   - gvr: Group/Version/Resource of the object
	//   - resource: Resource to evaluate
	//   - direction: Direction of sync (upstream or downstream)
	//
	// Returns true if the resource should be transformed.
	ShouldTransform(gvr schema.GroupVersionResource, resource *unstructured.Unstructured, direction SyncDirection) bool

	// ValidateTransformation validates that a transformed resource is valid and
	// conforms to the target cluster's requirements.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - original: Original resource before transformation
	//   - transformed: Resource after transformation
	//   - direction: Direction of sync (upstream or downstream)
	//
	// Returns error if validation fails.
	ValidateTransformation(ctx context.Context, original, transformed *unstructured.Unstructured, direction SyncDirection) error
}

// TransformationRule defines a specific transformation to be applied to resources.
type TransformationRule interface {
	// Matches determines if this rule applies to the given resource.
	//
	// Parameters:
	//   - gvr: Group/Version/Resource of the object
	//   - resource: Resource to evaluate
	//   - direction: Direction of sync
	//
	// Returns true if the rule should be applied.
	Matches(gvr schema.GroupVersionResource, resource *unstructured.Unstructured, direction SyncDirection) bool

	// Apply applies the transformation rule to the resource.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - workspace: Source or target workspace depending on direction
	//   - targetCluster: Target cluster for the transformation
	//   - resource: Resource to transform
	//   - direction: Direction of sync
	//
	// Returns transformed resource or error if transformation fails.
	Apply(ctx context.Context, workspace logicalcluster.Name, targetCluster logicalcluster.Name, resource *unstructured.Unstructured, direction SyncDirection) (*unstructured.Unstructured, error)

	// Priority returns the priority of this rule. Higher values indicate higher priority.
	// Rules are applied in priority order (highest first).
	Priority() int32
}

// FieldTransformer handles transformation of specific fields within resources.
type FieldTransformer interface {
	// TransformField transforms a specific field path within a resource.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - resource: Resource containing the field
	//   - fieldPath: JSONPath to the field to transform
	//   - direction: Direction of sync
	//   - workspace: Workspace context for the transformation
	//
	// Returns error if field transformation fails.
	TransformField(ctx context.Context, resource *unstructured.Unstructured, fieldPath string, direction SyncDirection, workspace logicalcluster.Name) error

	// SupportsField determines if this transformer can handle the specified field.
	//
	// Parameters:
	//   - gvr: Group/Version/Resource of the object
	//   - fieldPath: JSONPath to the field
	//
	// Returns true if the field is supported.
	SupportsField(gvr schema.GroupVersionResource, fieldPath string) bool
}

// TransformationContext provides additional context for resource transformations.
type TransformationContext struct {
	// SourceWorkspace is the workspace the resource is coming from.
	SourceWorkspace logicalcluster.Name

	// TargetWorkspace is the workspace the resource is going to.
	TargetWorkspace logicalcluster.Name

	// Direction indicates the direction of synchronization.
	Direction SyncDirection

	// PlacementName is the name of the placement that triggered this sync.
	PlacementName string

	// SyncTargetName is the name of the sync target for physical cluster operations.
	SyncTargetName string

	// Annotations contains additional metadata for the transformation.
	Annotations map[string]string
}
