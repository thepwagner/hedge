package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/debian"
	"github.com/thepwagner/hedge/pkg/npm"
	"github.com/thepwagner/hedge/pkg/observability"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

func RunServer(log logr.Logger, cfg Config) error {
	tp, err := newTracerProvider("http://192.168.1.23:14268/api/traces")
	if err != nil {
		return err
	}
	otel.SetTracerProvider(tp)
	client := observability.NewHTTPClient(tp)
	tracer := tp.Tracer("hedge")

	r := mux.NewRouter()
	if len(cfg.Debian) > 0 {
		log.V(1).Info("enabled debian support", "debian_repos", len(cfg.Debian))
		h, err := debian.NewHandler(tp, cfg.Debian)
		if err != nil {
			return err
		}
		h.Register(r)
	}

	if len(cfg.NPM) > 0 {
		log.V(1).Info("enabled NPM support", "npm", len(cfg.NPM))
		h, err := npm.NewHandler(tracer, client, cfg.NPM)
		if err != nil {
			return err
		}
		h.Register(r)
	}

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: otelhttp.NewHandler(r, "ServeHTTP", otelhttp.WithTracerProvider(tp)),
	}
	log.Info("starting server", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func newTracerProvider(url string) (*tracesdk.TracerProvider, error) {
	jaegerOut, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(jaegerOut),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("hedge"),
		)),
	)
	return tp, nil
}
