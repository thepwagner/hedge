package cli

import (
	"github.com/go-logr/logr"
	"github.com/thepwagner/hedge/pkg/server"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

const (
	flagConfigDirectory = "config-directory"
)

func App(log logr.Logger) *cli.App {
	return &cli.App{
		Name:        "hedge",
		Description: "Package proxy",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  flagConfigDirectory,
				Value: "./pkg/server/testdata/config/",
			},
		},
		Commands: []*cli.Command{
			ServerCommand(),
			SyncCommand(log),
		},
	}
}

func newTracerProvider(cfg *server.Config) (*sdktrace.TracerProvider, error) {
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
