package virtualworkspace

import (
	"context"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// LocationResponse from a single location
type LocationResponse struct {
	// Location name
	Location string

	// StatusCode from location
	StatusCode int

	// Body of response
	Body []byte

	// Headers from response
	Headers http.Header

	// Error if request failed
	Error error
}

// ResponseAggregator aggregates responses from multiple locations
type ResponseAggregator interface {
	// AggregateList aggregates list responses
	AggregateList(
		ctx context.Context,
		responses []LocationResponse,
	) (*unstructured.UnstructuredList, error)

	// AggregateGet aggregates get responses
	AggregateGet(
		ctx context.Context,
		responses []LocationResponse,
	) (*unstructured.Unstructured, error)

	// AggregateStatus aggregates status responses
	AggregateStatus(
		ctx context.Context,
		responses []LocationResponse,
	) (*unstructured.Unstructured, error)
}

// AggregationStrategyFunc defines how to aggregate
type AggregationStrategyFunc func(
	responses []LocationResponse,
) (*runtime.Object, error)

// ListAggregator aggregates list operations
type ListAggregator interface {
	// MergeLists combines multiple lists
	MergeLists(lists []*unstructured.UnstructuredList) (*unstructured.UnstructuredList, error)

	// DeduplicateItems removes duplicates
	DeduplicateItems(list *unstructured.UnstructuredList) (*unstructured.UnstructuredList, error)

	// SortItems sorts list items
	SortItems(list *unstructured.UnstructuredList, field string) error
}

// ConflictResolver resolves conflicts in responses
type ConflictResolver interface {
	// ResolveConflict picks winning response
	ResolveConflict(
		responses []LocationResponse,
		strategy ConflictResolutionStrategy,
	) (*LocationResponse, error)
}

type ConflictResolutionStrategy string

const (
	ConflictResolutionLatest         ConflictResolutionStrategy = "Latest"
	ConflictResolutionHighestVersion ConflictResolutionStrategy = "HighestVersion"
	ConflictResolutionPriority       ConflictResolutionStrategy = "Priority"
)

// ResponseTransformer transforms responses
type ResponseTransformer interface {
	// Transform modifies response before returning
	Transform(
		ctx context.Context,
		response *LocationResponse,
		location string,
	) (*LocationResponse, error)
}