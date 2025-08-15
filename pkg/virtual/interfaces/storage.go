package interfaces

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
)

// StorageInterface provides persistence capabilities for virtual workspace data.
// It defines a generic storage contract for persisting Kubernetes objects
// within virtual workspace contexts, supporting CRUD operations and real-time monitoring.
type StorageInterface interface {
	// Get retrieves an object from storage using the specified key.
	// The object parameter should be a pointer to the expected type.
	Get(ctx context.Context, key string, obj runtime.Object) error

	// Create stores a new object in storage with the specified key.
	// Returns an error if an object with the same key already exists.
	Create(ctx context.Context, key string, obj runtime.Object) error

	// Update modifies an existing object in storage.
	// The object must already exist or an error will be returned.
	Update(ctx context.Context, key string, obj runtime.Object) error

	// Delete removes an object from storage using the specified key.
	// Returns an error if the object does not exist.
	Delete(ctx context.Context, key string) error

	// List returns objects matching the provided options.
	// This enables efficient querying and pagination of stored objects.
	List(ctx context.Context, opts ListOptions) ([]runtime.Object, error)

	// Watch monitors for changes to objects in storage, returning
	// a channel of watch events for real-time updates.
	Watch(ctx context.Context, opts WatchOptions) (<-chan WatchEvent, error)
}

// ListOptions configures a list operation with filtering and pagination.
// These options enable efficient retrieval of object collections from storage.
type ListOptions struct {
	// Prefix filters objects by key prefix, enabling namespace-like organization.
	Prefix string

	// Limit restricts the maximum number of objects returned in a single request.
	Limit int

	// Continue is a pagination token for retrieving additional results.
	// This is returned from previous list operations when more results are available.
	Continue string
}

// WatchOptions configures a watch operation for monitoring object changes.
// These options control the scope and behavior of watch operations.
type WatchOptions struct {
	// Prefix filters watch events by key prefix, enabling scoped monitoring.
	Prefix string

	// ResourceVersion specifies the starting point for watch operations.
	// Only changes after this version will be included in the watch stream.
	ResourceVersion string
}

// WatchEvent represents a change to stored data within the storage system.
// These events enable reactive programming patterns and real-time synchronization.
type WatchEvent struct {
	// Type categorizes the event (added, modified, deleted).
	Type EventType

	// Object contains the object data associated with this event.
	// For delete events, this represents the object state before deletion.
	Object runtime.Object
}