package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/filter"
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
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = tp.Shutdown(shutdownCtx)
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

			// signed := cached.NewSignedCache(map[string][]byte{
			// 	"foo": []byte("bar"),
			// }, "foo", storage)

			for _, ep := range server.Ecosystems(tracer, client, storage) {
				eco := ep.Ecosystem()
				ecoLog := log.WithValues("ecosystem", eco)
				ecoCfg := cfg.Ecosystems[eco]
				ecoLog.Info("syncing ecosystem", "repository_count", len(ecoCfg.Repositories))

				policyDir := filepath.Join(cfgDir, string(eco), "policies")

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
							Priority:      deb.Priority,
							InstalledSize: uint64(deb.InstalledSize()),
						})
					}
					return &hedge.DebianPackages{
						Packages: debs,
					}, nil
				}

				debStorage := cached.Race(tracer, "LoadPackages", map[string]cached.Function[*debian.RepositoryConfig, *hedge.DebianPackages]{
					// "direct":                  direct,
					// "redis+json":              cached.Wrap(storage, direct),
					// "redis+json+gz":           cached.Wrap(cached.WithGzip[string, []byte](cached.WithPrefix[string, []byte]("gz", storage)), direct),
					// "redis+json+zstd":         cached.Wrap(cached.WithZstd[string, []byte](cached.WithPrefix[string, []byte]("zstd", storage)), direct),
					// "redis+proto":             cached.Wrap(cached.WithPrefix[string, []byte]("proto", storage), direct, cached.AsProtoBuf[*debian.RepositoryConfig, *hedge.DebianPackages]()),
					// "redis+proto+gz":          cached.Wrap(cached.WithGzip[string, []byte](cached.WithPrefix[string, []byte]("proto_gz", storage)), direct, cached.AsProtoBuf[*debian.RepositoryConfig, *hedge.DebianPackages]()),
					"redis+proto+zstd": cached.Wrap(cached.WithZstd[string, []byte](cached.WithPrefix[string, []byte]("proto_zstdz", storage)), direct, cached.AsProtoBuf[*debian.RepositoryConfig, *hedge.DebianPackages]()),
					// "redis+signed+proto+zstd": cached.Wrap(cached.WithZstd[string, []byte](cached.WithPrefix[string, []byte]("signed_proto_zstd", signed)), direct, cached.AsProtoBuf[*debian.RepositoryConfig, *hedge.DebianPackages]()),
				})

				for _, repo := range ecoCfg.Repositories {
					repoLog := ecoLog.WithValues("repository", repo.Name()).V(1)
					repoLog.Info("loading packages...")
					allPackages, err := debStorage(ctx, repo.(*debian.RepositoryConfig))
					if err != nil {
						return err
					}
					repoLog.Info("loaded packages repository", "package_count", len(allPackages.Packages))

					ctx, filterSpan := tracer.Start(ctx, "FilterPackages")
					pred, err := filter.CueConfigToPredicate[*hedge.DebianPackage](policyDir, repo.FilterConfig())
					if err != nil {
						filterSpan.End()
						return err
					}
					var filtered []*hedge.DebianPackage
					for _, pkg := range allPackages.GetPackages() {
						ok, err := pred(ctx, pkg)
						if err != nil {
							filterSpan.End()
							return err
						}
						if ok {
							repoLog.V(1).Info("accepted package", "package_name", pkg.GetPackage())
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
