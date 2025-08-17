package virtualworkspace

import (
	"context"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"

	apiresourcev1alpha1 "github.com/kcp-dev/kcp/pkg/apis/apiresource/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/logicalcluster"
	"github.com/kcp-dev/kcp/pkg/interfaces/virtualworkspace/router"
)

// VirtualWorkspace provides a unified API surface across locations
type VirtualWorkspace interface {
	// Initialize sets up the virtual workspace
	Initialize(ctx context.Context, config *VirtualWorkspaceConfig) error

	// Start begins serving requests
	Start(ctx context.Context) error

	// Stop halts the virtual workspace
	Stop() error

	// GetURL returns the virtual workspace URL
	GetURL() string

	// GetAPIGroups returns available API groups
	GetAPIGroups() []APIGroupInfo

	// HandleRequest processes an API request
	HandleRequest(w http.ResponseWriter, r *http.Request) error

	// RegisterLocation adds a location to serve from
	RegisterLocation(location router.LocationInfo) error

	// UnregisterLocation removes a location
	UnregisterLocation(name string) error
}

// VirtualWorkspaceConfig contains VW configuration
type VirtualWorkspaceConfig struct {
	// Name of the virtual workspace
	Name string

	// Workspace logical cluster
	Workspace logicalcluster.Name

	// ListenAddress for the VW server
	ListenAddress string

	// TLSConfig for secure connections
	TLSConfig *TLSConfig

	// AuthConfig for authentication
	AuthConfig *AuthConfig

	// ProjectionPolicy for API projection
	ProjectionPolicy ProjectionPolicy

	// AggregationPolicy for response aggregation
	AggregationPolicy AggregationPolicy
}

// TLSConfig contains TLS settings
type TLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	// Type of authentication
	Type AuthType

	// TokenReviewURL for token validation
	TokenReviewURL string

	// ClientCAFile for client cert auth
	ClientCAFile string
}

type AuthType string

const (
	AuthTypeToken      AuthType = "Token"
	AuthTypeClientCert AuthType = "ClientCert"
	AuthTypeOIDC       AuthType = "OIDC"
)

// APIGroupInfo describes an API group
type APIGroupInfo struct {
	// Name of the group
	Name string

	// Versions available
	Versions []string

	// PreferredVersion
	PreferredVersion string

	// Resources in this group
	Resources []ResourceInfo
}

// ResourceInfo describes a resource
type ResourceInfo struct {
	// Name (plural) of the resource
	Name string

	// Kind of the resource
	Kind string

	// Namespaced or cluster-scoped
	Namespaced bool

	// Verbs supported
	Verbs []string

	// Locations serving this resource
	Locations []string
}

// ProjectionPolicy controls API projection
type ProjectionPolicy struct {
	// Mode of projection
	Mode ProjectionMode

	// IncludeAPIs to project
	IncludeAPIs []schema.GroupVersion

	// ExcludeAPIs from projection
	ExcludeAPIs []schema.GroupVersion
}

type ProjectionMode string

const (
	ProjectionModeAll       ProjectionMode = "All"
	ProjectionModeWhitelist ProjectionMode = "Whitelist"
	ProjectionModeBlacklist ProjectionMode = "Blacklist"
)

// AggregationPolicy controls response aggregation
type AggregationPolicy struct {
	// Strategy for aggregation
	Strategy AggregationStrategy

	// Timeout for requests
	Timeout time.Duration
}

type AggregationStrategy string

const (
	AggregationStrategyFirstValid AggregationStrategy = "FirstValid"
	AggregationStrategyMergeAll   AggregationStrategy = "MergeAll"
	AggregationStrategyRoundRobin AggregationStrategy = "RoundRobin"
)