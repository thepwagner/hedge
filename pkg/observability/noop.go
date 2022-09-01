package observability

import "go.opentelemetry.io/otel/trace"

var (
	NoopTraceProvider = trace.NewNoopTracerProvider()
	NoopTracer        = NoopTraceProvider.Tracer("")
)
