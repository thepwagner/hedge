package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry/debian"
	"github.com/thepwagner/hedge/pkg/registry/npm"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

func RunServer(cfg Config) error {
	tp, err := newTracerProvider(cfg)
	if err != nil {
		return err
	}
	client := observability.NewHTTPClient(tp)
	tracer := tp.Tracer("hedge")

	r := mux.NewRouter()
	debHandler, err := debian.NewHandler(tracer, client, cfg.ConfigDir, cfg.Debian)
	if err != nil {
		return err
	}
	debHandler.Register(r)

	npmHandler, err := npm.NewHandler(tracer, client, cfg.ConfigDir, cfg.NPM)
	if err != nil {
		return err
	}
	npmHandler.Register(r)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: otelhttp.NewHandler(r, "ServeHTTP", otelhttp.WithTracerProvider(tp)),
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func newTracerProvider(cfg Config) (*trace.TracerProvider, error) {
	tpOptions := []trace.TracerProviderOption{
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("hedge"),
		)),
	}
	if cfg.TracerEndpoint != "" {
		jaegerOut, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.TracerEndpoint)))
		if err != nil {
			return nil, err
		}
		tpOptions = append(tpOptions, trace.WithBatcher(jaegerOut))
	}

	return trace.NewTracerProvider(tpOptions...), nil
}
