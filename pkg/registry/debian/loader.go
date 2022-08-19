package debian

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/thepwagner/hedge/pkg/filter"
	"github.com/thepwagner/hedge/pkg/signature"
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
	rekor   signature.RekorFinder
}

func NewFilteredPackageLoader(tracer trace.Tracer, wrapped PackagesLoader, rekor signature.RekorFinder, pred filter.Predicate[Package]) *FilteredPackageLoader {
	return &FilteredPackageLoader{
		tracer:  tracer,
		wrapped: wrapped,
		rekor:   rekor,
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

	decorated := make([]Package, 0, len(pkgs))
	for _, pkg := range pkgs {
		digest, err := hex.DecodeString(pkg.Sha256)
		if err != nil {
			return nil, err
		}
		if signer, err := p.rekor.GetSignature(ctx, digest); err != nil {
			return nil, err
		} else if signer != nil {
			s, _ := json.Marshal(signer)
			pkg.RekorRaw = string(s)
		}

		decorated = append(decorated, pkg)
	}

	return filter.FilterSlice(ctx, p.pred, decorated...)
}
