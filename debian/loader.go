package debian

import (
	"context"
	"sync"

	"github.com/thepwagner/hedge/pkg/filter"
)

type ReleaseLoader interface {
	Load(context.Context) (*Release, error)
}

type PackagesLoader interface {
	BaseURL() string
	LoadPackages(ctx context.Context, arch Architecture) ([]Package, error)
}

type FilteredPackageLoader struct {
	wrapped PackagesLoader
	pred    filter.Predicate[Package]
}

func NewFilteredPackageLoader(wrapped PackagesLoader, pred filter.Predicate[Package]) *FilteredPackageLoader {
	return &FilteredPackageLoader{
		wrapped: wrapped,
		pred:    pred,
	}
}

func (p FilteredPackageLoader) BaseURL() string { return p.wrapped.BaseURL() }
func (p FilteredPackageLoader) LoadPackages(ctx context.Context, arch Architecture) ([]Package, error) {
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
