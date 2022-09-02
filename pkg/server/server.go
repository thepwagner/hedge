package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry/base"
	"github.com/thepwagner/hedge/pkg/registry/debian"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
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

func newMuxRouter(ctx context.Context, tp *sdktrace.TracerProvider, cfg Config) (http.Handler, error) {
	tracer := tp.Tracer("hedge")
	_, span := tracer.Start(ctx, "newMuxRouter")
	defer span.End()

	fmt.Printf("http://riker.pwagner.net:16686/trace/%s\n", span.SpanContext().TraceID())

	client := observability.NewHTTPClient(tp)

	// Use a traced redis cache for storage:
	storage := cached.InRedis(cfg.RedisAddr, tp)

	bh := base.NewHandler(tracer, storage)
	debian.NewHandler(bh, tracer, storage, client, cfg.Ecosystems[debian.Ecosystem])

	// for _, ep := range Ecosystems(tracer, client, storage) {
	// 	eco := ep.Ecosystem()
	// 	ecoCfg := cfg.Ecosystems[eco]
	// 	if len(ecoCfg.Repositories) == 0 {
	// 		continue
	// 	}

	// 	span.AddEvent("register ecosystem handler", trace.WithAttributes(
	// 		observability.Ecosystem(eco),
	// 		attribute.Int("repository_count", len(ecoCfg.Repositories)),
	// 	))

	// 	h, err := ep.NewHandler(registry.HandlerArgs{
	// 		Tracer:      tracer,
	// 		Client:      client,
	// 		ByteStorage: storage,
	// 		Ecosystem:   ecoCfg,
	// 	})
	// 	if err != nil {
	// 		span.RecordError(err, trace.WithAttributes(observability.Ecosystem(eco)))
	// 		span.SetStatus(codes.Error, err.Error())
	// 		return nil, err
	// 	}
	// 	h.Register(r)
	// }
	return bh, nil
}
