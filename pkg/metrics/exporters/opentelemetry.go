/*
Copyright 2023 The KCP Authors.

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

package exporters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/klog/v2"

	tmcmetrics "github.com/kcp-dev/kcp/pkg/metrics"
)

// OpenTelemetryExporter implements the MetricExporter interface for OpenTelemetry metrics and traces.
// It provides integration with OpenTelemetry SDKs for metrics and tracing.
type OpenTelemetryExporter struct {
	mu sync.RWMutex

	// Configuration
	endpoint        string
	serviceName     string
	serviceVersion  string
	exportInterval  time.Duration

	// OpenTelemetry components
	tracer         trace.Tracer
	meter          metric.Meter
	traceProvider  *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider

	// Registry reference
	registry *tmcmetrics.MetricsRegistry

	// Collection state
	collectTicker *time.Ticker
	stopChan      chan struct{}
}

// OpenTelemetryExporterOptions configures the OpenTelemetry exporter.
type OpenTelemetryExporterOptions struct {
	Endpoint       string
	ServiceName    string
	ServiceVersion string
	ExportInterval time.Duration
}

// DefaultOpenTelemetryExporterOptions returns default configuration for the OpenTelemetry exporter.
func DefaultOpenTelemetryExporterOptions() *OpenTelemetryExporterOptions {
	return &OpenTelemetryExporterOptions{
		Endpoint:       "localhost:4317",
		ServiceName:    "kcp-tmc",
		ServiceVersion: "v0.1.0",
		ExportInterval: 15 * time.Second,
	}
}

// NewOpenTelemetryExporter creates a new OpenTelemetry metrics and traces exporter.
func NewOpenTelemetryExporter(registry *tmcmetrics.MetricsRegistry, opts *OpenTelemetryExporterOptions) *OpenTelemetryExporter {
	if opts == nil {
		opts = DefaultOpenTelemetryExporterOptions()
	}

	exporter := &OpenTelemetryExporter{
		endpoint:       opts.Endpoint,
		serviceName:    opts.ServiceName,
		serviceVersion: opts.ServiceVersion,
		exportInterval: opts.ExportInterval,
		registry:       registry,
		stopChan:       make(chan struct{}),
	}

	return exporter
}

// Name returns the exporter name.
func (e *OpenTelemetryExporter) Name() string {
	return "opentelemetry"
}

// Start begins the OpenTelemetry metrics and traces export.
func (e *OpenTelemetryExporter) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Initialize trace provider
	if err := e.initTraceProvider(ctx); err != nil {
		return err
	}

	// Initialize meter provider
	if err := e.initMeterProvider(ctx); err != nil {
		return err
	}

	// Set global providers
	otel.SetTracerProvider(e.traceProvider)
	otel.SetMeterProvider(e.meterProvider)

	// Set text map propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Get tracer and meter
	e.tracer = otel.Tracer("github.com/kcp-dev/kcp/metrics")
	e.meter = otel.Meter("github.com/kcp-dev/kcp/metrics")

	// Start periodic export
	e.collectTicker = time.NewTicker(e.exportInterval)
	go e.exportLoop(ctx)

	klog.Infof("Started OpenTelemetry exporter (endpoint: %s, service: %s)", 
		e.endpoint, e.serviceName)
	return nil
}

// Stop gracefully shuts down the OpenTelemetry exporter.
func (e *OpenTelemetryExporter) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Stop export loop
	if e.collectTicker != nil {
		e.collectTicker.Stop()
	}
	close(e.stopChan)

	// Shutdown providers
	if e.traceProvider != nil {
		if err := e.traceProvider.Shutdown(ctx); err != nil {
			klog.Errorf("Failed to shutdown trace provider: %v", err)
		}
	}

	if e.meterProvider != nil {
		if err := e.meterProvider.Shutdown(ctx); err != nil {
			klog.Errorf("Failed to shutdown meter provider: %v", err)
		}
	}

	klog.Info("OpenTelemetry exporter stopped")
	return nil
}

// initTraceProvider initializes the OpenTelemetry trace provider.
func (e *OpenTelemetryExporter) initTraceProvider(ctx context.Context) error {
	// Create OTLP trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(e.endpoint),
		otlptracegrpc.WithInsecure(), // TODO: Configure TLS properly
	)
	if err != nil {
		return err
	}

	// Create trace provider
	e.traceProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(e.createResource()),
	)

	return nil
}

// initMeterProvider initializes the OpenTelemetry meter provider.
func (e *OpenTelemetryExporter) initMeterProvider(ctx context.Context) error {
	// For now, use a simple periodic reader
	// In production, you might want to use OTLP metric exporter
	reader := sdkmetric.NewPeriodicReader(
		sdkmetric.NewManualReader(), // Placeholder - would use OTLP metric exporter
		sdkmetric.WithInterval(e.exportInterval),
	)

	e.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(e.createResource()),
		sdkmetric.WithReader(reader),
	)

	return nil
}

// createResource creates an OpenTelemetry resource with service information.
func (e *OpenTelemetryExporter) createResource() *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(e.serviceName),
		semconv.ServiceVersion(e.serviceVersion),
		attribute.String("component", "kcp-tmc-metrics"),
	)
}

// exportLoop periodically exports metrics and traces.
func (e *OpenTelemetryExporter) exportLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-e.collectTicker.C:
			e.exportMetrics(ctx)
		}
	}
}

// exportMetrics exports current metrics through OpenTelemetry.
func (e *OpenTelemetryExporter) exportMetrics(ctx context.Context) {
	// Collect from all collectors first
	if err := e.registry.CollectAll(); err != nil {
		klog.Errorf("Failed to collect metrics for OTel export: %v", err)
		return
	}

	// Create a span to track the export operation
	ctx, span := e.tracer.Start(ctx, "export_metrics")
	defer span.End()

	// TODO: In a full implementation, this would convert Prometheus metrics
	// to OpenTelemetry metrics and export them. For now, we just create
	// a trace span to show the integration works.

	span.SetAttributes(
		attribute.Int64("export.timestamp", time.Now().Unix()),
		attribute.String("export.type", "metrics"),
	)

	klog.V(4).Info("Exported metrics through OpenTelemetry")
}

// CreateSpan creates a new trace span for operation tracking.
// This method is provided for other TMC components to create spans.
func (e *OpenTelemetryExporter) CreateSpan(ctx context.Context, operationName string) (context.Context, trace.Span) {
	if e.tracer == nil {
		// Return a no-op span if tracer is not initialized
		return ctx, trace.SpanFromContext(ctx)
	}
	return e.tracer.Start(ctx, operationName)
}

// GetMeter returns the OpenTelemetry meter for creating instruments.
func (e *OpenTelemetryExporter) GetMeter() metric.Meter {
	return e.meter
}

// GetTracer returns the OpenTelemetry tracer for creating spans.
func (e *OpenTelemetryExporter) GetTracer() trace.Tracer {
	return e.tracer
}

// Helper methods for creating OpenTelemetry instruments

// CreateCounter creates an OpenTelemetry counter instrument.
func (e *OpenTelemetryExporter) CreateCounter(name, description string) (metric.Int64Counter, error) {
	if e.meter == nil {
		return nil, fmt.Errorf("meter not initialized")
	}
	
	return e.meter.Int64Counter(
		name,
		metric.WithDescription(description),
	)
}

// CreateGauge creates an OpenTelemetry gauge instrument.
func (e *OpenTelemetryExporter) CreateGauge(name, description string) (metric.Int64Gauge, error) {
	if e.meter == nil {
		return nil, fmt.Errorf("meter not initialized")
	}
	
	return e.meter.Int64Gauge(
		name,
		metric.WithDescription(description),
	)
}

// CreateHistogram creates an OpenTelemetry histogram instrument.
func (e *OpenTelemetryExporter) CreateHistogram(name, description string) (metric.Float64Histogram, error) {
	if e.meter == nil {
		return nil, fmt.Errorf("meter not initialized")
	}
	
	return e.meter.Float64Histogram(
		name,
		metric.WithDescription(description),
	)
}