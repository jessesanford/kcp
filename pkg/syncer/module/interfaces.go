package module

import "context"

// ResourceSyncer defines an interface for synchronizing resources between KCP and workload clusters.
type ResourceSyncer interface {
	Start(ctx context.Context) error
}

// ConflictResolver defines an interface for resolving conflicts encountered during synchronization.
type ConflictResolver interface {
	Resolve(ctx context.Context, err error) error
}

// ErrorHandler defines an interface for handling non-conflict errors during synchronization.
type ErrorHandler interface {
	Handle(ctx context.Context, err error)
}

// MetricsCollector defines an interface for collecting synchronization metrics.
type MetricsCollector interface {
	IncConflict()
	IncError()
}

// Logger abstracts logging functionality for synchronization activities.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(err error, msg string, keysAndValues ...interface{})
}
