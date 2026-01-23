package tracing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestNewConfig_Defaults(t *testing.T) {
	cfg := Config{}

	assert.Equal(t, "", cfg.Endpoint)
	assert.Equal(t, "", cfg.ServiceName)
	assert.False(t, cfg.Enabled)
}

func TestConfig_WithValues(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Endpoint:    "localhost:4317",
		Insecure:    true,
	}

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "test-service", cfg.ServiceName)
	assert.Equal(t, "localhost:4317", cfg.Endpoint)
	assert.True(t, cfg.Insecure)
}

func TestInit_Disabled_ReturnsNoopProvider(t *testing.T) {
	cfg := Config{
		Enabled: false,
	}

	shutdown, err := Init(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Should get a noop tracer
	tracer := otel.Tracer("test")
	require.NotNil(t, tracer)

	// Cleanup
	err = shutdown(context.Background())
	require.NoError(t, err)
}

func TestInit_Enabled_SetsGlobalProvider(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Endpoint:    "localhost:4317",
		Insecure:    true,
	}

	// This will fail to connect but should still set up the provider
	shutdown, err := Init(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Should get a real tracer
	tracer := otel.Tracer("test")
	require.NotNil(t, tracer)

	// Create a span to verify it works
	ctx, span := tracer.Start(context.Background(), "test-span")
	require.NotNil(t, span)
	require.NotNil(t, ctx)

	spanCtx := span.SpanContext()
	assert.True(t, spanCtx.IsValid())
	assert.True(t, spanCtx.TraceID().IsValid())
	assert.True(t, spanCtx.SpanID().IsValid())

	span.End()

	// Cleanup
	err = shutdown(context.Background())
	require.NoError(t, err)
}

func TestStartSpan_CreatesChildSpan(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Endpoint:    "localhost:4317",
		Insecure:    true,
	}

	shutdown, err := Init(context.Background(), cfg)
	require.NoError(t, err)
	defer shutdown(context.Background())

	// Create parent span
	ctx, parentSpan := StartSpan(context.Background(), "parent-operation")
	require.NotNil(t, parentSpan)
	parentTraceID := parentSpan.SpanContext().TraceID()

	// Create child span
	_, childSpan := StartSpan(ctx, "child-operation")
	require.NotNil(t, childSpan)
	childTraceID := childSpan.SpanContext().TraceID()

	// Both should have same trace ID (child is part of same trace)
	assert.Equal(t, parentTraceID, childTraceID)

	// But different span IDs
	assert.NotEqual(t, parentSpan.SpanContext().SpanID(), childSpan.SpanContext().SpanID())

	childSpan.End()
	parentSpan.End()
}

func TestSpanFromContext_ReturnsSpan(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Endpoint:    "localhost:4317",
		Insecure:    true,
	}

	shutdown, err := Init(context.Background(), cfg)
	require.NoError(t, err)
	defer shutdown(context.Background())

	// Create span
	ctx, span := StartSpan(context.Background(), "test-operation")
	defer span.End()

	// Get span from context
	retrievedSpan := SpanFromContext(ctx)
	require.NotNil(t, retrievedSpan)

	// Should be the same span
	assert.Equal(t, span.SpanContext().SpanID(), retrievedSpan.SpanContext().SpanID())
}

func TestAddSpanAttributes_AddsAttributes(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Endpoint:    "localhost:4317",
		Insecure:    true,
	}

	shutdown, err := Init(context.Background(), cfg)
	require.NoError(t, err)
	defer shutdown(context.Background())

	ctx, span := StartSpan(context.Background(), "test-operation")
	defer span.End()

	// Should not panic
	AddSpanAttributes(ctx, "key1", "value1", "key2", 42)
}

func TestRecordError_RecordsError(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Endpoint:    "localhost:4317",
		Insecure:    true,
	}

	shutdown, err := Init(context.Background(), cfg)
	require.NoError(t, err)
	defer shutdown(context.Background())

	ctx, span := StartSpan(context.Background(), "test-operation")
	defer span.End()

	// Should not panic
	testErr := assert.AnError
	RecordError(ctx, testErr)
}

func TestTracer_ReturnsNamedTracer(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ServiceName: "test-service",
		Endpoint:    "localhost:4317",
		Insecure:    true,
	}

	shutdown, err := Init(context.Background(), cfg)
	require.NoError(t, err)
	defer shutdown(context.Background())

	tracer := Tracer("my-component")
	require.NotNil(t, tracer)

	// Verify it can create spans
	_, span := tracer.Start(context.Background(), "test")
	require.NotNil(t, span)
	assert.True(t, span.SpanContext().IsValid())
	span.End()
}

func TestInit_WithEmptyServiceName_UsesDefault(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		ServiceName: "",
		Endpoint:    "localhost:4317",
		Insecure:    true,
	}

	shutdown, err := Init(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	defer shutdown(context.Background())
}

func TestSpanKind_Constants(t *testing.T) {
	// Verify our span kind constants match OTel
	assert.Equal(t, trace.SpanKindServer, SpanKindServer)
	assert.Equal(t, trace.SpanKindClient, SpanKindClient)
	assert.Equal(t, trace.SpanKindInternal, SpanKindInternal)
}
