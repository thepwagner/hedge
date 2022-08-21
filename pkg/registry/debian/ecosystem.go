package debian

import (
	"context"
	"fmt"
	"net/http"

	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/registry"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

const Ecosystem registry.Ecosystem = "debian"

type EcosystemProvider struct {
	tracer  trace.Tracer
	client  *http.Client
	storage cached.ByteStorage
}

func NewEcosystemProvider(tracer trace.Tracer, client *http.Client, storage cached.ByteStorage) *EcosystemProvider {
	return &EcosystemProvider{
		tracer:  tracer,
		client:  client,
		storage: storage,
	}
}

var _ registry.EcosystemProvider = (*EcosystemProvider)(nil)

func (e EcosystemProvider) Ecosystem() registry.Ecosystem { return Ecosystem }
func (e EcosystemProvider) BlankRepositoryConfig() registry.RepositoryConfig {
	return &RepositoryConfig{}
}

func (e EcosystemProvider) AllPackages(ctx context.Context, repoCfg registry.RepositoryConfig) ([]registry.Package, error) {
	cfg, ok := repoCfg.(*RepositoryConfig)
	if !ok {
		return nil, fmt.Errorf("invalid repository config: %T", repoCfg)
	}

	var releases ReleaseLoader
	var packages PackagesLoader
	var err error
	if upCfg := cfg.Source.Upstream; upCfg != nil {
		releases, packages, err = NewRemoteLoader(e.tracer, e.client, e.storage, *cfg.Source.Upstream)
		if err != nil {
			return nil, err
		}
	} else if ghCfg := cfg.Source.GitHub; ghCfg != nil {
		return nil, nil
		// packages = NewGitHubPackagesLoader(args.Tracer, args.Client, *cfg.Source.GitHub)
	} else {
		return nil, fmt.Errorf("no source specified")
	}
	if err != nil {
		return nil, err
	}

	r, err := releases.Load(ctx)
	if err != nil {
		return nil, err
	}

	eg, ctx := errgroup.WithContext(ctx)
	res := make(chan registry.Package)

	for _, arch := range r.Architectures() {
		arch := arch
		eg.Go(func() error {
			pkgs, err := packages.LoadPackages(ctx, r, arch)
			if err != nil {
				return err
			}
			for _, pkg := range pkgs {
				res <- pkg
			}
			return nil
		})
	}
	go func() {
		_ = eg.Wait()
		close(res)
	}()

	var allPackages []registry.Package
	for pkg := range res {
		allPackages = append(allPackages, pkg)
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return allPackages, nil
}

func (e EcosystemProvider) NewHandler(args registry.HandlerArgs) (registry.HasRoutes, error) {
	// h, err := base.NewHandler(args, func(cfg *RepositoryConfig) (*repositoryHandler, error) {
	// 	return newRepositoryHandler(args, cfg)
	// })
	// if err != nil {
	// 	return nil, err
	// }
	// return &Handler{Handler: *h}, nil
	return nil, nil
}
