package npm

import (
	"context"

	"github.com/thepwagner/hedge/pkg/filter"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type PackageFilter struct {
	tracer trace.Tracer
	loader PackageLoader

	globalPackageFilter filter.Predicate[Package]
	globalVersionFilter filter.Predicate[Version]

	versionFiltersByPkg map[string][]filter.Predicate[Version]
}

var _ PackageLoader = (*PackageFilter)(nil)

func NewPackageFilter(tracer trace.Tracer, wrapped PackageLoader, rules ...FilterRule) (*PackageFilter, error) {
	var globalPackagePreds []filter.Predicate[Package]
	var globalVersionPreds []filter.Predicate[Version]
	for _, rule := range rules {
		if rule.Pattern != "" {
			predicate, err := filter.MatchesPattern[Package](rule.Pattern)
			if err != nil {
				return nil, err
			}
			globalPackagePreds = append(globalPackagePreds, predicate)
		}

		if rule.Deprecated != nil {
			globalVersionPreds = append(globalVersionPreds, filter.MatchesDeprecated[Version](*rule.Deprecated))
		}
	}

	f := PackageFilter{
		tracer:              tracer,
		loader:              wrapped,
		globalPackageFilter: filter.AnyOf(globalPackagePreds...),
		globalVersionFilter: filter.AnyOf(globalVersionPreds...),
	}
	return &f, nil
}

func (f *PackageFilter) GetPackage(ctx context.Context, pkgName string) (*Package, error) {
	ctx, span := f.tracer.Start(ctx, "npm-filter.GetPackage")
	defer span.End()
	span.SetAttributes(attribute.String("npm.package", pkgName))

	pkg, err := f.loader.GetPackage(ctx, pkgName)
	if err != nil {
		return nil, err
	}

	allowed, err := f.globalPackageFilter(ctx, *pkg)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, nil
	}

	allowedVersions, err := filter.FilterMap(ctx, filter.AnyOf(append(f.versionFiltersByPkg[pkgName], f.globalVersionFilter)...), pkg.Versions)
	if err != nil {
		return nil, err
	}
	if len(allowedVersions) == 0 {
		return nil, nil
	}
	pkg.Versions = allowedVersions

	return pkg, nil
}
