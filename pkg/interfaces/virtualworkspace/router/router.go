package router

import (
	"context"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"

	apiresourcev1alpha1 "github.com/kcp-dev/kcp/pkg/apis/apiresource/v1alpha1"
	"github.com/kcp-dev/kcp/pkg/logicalcluster"
)

// RequestRouter routes requests to appropriate locations
type RequestRouter interface {
	// RouteRequest determines where to send request
	RouteRequest(
		ctx context.Context,
		req *http.Request,
		info *request.RequestInfo,
	) (*RoutingDecision, error)

	// ExecuteRequest sends request to location(s)
	ExecuteRequest(
		ctx context.Context,
		req *http.Request,
		decision *RoutingDecision,
	) (*RoutingResult, error)
}

// RoutingDecision describes where to route
type RoutingDecision struct {
	// Strategy used for routing
	Strategy RoutingStrategy

	// Locations to route to
	Locations []LocationRoute

	// Timeout for the request
	Timeout time.Duration

	// RetryPolicy if request fails
	RetryPolicy *RetryPolicy
}

// LocationRoute describes routing to a location
type LocationRoute struct {
	// Name of the location
	Name string

	// URL to use
	URL string

	// Weight for weighted routing
	Weight int

	// Headers to add
	Headers map[string]string
}

type RoutingStrategy string

const (
	RoutingStrategySingle     RoutingStrategy = "Single"
	RoutingStrategyBroadcast  RoutingStrategy = "Broadcast"
	RoutingStrategyRoundRobin RoutingStrategy = "RoundRobin"
	RoutingStrategyWeighted   RoutingStrategy = "Weighted"
)

// LocationInfo describes a location
type LocationInfo struct {
	// Name of the location
	Name string

	// URL to access the location
	URL string

	// DiscoveredAPIs at this location
	DiscoveredAPIs []apiresourcev1alpha1.DiscoveredAPIGroup

	// Healthy status
	Healthy bool
}

// RoutingResult contains routing outcome
type RoutingResult struct {
	// Success indicates if routing succeeded
	Success bool

	// Error if routing failed
	Error error
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	// MaxAttempts to retry
	MaxAttempts int

	// BackoffMultiplier for exponential backoff
	BackoffMultiplier float64

	// MaxBackoff duration
	MaxBackoff time.Duration
}

// LoadBalancer balances requests across locations
type LoadBalancer interface {
	// SelectLocation picks a location
	SelectLocation(locations []LocationInfo) (*LocationInfo, error)

	// UpdateHealth updates location health
	UpdateHealth(location string, healthy bool)
}