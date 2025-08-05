package module

import (
	"context"
	"sync/atomic"

	"k8s.io/klog/v2"
)

// DefaultLogger implements Logger using klog.
type DefaultLogger struct{}

func (DefaultLogger) Info(msg string, keysAndValues ...interface{}) {
	klog.InfoS(msg, keysAndValues...)
}

func (DefaultLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	klog.ErrorS(err, msg, keysAndValues...)
}

// DefaultMetricsCollector provides basic counters for conflicts and errors.
type DefaultMetricsCollector struct {
	conflict atomic.Int64
	errCount atomic.Int64
}

func (m *DefaultMetricsCollector) IncConflict() { m.conflict.Add(1) }
func (m *DefaultMetricsCollector) IncError()    { m.errCount.Add(1) }

// NopResourceSyncer is a placeholder ResourceSyncer used until concrete
// implementations are wired in.
type NopResourceSyncer struct{}

func (NopResourceSyncer) Start(ctx context.Context) error { return nil }

// NopConflictResolver simply returns the provided error.
type NopConflictResolver struct{}

func (NopConflictResolver) Resolve(ctx context.Context, err error) error { return err }

// NopErrorHandler logs the error via the provided logger if any.
type NopErrorHandler struct{ Logger }

func (h NopErrorHandler) Handle(ctx context.Context, err error) {
	if err != nil && h.Logger != nil {
		h.Logger.Error(err, "syncer error")
	}
}
