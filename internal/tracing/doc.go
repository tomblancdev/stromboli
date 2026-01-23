// Package tracing provides OpenTelemetry distributed tracing support for Stromboli.
//
// # Configuration
//
// Tracing is configured via environment variables or config file:
//
//	STROMBOLI_TRACING_ENABLED=true
//	STROMBOLI_TRACING_SERVICE_NAME=stromboli
//	STROMBOLI_TRACING_ENDPOINT=localhost:4317
//	STROMBOLI_TRACING_INSECURE=true
//
// # Usage
//
// Initialize tracing at application startup:
//
//	shutdown, err := tracing.Init(ctx, tracing.Config{
//	    Enabled:     true,
//	    ServiceName: "stromboli",
//	    Endpoint:    "localhost:4317",
//	    Insecure:    true,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer shutdown(ctx)
//
// Create spans in your code:
//
//	ctx, span := tracing.StartSpan(ctx, "operation-name")
//	defer span.End()
//
//	// Add attributes
//	tracing.AddSpanAttributes(ctx, "key", "value", "count", 42)
//
//	// Record errors
//	if err != nil {
//	    tracing.RecordError(ctx, err)
//	}
//
// # Gin Middleware
//
// For HTTP tracing, use the otelgin middleware in your routes:
//
//	import "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
//
//	router.Use(otelgin.Middleware("stromboli"))
package tracing
