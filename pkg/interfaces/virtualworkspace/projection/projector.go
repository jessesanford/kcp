package projection

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"

	apiresourcev1alpha1 "github.com/kcp-dev/kcp/pkg/apis/apiresource/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/interfaces/virtualworkspace"
)

// APIProjector projects APIs into virtual workspace
type APIProjector interface {
	// ProjectAPIs creates virtual API surface
	ProjectAPIs(
		ctx context.Context,
		discoveries []apiresourcev1alpha1.APIDiscovery,
		policy virtualworkspace.ProjectionPolicy,
	) (*ProjectedAPIs, error)

	// UpdateProjection updates existing projection
	UpdateProjection(
		ctx context.Context,
		current *ProjectedAPIs,
		discoveries []apiresourcev1alpha1.APIDiscovery,
	) (*ProjectedAPIs, error)

	// GetProjectedResource gets projected resource info
	GetProjectedResource(gvr schema.GroupVersionResource) (*ProjectedResource, error)
}

// ProjectedAPIs contains projected API surface
type ProjectedAPIs struct {
	// Groups projected
	Groups []ProjectedGroup

	// ResourceMap for quick lookup
	ResourceMap map[schema.GroupVersionResource]*ProjectedResource

	// LocationMap tracks resource locations
	LocationMap map[schema.GroupVersionResource][]string
}

// ProjectedGroup is a projected API group
type ProjectedGroup struct {
	// Name of the group
	Name string

	// Versions projected
	Versions []ProjectedVersion

	// SourceLocations contributing to this group
	SourceLocations []string
}

// ProjectedVersion is a projected API version
type ProjectedVersion struct {
	// Version string
	Version string

	// Resources in this version
	Resources []ProjectedResource
}

// ProjectedResource is a projected resource
type ProjectedResource struct {
	// GVR of the resource
	GVR schema.GroupVersionResource

	// Kind of the resource
	Kind string

	// Namespaced or cluster-scoped
	Namespaced bool

	// Verbs supported across all locations
	Verbs []string

	// Locations serving this resource
	Locations []string

	// SchemaHash for compatibility checking
	SchemaHash string
}

// ProjectionFilter filters resources for projection
type ProjectionFilter interface {
	// ShouldProject determines if resource should be projected
	ShouldProject(gvr schema.GroupVersionResource) bool

	// FilterVerbs filters allowed verbs
	FilterVerbs(verbs []string) []string
}