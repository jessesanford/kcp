package syncer

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	workloadv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/workload/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/logicalcluster"
)

// Transformer transforms resources during sync
type Transformer interface {
	// Transform applies transformations to a resource
	Transform(
		ctx context.Context,
		obj *unstructured.Unstructured,
		location string,
		direction TransformDirection,
	) (*unstructured.Unstructured, error)

	// CanTransform checks if transformation is possible
	CanTransform(obj *unstructured.Unstructured) bool

	// GetTransformRules returns applicable rules
	GetTransformRules(location string) []workloadv1alpha1.Transform
}

// TransformDirection indicates transformation direction
type TransformDirection string

const (
	// TransformDirectionUpToDown from KCP to physical cluster
	TransformDirectionUpToDown TransformDirection = "UpToDown"

	// TransformDirectionDownToUp from physical cluster to KCP
	TransformDirectionDownToUp TransformDirection = "DownToUp"
)

// TransformChain applies multiple transformers
type TransformChain interface {
	// AddTransformer adds a transformer to the chain
	AddTransformer(transformer Transformer) error

	// RemoveTransformer removes a transformer
	RemoveTransformer(name string) error

	// Apply applies all transformers in order
	Apply(
		ctx context.Context,
		obj *unstructured.Unstructured,
		location string,
		direction TransformDirection,
	) (*unstructured.Unstructured, error)
}

// TransformerPlugin is a pluggable transformer
type TransformerPlugin interface {
	Transformer

	// Name returns the plugin name
	Name() string

	// Initialize sets up the plugin
	Initialize(config map[string]interface{}) error

	// Validate validates plugin configuration
	Validate() error
}

// TransformContext provides context for transformations
type TransformContext struct {
	// Workspace being synced from
	SourceWorkspace logicalcluster.Name

	// Target location
	TargetLocation string

	// Additional metadata
	Metadata map[string]interface{}
}