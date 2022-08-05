package debian

import (
	"context"
	"sync"

	"github.com/thepwagner/hedge/pkg/filter"
	"go.opentelemetry.io/otel/trace"
)

type ReleaseLoader interface {
	Load(context.Context) (*Release, error)
}

type PackagesLoader interface {
	BaseURL() string
	LoadPackages(ctx context.Context, arch Architecture) ([]Package, error)
}

type FilteredPackageLoader struct {
	tracer  trace.Tracer
	wrapped PackagesLoader
	pred    filter.Predicate[Package]
}

func NewFilteredPackageLoader(tracer trace.Tracer, wrapped PackagesLoader, pred filter.Predicate[Package]) *FilteredPackageLoader {
	return &FilteredPackageLoader{
		tracer:  tracer,
		wrapped: wrapped,
		pred:    pred,
	}
}

func (p FilteredPackageLoader) BaseURL() string { return p.wrapped.BaseURL() }
func (p FilteredPackageLoader) LoadPackages(ctx context.Context, arch Architecture) ([]Package, error) {
	ctx, span := p.tracer.Start(ctx, "debianfilter.LoadPackages")
	defer span.End()
	span.SetAttributes(attrArchitecture.String(string(arch)))

	pkgs, err := p.wrapped.LoadPackages(ctx, arch)
	if err != nil {
		return nil, err
	}
	return filter.FilterSlice(ctx, p.pred, pkgs...)
}

// In-memory caching of values, for early dev
// TODO: replace with external cache (redis?)

type freezingPackagesLoader struct {
	wrapped PackagesLoader

	mu       sync.Mutex
	packages map[Architecture][]Package
}

func freezePackagesLoader(wrapped PackagesLoader) *freezingPackagesLoader {
	return &freezingPackagesLoader{
		wrapped:  wrapped,
		packages: map[Architecture][]Package{},
	}
}

func (f *freezingPackagesLoader) BaseURL() string { return f.wrapped.BaseURL() }

func (f *freezingPackagesLoader) LoadPackages(ctx context.Context, arch Architecture) ([]Package, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if pkgs, ok := f.packages[arch]; ok {
		return pkgs, nil
	}
	pkgs, err := f.wrapped.LoadPackages(ctx, arch)
	if err != nil {
		return nil, err
	}
	f.packages[arch] = pkgs
	return pkgs, nil
}

type freezingReleaseLoader struct {
	wrapped ReleaseLoader

	mu      sync.Mutex
	release *Release
}

func freezeReleaseLoader(wrapped ReleaseLoader) *freezingReleaseLoader {
	return &freezingReleaseLoader{
		wrapped: wrapped,
	}
}

func (f *freezingReleaseLoader) Load(ctx context.Context) (*Release, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.release != nil {
		return f.release, nil
	}

	release, err := f.wrapped.Load(ctx)
	if err != nil {
		return nil, err
	}
	f.release = release
	return release, nil
}
