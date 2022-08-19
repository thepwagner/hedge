package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/pkg/observability"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
)

func RunServer(ctx context.Context, cfg Config) error {
	tp, err := newTracerProvider(cfg)
	if err != nil {
		return err
	}

	router, err := newMuxRouter(ctx, tp, cfg)
	if err != nil {
		return err
	}

	handler := otelhttp.NewHandler(router, "ServeHTTP", otelhttp.WithTracerProvider(tp))
	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func newTracerProvider(cfg Config) (*sdktrace.TracerProvider, error) {
	tpOptions := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("hedge"),
		)),
	}
	if cfg.TracerEndpoint != "" {
		jaegerOut, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.TracerEndpoint)))
		if err != nil {
			return nil, err
		}
		tpOptions = append(tpOptions, sdktrace.WithBatcher(jaegerOut))
	}

	return sdktrace.NewTracerProvider(tpOptions...), nil
}

func newMuxRouter(ctx context.Context, tp *sdktrace.TracerProvider, cfg Config) (*mux.Router, error) {
	tracer := tp.Tracer("hedge")
	_, span := tracer.Start(ctx, "newMuxRouter")
	defer span.End()

	client := observability.NewHTTPClient(tp)
	r := mux.NewRouter()

	for _, ep := range ecosystems {
		eco := ep.Ecosystem()
		ecoCfg := cfg.Ecosystems[eco]
		if len(ecoCfg.Repositories) == 0 {
			continue
		}

		span.AddEvent("register ecosystem handler", trace.WithAttributes(
			observability.Ecosystem(eco),
			attribute.Int("repository_count", len(ecoCfg.Repositories)),
		))

		h, err := ep.NewHandler(tracer, client, ecoCfg)
		if err != nil {
			span.RecordError(err, trace.WithAttributes(observability.Ecosystem(eco)))
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		h.Register(r)
	}
	return r, nil
}
