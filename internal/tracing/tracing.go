// Package tracing provides OpenTelemetry distributed tracing support for Stromboli.
// It configures a trace provider with OTLP export and provides helper functions
// for creating and managing spans throughout the application.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds OpenTelemetry tracing configuration
type Config struct {
	// Enabled determines if tracing is active
	Enabled bool
	// ServiceName is the name of the service for traces
	ServiceName string
	// Endpoint is the OTLP collector endpoint (e.g., "localhost:4317")
	Endpoint string
	// Insecure disables TLS for the gRPC connection
	Insecure bool
}

// Span kind constants for convenience
const (
	SpanKindServer   = trace.SpanKindServer
	SpanKindClient   = trace.SpanKindClient
	SpanKindInternal = trace.SpanKindInternal
)

const (
	defaultServiceName = "stromboli"
	tracerName         = "stromboli"
)

// ShutdownFunc is a function that shuts down the trace provider
type ShutdownFunc func(context.Context) error

// Init initializes the OpenTelemetry trace provider.
// If tracing is disabled, it sets up a noop provider.
// Returns a shutdown function that must be called on application exit.
func Init(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	if !cfg.Enabled {
		// Set noop provider
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(ctx context.Context) error { return nil }, nil
	}

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = defaultServiceName
	}

	// Create OTLP exporter
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator for context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Return shutdown function
	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}, nil
}

// Tracer returns a named tracer for creating spans
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// StartSpan starts a new span with the given name.
// Returns the updated context and the span. The span must be ended with span.End().
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name, opts...)
}

// StartSpanWithKind starts a new span with a specific span kind.
func StartSpanWithKind(ctx context.Context, name string, kind trace.SpanKind) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name, trace.WithSpanKind(kind))
}

// SpanFromContext returns the current span from the context.
// Returns a noop span if no span is in the context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddSpanAttributes adds attributes to the current span in the context.
// Attributes should be provided as key-value pairs.
func AddSpanAttributes(ctx context.Context, kvs ...interface{}) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.IsRecording() {
		return
	}

	attrs := make([]attribute.KeyValue, 0, len(kvs)/2)
	for i := 0; i < len(kvs)-1; i += 2 {
		key, ok := kvs[i].(string)
		if !ok {
			continue
		}
		attrs = append(attrs, attributeFromValue(key, kvs[i+1]))
	}
	span.SetAttributes(attrs...)
}

// RecordError records an error on the current span in the context.
func RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.IsRecording() {
		return
	}
	span.RecordError(err, opts...)
}

// SetSpanStatus sets the status of the current span.
func SetSpanStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.IsRecording() {
		return
	}
	span.SetStatus(code, description)
}

// Status code constants for convenience
const (
	StatusOK    = codes.Ok
	StatusError = codes.Error
)

// attributeFromValue converts a value to an OTel attribute
func attributeFromValue(key string, value interface{}) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}
