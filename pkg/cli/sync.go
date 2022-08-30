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
	"golang.org/x/sync/errgroup"
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
			storage = cached.InRedis(cfg.RedisAddr, tp)

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
							Name:          deb.Package,
							Source:        deb.Source,
							Version:       deb.Version,
							Priority:      deb.Priority,
							InstalledSize: uint64(deb.InstalledSize()),
							Maintainer:    deb.Maintainer,
							Tags:          deb.Tags(),
						})
					}
					return &hedge.DebianPackages{
						Packages: debs,
					}, nil
				}

				debStorage := cached.Wrap(cached.WithZstd[string, []byte](cached.WithPrefix[string, []byte]("repo_deb_packages", storage)), direct, cached.AsProtoBuf[*debian.RepositoryConfig, *hedge.DebianPackages]())

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
					filtered := make(chan *hedge.DebianPackage)
					eg, egCtx := errgroup.WithContext(ctx)
					for _, pkg := range allPackages.GetPackages() {
						pkg := pkg
						eg.Go(func() error {
							ok, err := pred(egCtx, pkg)
							if err != nil {
								return err
							}
							if ok {
								repoLog.V(1).Info("accepted package", "package_name", pkg.GetName())
								filtered <- pkg
							}
							return nil
						})
					}
					go func() {
						_ = eg.Wait()
						close(filtered)
					}()

					var ret []*hedge.DebianPackage
					for pkg := range filtered {
						ret = append(ret, pkg)
					}
					if err := eg.Wait(); err != nil {
						return err
					}
					filterSpan.End()
					repoLog.Info("filtered packages repository", "package_count", len(ret))
					filterSpan.End()
				}
			}
			return nil
		},
	}
}
