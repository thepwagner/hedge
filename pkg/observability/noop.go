package observability

import "go.opentelemetry.io/otel/trace"

var NoopTracer = trace.NewNoopTracerProvider().Tracer("")
