package module

// Modules groups implementations of the various syncer modules.
type Modules struct {
	ResourceSyncer   ResourceSyncer
	ConflictResolver ConflictResolver
	ErrorHandler     ErrorHandler
	MetricsCollector MetricsCollector
	Logger           Logger
}

// ApplyDefaults populates nil modules with default implementations.
func (m *Modules) ApplyDefaults() {
	if m.ResourceSyncer == nil {
		m.ResourceSyncer = NopResourceSyncer{}
	}
	if m.ConflictResolver == nil {
		m.ConflictResolver = NopConflictResolver{}
	}
	if m.ErrorHandler == nil {
		m.ErrorHandler = NopErrorHandler{Logger: m.Logger}
	}
	if m.MetricsCollector == nil {
		m.MetricsCollector = &DefaultMetricsCollector{}
	}
	if m.Logger == nil {
		m.Logger = DefaultLogger{}
	}
}
