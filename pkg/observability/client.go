package observability

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

func NewHTTPClient(tp trace.TracerProvider) *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithTracerProvider(tp)),
	}
}
