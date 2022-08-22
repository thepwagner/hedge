package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry/debian"
	"github.com/thepwagner/hedge/pkg/server"
	"github.com/thepwagner/hedge/proto/hedge/v1"
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

			var storage cached.ByteStorage
			storage = cached.InRedis(cfg.RedisAddr)
			storage = cached.WithTracer[string, []byte](tracer, storage)

			signed := cached.NewSignedCache(map[string][]byte{
				"foo": []byte("bar"),
			}, "foo", storage)

			for _, ep := range server.Ecosystems(tracer, client, storage) {
				eco := ep.Ecosystem()
				ecoLog := log.WithValues("ecosystem", eco)
				ecoCfg := cfg.Ecosystems[eco]
				ecoLog.Info("syncing ecosystem", "repository_count", len(ecoCfg.Repositories))

				// policyDir := filepath.Join(cfgDir, string(eco), "policies")

				direct := func(ctx context.Context, cfg *debian.RepositoryConfig) (*hedge.DebianPackages, error) {
					pkgs, err := ep.AllPackages(ctx, cfg)
					if err != nil {
						return nil, err
					}

					debs := make([]*hedge.DebianPackage, 0, len(pkgs))
					for _, pkg := range pkgs {
						deb := pkg.(debian.Package)
						debs = append(debs, &hedge.DebianPackage{
							Package:       deb.Package,
							Source:        deb.Source,
							Version:       deb.Version,
							InstalledSize: uint64(deb.InstalledSize()),
						})
					}
					return &hedge.DebianPackages{
						Packages: debs,
					}, nil
				}

				debStorage := cached.Race(tracer, "LoadPackages", map[string]cached.Function[*debian.RepositoryConfig, *hedge.DebianPackages]{
					"direct":                  direct,
					"redis+json":              cached.AsJSON(storage, 5*time.Minute, direct),
					"redis+json+gz":           cached.AsJSON(cached.WithGzip[string](cached.WithPrefix[[]byte]("gz", storage)), 5*time.Minute, direct),
					"redis+json+zstd":         cached.AsJSON(cached.WithZstd[string](cached.WithPrefix[[]byte]("zstd", storage)), 5*time.Minute, direct),
					"redis+proto":             cached.AsProtoBuf(cached.WithPrefix[[]byte]("proto", storage), 5*time.Minute, direct),
					"redis+proto+gz":          cached.AsProtoBuf(cached.WithGzip[string](cached.WithPrefix[[]byte]("proto_gz", storage)), 5*time.Minute, direct),
					"redis+proto+zstd":        cached.AsProtoBuf(cached.WithZstd[string](cached.WithPrefix[[]byte]("proto_zstd", storage)), 5*time.Minute, direct),
					"redis+signed+proto+zstd": cached.AsProtoBuf(cached.WithZstd[string](cached.WithPrefix[[]byte]("signed_proto_zstd", signed)), 5*time.Minute, direct),
					// "inmem+json":       cached.AsJSON(tracer, cached.WithPrefix[[]byte]("proto", inMemory), 5*time.Minute, direct),
					// "inmem+proto":      cached.AsProtoBuf(tracer, cached.WithPrefix[[]byte]("proto", inMemory), 5*time.Minute, direct),
				})

				for _, repo := range ecoCfg.Repositories {
					repoLog := ecoLog.WithValues("repository", repo.Name()).V(1)
					repoLog.Info("loading packages...")
					allPackages, err := debStorage(ctx, repo.(*debian.RepositoryConfig))
					if err != nil {
						return err
					}
					repoLog.Info("loaded packages repository", "package_count", len(allPackages.Packages))

					// ctx, filterSpan := tracer.Start(ctx, "FilterPackages")
					// pred, err := filter.CueConfigToPredicate[registry.Package](policyDir, repo.FilterConfig())
					// if err != nil {
					// 	filterSpan.End()
					// 	return err
					// }
					// var filtered []registry.Package
					// for _, pkg := range allPackages {
					// 	ok, err := pred(ctx, pkg)
					// 	if err != nil {
					// 		filterSpan.End()
					// 		return err
					// 	}
					// 	if ok {
					// 		repoLog.V(1).Info("accepted package", "package_name", pkg.GetName())
					// 		filtered = append(filtered, pkg)
					// 	}
					// }
					// if err != nil {
					// 	return err
					// }
					// repoLog.Info("filtered packages repository", "package_count", len(filtered))
				}
			}
			return nil
		},
	}
}
