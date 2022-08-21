package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/thepwagner/hedge/pkg/cache"
	"github.com/thepwagner/hedge/pkg/filter"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry"
	"github.com/thepwagner/hedge/pkg/server"
	"github.com/urfave/cli/v2"
)

func SyncCommand(log logr.Logger) *cli.Command {
	return &cli.Command{
		Name: "sync",
		Action: func(c *cli.Context) error {
			// Load configuration
			cfgDir := c.String(flagConfigDirectory)
			cfg, err := server.LoadConfig(cfgDir)
			if err != nil {
				return err
			}
			log.Info("loaded configuration", "ecosystem_count", len(cfg.Ecosystems))

			// Init tracing:
			tp, err := newTracerProvider(cfg)
			if err != nil {
				return err
			}
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = tp.Shutdown(ctx)
			}()
			tracer := tp.Tracer("hedge")
			client := observability.NewHTTPClient(tp)
			ctx, span := tracer.Start(c.Context, "SyncCommand")
			defer span.End()
			traceID := span.SpanContext().TraceID()
			log.Info("tracing", "trace_id", traceID)
			fmt.Printf("http://riker.pwagner.net:16686/trace/%s\n", traceID)

			redisCache := cache.NewRedis(cfg.RedisAddr)
			storage := cache.NewTracedCache[[]byte](tracer, redisCache)

			for _, ep := range server.Ecosystems(tracer, client, storage) {
				eco := ep.Ecosystem()
				ecoLog := log.WithValues("ecosystem", eco)
				ecoCfg := cfg.Ecosystems[eco]
				ecoLog.Info("syncing ecosystem", "repository_count", len(ecoCfg.Repositories))

				policyDir := filepath.Join(cfgDir, string(eco), "policies")

				for _, repo := range ecoCfg.Repositories {
					repoLog := ecoLog.WithValues("repository", repo.Name()).V(1)
					allPackages, err := ep.AllPackages(ctx, repo)
					if err != nil {
						return err
					}
					repoLog.Info("loaded packages repository", "package_count", len(allPackages))

					ctx, filterSpan := tracer.Start(ctx, "FilterPackages")
					pred, err := filter.CueConfigToPredicate[registry.Package](policyDir, repo.FilterConfig())
					if err != nil {
						filterSpan.End()
						return err
					}
					var filtered []registry.Package
					for _, pkg := range allPackages {
						ok, err := pred(ctx, pkg)
						if err != nil {
							filterSpan.End()
							return err
						}
						if ok {
							repoLog.V(1).Info("accepted package", "package_name", pkg.GetName())
							filtered = append(filtered, pkg)
						}
					}
					if err != nil {
						return err
					}
					repoLog.Info("filtered packages repository", "package_count", len(filtered))
				}
			}
			return nil
		},
	}
}
