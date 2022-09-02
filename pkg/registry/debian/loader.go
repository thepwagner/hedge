package debian

import (
	"context"

	"github.com/thepwagner/hedge/proto/hedge/v1"
)

type LoadReleaseArgs struct {
	MirrorURL  string
	SigningKey string
	Dist       string
}

type ReleaseLoader interface {
	LoadRelease(ctx context.Context, args LoadReleaseArgs) (*hedge.DebianRelease, error)
}

type LoadPackagesArgs struct {
	Release      *hedge.DebianRelease
	Architecture Architecture
}

type PackagesLoader interface {
	LoadPackages(ctx context.Context, args LoadPackagesArgs) (*hedge.DebianPackages, error)
}

// type FilteredPackageLoader struct {
// 	tracer  trace.Tracer
// 	wrapped PackagesLoader
// 	pred    filter.Predicate[Package]
// 	rekor   signature.RekorFinder
// }

// func NewFilteredPackageLoader(tracer trace.Tracer, wrapped PackagesLoader, rekor signature.RekorFinder, pred filter.Predicate[Package]) *FilteredPackageLoader {
// 	return &FilteredPackageLoader{
// 		tracer:  tracer,
// 		wrapped: wrapped,
// 		rekor:   rekor,
// 		pred:    pred,
// 	}
// }

// // func (p FilteredPackageLoader) BaseURL() string { return p.wrapped.BaseURL() }
// func (p FilteredPackageLoader) LoadPackages(ctx context.Context, r *Release, arch Architecture) ([]Package, error) {
// 	ctx, span := p.tracer.Start(ctx, "debianfilter.LoadPackages", trace.WithAttributes(attrArchitecture(arch)))
// 	defer span.End()

// 	pkgs, err := p.wrapped.LoadPackages(ctx, r, arch)
// 	if err != nil {
// 		return nil, err
// 	}

// 	decorated := make([]Package, 0, len(pkgs))
// 	for _, pkg := range pkgs {
// 		digest, err := hex.DecodeString(pkg.Sha256)
// 		if err != nil {
// 			return nil, err
// 		}
// 		if signer, err := p.rekor.GetSignature(ctx, digest); err != nil {
// 			return nil, err
// 		} else if signer != nil {
// 			s, _ := json.Marshal(signer)
// 			pkg.RekorRaw = string(s)
// 		}

// 		decorated = append(decorated, pkg)
// 	}

// 	return filter.FilterSlice(ctx, p.pred, decorated...)
// }
