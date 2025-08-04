/*
Copyright 2024 The KCP Authors.

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

package tmc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	"k8s.io/klog/v2"
)

const (
	// Service name for tracing
	TMCServiceName = "kcp-tmc"

	// Common attribute keys
	TMCComponentTypeKey     = "tmc.component.type"
	TMCComponentIDKey       = "tmc.component.id"
	TMCOperationKey         = "tmc.operation"
	TMCClusterKey           = "tmc.cluster"
	TMCLogicalClusterKey    = "tmc.logical_cluster"
	TMCResourceGVKKey       = "tmc.resource.gvk"
	TMCResourceNameKey      = "tmc.resource.name"
	TMCResourceNamespaceKey = "tmc.resource.namespace"
	TMCErrorTypeKey         = "tmc.error.type"
	TMCErrorSeverityKey     = "tmc.error.severity"
	TMCStrategyKey          = "tmc.strategy"
	TMCPlacementKey         = "tmc.placement"
	TMCMigrationPhaseKey    = "tmc.migration.phase"
)

// TracingManager manages distributed tracing for TMC operations
type TracingManager struct {
	tracer     oteltrace.Tracer
	propagator propagation.TextMapPropagator
	mu         sync.RWMutex

	// Configuration
	serviceName    string
	serviceVersion string
	enabled        bool

	// Sampling configuration
	samplingRate float64

	// Active spans tracking
	activeSpans map[string]oteltrace.Span
	spanCounter int64
}

// NewTracingManager creates a new tracing manager
func NewTracingManager(serviceName, serviceVersion string) *TracingManager {
	tm := &TracingManager{
		serviceName:    serviceName,
		serviceVersion: serviceVersion,
		enabled:        true,
		samplingRate:   0.1, // 10% sampling by default
		activeSpans:    make(map[string]oteltrace.Span),
	}

	tm.initializeTracing()
	return tm
}

// initializeTracing sets up OpenTelemetry tracing
func (tm *TracingManager) initializeTracing() {
	// Create Jaeger exporter
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://localhost:14268/api/traces")))
	if err != nil {
		klog.Warning("Failed to create Jaeger exporter, tracing will be disabled", "error", err)
		tm.enabled = false
		return
	}

	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(tm.serviceName),
			semconv.ServiceVersionKey.String(tm.serviceVersion),
			attribute.String("service.type", "kcp-controller"),
		),
	)
	if err != nil {
		klog.Warning("Failed to create tracing resource", "error", err)
		tm.enabled = false
		return
	}

	// Create trace provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.TraceIDRatioBased(tm.samplingRate)),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	tm.propagator = propagation.TraceContext{}
	otel.SetTextMapPropagator(tm.propagator)

	// Get tracer
	tm.tracer = otel.Tracer(tm.serviceName)

	klog.Info("Distributed tracing initialized",
		"serviceName", tm.serviceName,
		"serviceVersion", tm.serviceVersion,
		"samplingRate", tm.samplingRate)
}

// IsEnabled returns whether tracing is enabled
func (tm *TracingManager) IsEnabled() bool {
	return tm.enabled
}

// StartSpan starts a new trace span with TMC-specific attributes
func (tm *TracingManager) StartSpan(ctx context.Context, operationName string, options ...TMCSpanOption) (context.Context, oteltrace.Span) {
	if !tm.enabled {
		return ctx, oteltrace.SpanFromContext(ctx)
	}

	spanOptions := []oteltrace.SpanStartOption{
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal),
	}

	// Apply TMC-specific options
	for _, option := range options {
		option.apply(&spanOptions)
	}

	ctx, span := tm.tracer.Start(ctx, operationName, spanOptions...)

	// Track active span
	spanID := tm.generateSpanID()
	tm.mu.Lock()
	tm.activeSpans[spanID] = span
	tm.spanCounter++
	tm.mu.Unlock()

	// Set TMC service attribute
	span.SetAttributes(attribute.String("service.name", tm.serviceName))

	// Clean up when span ends
	go func() {
		<-ctx.Done()
		tm.mu.Lock()
		delete(tm.activeSpans, spanID)
		tm.mu.Unlock()
	}()

	return ctx, span
}

// StartComponentSpan starts a span for a TMC component operation
func (tm *TracingManager) StartComponentSpan(ctx context.Context, componentType ComponentType, componentID, operation string) (context.Context, oteltrace.Span) {
	spanName := fmt.Sprintf("%s.%s", componentType, operation)

	return tm.StartSpan(ctx, spanName,
		WithComponent(componentType, componentID),
		WithOperation(operation),
	)
}

// StartClusterSpan starts a span for cluster-specific operations
func (tm *TracingManager) StartClusterSpan(ctx context.Context, operation, clusterName, logicalCluster string) (context.Context, oteltrace.Span) {
	spanName := fmt.Sprintf("cluster.%s", operation)

	return tm.StartSpan(ctx, spanName,
		WithOperation(operation),
		WithCluster(clusterName, logicalCluster),
	)
}

// StartResourceSpan starts a span for resource operations
func (tm *TracingManager) StartResourceSpan(ctx context.Context, operation, gvk, namespace, name string) (context.Context, oteltrace.Span) {
	spanName := fmt.Sprintf("resource.%s", operation)

	return tm.StartSpan(ctx, spanName,
		WithOperation(operation),
		WithResource(gvk, namespace, name),
	)
}

// StartMigrationSpan starts a span for migration operations
func (tm *TracingManager) StartMigrationSpan(ctx context.Context, operation, sourceCluster, targetCluster, phase string) (context.Context, oteltrace.Span) {
	spanName := fmt.Sprintf("migration.%s", operation)

	return tm.StartSpan(ctx, spanName,
		WithOperation(operation),
		WithCluster(sourceCluster, ""),
		WithMigrationPhase(phase),
		WithAttribute("tmc.target_cluster", targetCluster),
	)
}

// RecordError records an error in the current span
func (tm *TracingManager) RecordError(span oteltrace.Span, err *TMCError) {
	if !tm.enabled || span == nil {
		return
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Message)
	span.SetAttributes(
		attribute.String(TMCErrorTypeKey, string(err.Type)),
		attribute.String(TMCErrorSeverityKey, string(err.Severity)),
		attribute.String("error.component", err.Component),
		attribute.String("error.operation", err.Operation),
	)

	if err.ClusterName != "" {
		span.SetAttributes(attribute.String(TMCClusterKey, err.ClusterName))
	}

	if err.GVK.Kind != "" {
		span.SetAttributes(attribute.String(TMCResourceGVKKey, err.GVK.String()))
	}
}

// AddEvent adds an event to the current span
func (tm *TracingManager) AddEvent(span oteltrace.Span, name string, attributes ...attribute.KeyValue) {
	if !tm.enabled || span == nil {
		return
	}

	span.AddEvent(name, oteltrace.WithAttributes(attributes...))
}

// SetAttributes sets attributes on the current span
func (tm *TracingManager) SetAttributes(span oteltrace.Span, attributes ...attribute.KeyValue) {
	if !tm.enabled || span == nil {
		return
	}

	span.SetAttributes(attributes...)
}

// InjectHeaders injects tracing context into HTTP headers
func (tm *TracingManager) InjectHeaders(ctx context.Context, headers map[string]string) {
	if !tm.enabled {
		return
	}

	tm.propagator.Inject(ctx, propagation.MapCarrier(headers))
}

// ExtractContext extracts tracing context from HTTP headers
func (tm *TracingManager) ExtractContext(ctx context.Context, headers map[string]string) context.Context {
	if !tm.enabled {
		return ctx
	}

	return tm.propagator.Extract(ctx, propagation.MapCarrier(headers))
}

func (tm *TracingManager) generateSpanID() string {
	tm.spanCounter++
	return fmt.Sprintf("span-%d-%d", time.Now().UnixNano(), tm.spanCounter)
}

// GetActiveSpansCount returns the number of active spans
func (tm *TracingManager) GetActiveSpansCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.activeSpans)
}

// TMCSpanOption represents options for creating TMC spans
type TMCSpanOption interface {
	apply(options *[]oteltrace.SpanStartOption)
}

type spanOptionFunc func(*[]oteltrace.SpanStartOption)

func (f spanOptionFunc) apply(options *[]oteltrace.SpanStartOption) {
	f(options)
}

// WithComponent adds component information to a span
func WithComponent(componentType ComponentType, componentID string) TMCSpanOption {
	return spanOptionFunc(func(options *[]oteltrace.SpanStartOption) {
		*options = append(*options, oteltrace.WithAttributes(
			attribute.String(TMCComponentTypeKey, string(componentType)),
			attribute.String(TMCComponentIDKey, componentID),
		))
	})
}

// WithOperation adds operation information to a span
func WithOperation(operation string) TMCSpanOption {
	return spanOptionFunc(func(options *[]oteltrace.SpanStartOption) {
		*options = append(*options, oteltrace.WithAttributes(
			attribute.String(TMCOperationKey, operation),
		))
	})
}

// WithCluster adds cluster information to a span
func WithCluster(clusterName, logicalCluster string) TMCSpanOption {
	return spanOptionFunc(func(options *[]oteltrace.SpanStartOption) {
		attrs := []attribute.KeyValue{
			attribute.String(TMCClusterKey, clusterName),
		}
		if logicalCluster != "" {
			attrs = append(attrs, attribute.String(TMCLogicalClusterKey, logicalCluster))
		}
		*options = append(*options, oteltrace.WithAttributes(attrs...))
	})
}

// WithResource adds resource information to a span
func WithResource(gvk, namespace, name string) TMCSpanOption {
	return spanOptionFunc(func(options *[]oteltrace.SpanStartOption) {
		attrs := []attribute.KeyValue{
			attribute.String(TMCResourceGVKKey, gvk),
			attribute.String(TMCResourceNameKey, name),
		}
		if namespace != "" {
			attrs = append(attrs, attribute.String(TMCResourceNamespaceKey, namespace))
		}
		*options = append(*options, oteltrace.WithAttributes(attrs...))
	})
}

// WithStrategy adds strategy information to a span
func WithStrategy(strategy string) TMCSpanOption {
	return spanOptionFunc(func(options *[]oteltrace.SpanStartOption) {
		*options = append(*options, oteltrace.WithAttributes(
			attribute.String(TMCStrategyKey, strategy),
		))
	})
}

// WithPlacement adds placement information to a span
func WithPlacement(placement string) TMCSpanOption {
	return spanOptionFunc(func(options *[]oteltrace.SpanStartOption) {
		*options = append(*options, oteltrace.WithAttributes(
			attribute.String(TMCPlacementKey, placement),
		))
	})
}

// WithMigrationPhase adds migration phase information to a span
func WithMigrationPhase(phase string) TMCSpanOption {
	return spanOptionFunc(func(options *[]oteltrace.SpanStartOption) {
		*options = append(*options, oteltrace.WithAttributes(
			attribute.String(TMCMigrationPhaseKey, phase),
		))
	})
}

// WithAttribute adds a custom attribute to a span
func WithAttribute(key, value string) TMCSpanOption {
	return spanOptionFunc(func(options *[]oteltrace.SpanStartOption) {
		*options = append(*options, oteltrace.WithAttributes(
			attribute.String(key, value),
		))
	})
}

// WithSpanKind sets the span kind
func WithSpanKind(kind oteltrace.SpanKind) TMCSpanOption {
	return spanOptionFunc(func(options *[]oteltrace.SpanStartOption) {
		*options = append(*options, oteltrace.WithSpanKind(kind))
	})
}

// TraceableContext provides a context wrapper with tracing capabilities
type TraceableContext struct {
	context.Context
	tracingManager *TracingManager
	span           oteltrace.Span
}

// NewTraceableContext creates a new traceable context
func (tm *TracingManager) NewTraceableContext(ctx context.Context, operationName string, options ...TMCSpanOption) *TraceableContext {
	tracedCtx, span := tm.StartSpan(ctx, operationName, options...)

	return &TraceableContext{
		Context:        tracedCtx,
		tracingManager: tm,
		span:           span,
	}
}

// RecordError records an error in the context's span
func (tc *TraceableContext) RecordError(err *TMCError) {
	tc.tracingManager.RecordError(tc.span, err)
}

// AddEvent adds an event to the context's span
func (tc *TraceableContext) AddEvent(name string, attributes ...attribute.KeyValue) {
	tc.tracingManager.AddEvent(tc.span, name, attributes...)
}

// SetAttributes sets attributes on the context's span
func (tc *TraceableContext) SetAttributes(attributes ...attribute.KeyValue) {
	tc.tracingManager.SetAttributes(tc.span, attributes...)
}

// Finish completes the span
func (tc *TraceableContext) Finish() {
	tc.span.End()
}

// FinishWithError completes the span with an error
func (tc *TraceableContext) FinishWithError(err *TMCError) {
	tc.RecordError(err)
	tc.span.End()
}

// TracingMiddleware provides middleware for adding tracing to operations
type TracingMiddleware struct {
	tracingManager *TracingManager
}

// NewTracingMiddleware creates a new tracing middleware
func NewTracingMiddleware(tracingManager *TracingManager) *TracingMiddleware {
	return &TracingMiddleware{
		tracingManager: tracingManager,
	}
}

// WrapOperation wraps an operation with tracing
func (tm *TracingMiddleware) WrapOperation(operationName string, operation func(ctx context.Context) error, options ...TMCSpanOption) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !tm.tracingManager.IsEnabled() {
			return operation(ctx)
		}

		tracedCtx, span := tm.tracingManager.StartSpan(ctx, operationName, options...)
		defer span.End()

		err := operation(tracedCtx)
		if err != nil {
			if tmcErr, ok := err.(*TMCError); ok {
				tm.tracingManager.RecordError(span, tmcErr)
			} else {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
		}

		return err
	}
}

// WrapComponentOperation wraps a component operation with tracing
func (tm *TracingMiddleware) WrapComponentOperation(
	componentType ComponentType,
	componentID, operation string,
	operationFunc func(ctx context.Context) error,
) func(ctx context.Context) error {
	return tm.WrapOperation(
		fmt.Sprintf("%s.%s", componentType, operation),
		operationFunc,
		WithComponent(componentType, componentID),
		WithOperation(operation),
	)
}

// TraceSpan is a convenience function to trace a function execution
func TraceSpan(ctx context.Context, tm *TracingManager, operationName string, fn func(ctx context.Context) error, options ...TMCSpanOption) error {
	if !tm.IsEnabled() {
		return fn(ctx)
	}

	tracedCtx, span := tm.StartSpan(ctx, operationName, options...)
	defer span.End()

	err := fn(tracedCtx)
	if err != nil {
		if tmcErr, ok := err.(*TMCError); ok {
			tm.RecordError(span, tmcErr)
		} else {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}

	return err
}

// GetTracingManager returns a singleton tracing manager for the TMC system
var (
	globalTracingManager *TracingManager
	tracingOnce          sync.Once
)

// GetGlobalTracingManager returns the global tracing manager
func GetGlobalTracingManager() *TracingManager {
	tracingOnce.Do(func() {
		globalTracingManager = NewTracingManager(TMCServiceName, "v1.0.0")
	})
	return globalTracingManager
}

// Helper functions for common tracing patterns

// TraceComponentOperation traces a component operation
func TraceComponentOperation(ctx context.Context, componentType ComponentType, componentID, operation string, fn func(ctx context.Context) error) error {
	tm := GetGlobalTracingManager()
	return TraceSpan(ctx, tm, fmt.Sprintf("%s.%s", componentType, operation), fn,
		WithComponent(componentType, componentID),
		WithOperation(operation),
	)
}

// TraceClusterOperation traces a cluster operation
func TraceClusterOperation(ctx context.Context, operation, clusterName, logicalCluster string, fn func(ctx context.Context) error) error {
	tm := GetGlobalTracingManager()
	return TraceSpan(ctx, tm, fmt.Sprintf("cluster.%s", operation), fn,
		WithOperation(operation),
		WithCluster(clusterName, logicalCluster),
	)
}

// TraceResourceOperation traces a resource operation
func TraceResourceOperation(ctx context.Context, operation, gvk, namespace, name string, fn func(ctx context.Context) error) error {
	tm := GetGlobalTracingManager()
	return TraceSpan(ctx, tm, fmt.Sprintf("resource.%s", operation), fn,
		WithOperation(operation),
		WithResource(gvk, namespace, name),
	)
}
