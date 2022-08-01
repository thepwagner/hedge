package npm

import (
	"context"
	"regexp"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type PackageFilter struct {
	tracer trace.Tracer
	loader PackageLoader

	packageRules       []func(*Package) bool
	versionRules       map[string][]func(*Version) bool
	globalVersionRules []func(*Version) bool
}

var _ PackageLoader = (*PackageFilter)(nil)

func NewPackageFilter(tracer trace.Tracer, wrapped PackageLoader, rules ...FilterRule) (*PackageFilter, error) {
	f := PackageFilter{
		tracer: tracer,
		loader: wrapped,
	}
	for _, rule := range rules {
		if rule.Pattern != "" {
			pattern, err := regexp.Compile(rule.Pattern)
			if err != nil {
				return nil, err
			}
			f.packageRules = append(f.packageRules, func(pkg *Package) bool { return pattern.MatchString(pkg.Name) })
		}

		if rule.Deprecated != nil {
			f.globalVersionRules = append(f.globalVersionRules, func(v *Version) bool { return *rule.Deprecated == v.Deprecated() })
		}
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

	var allowedPkg bool
	for _, pkgRule := range f.packageRules {
		if pkgRule(pkg) {
			allowedPkg = true
			break
		}
	}
	if !allowedPkg {
		return nil, nil
	}

	versionRules := append(f.globalVersionRules, f.versionRules[pkgName]...)
	if len(versionRules) > 0 {
		allowedVersions := make(map[string]Version, len(pkg.Versions))
		for _, versionRule := range versionRules {
			for key, version := range pkg.Versions {
				if _, ok := allowedVersions[key]; ok {
					continue
				}
				if versionRule(&version) {
					allowedVersions[key] = version
				}
			}
		}
		if len(allowedVersions) == 0 {
			return nil, nil
		}
		pkg.Versions = allowedVersions
	}

	return pkg, nil
}
