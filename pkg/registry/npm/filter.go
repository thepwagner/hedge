package npm

import (
	"context"

	"github.com/thepwagner/hedge/pkg/filter"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/mod/semver"
)

type PackageVersion struct {
	Package *Package `json:"pkg"`
	Version *Version `json:"version"`
}

type PackageFilter struct {
	tracer trace.Tracer
	loader PackageLoader

	filter filter.Predicate[PackageVersion]
}

var _ PackageLoader = (*PackageFilter)(nil)

func NewPackageFilter(tracer trace.Tracer, wrapped PackageLoader, filter filter.Predicate[PackageVersion]) *PackageFilter {
	return &PackageFilter{
		tracer: tracer,
		loader: wrapped,
		filter: filter,
	}
}

func (f *PackageFilter) GetPackage(ctx context.Context, pkgName string) (*Package, error) {
	ctx, span := f.tracer.Start(ctx, "npmfilter.GetPackage")
	defer span.End()
	span.SetAttributes(attribute.String("npm.package", pkgName))

	pkg, err := f.loader.GetPackage(ctx, pkgName)
	if err != nil {
		return nil, err
	}

	allowedVersions := make(map[string]Version, len(pkg.Versions))
	for version, versionData := range pkg.Versions {
		allowed, err := f.filter(ctx, PackageVersion{
			Package: pkg,
			Version: &versionData,
		})
		if err != nil {
			return nil, err
		}
		if allowed {
			allowedVersions[version] = versionData
		}
	}
	if len(allowedVersions) == 0 {
		return nil, nil
	}

	// Delete any dates of filtered versions:
	filteredTimes := make(map[string]string, len(allowedVersions))
	for version, versionTime := range pkg.Times {
		switch version {
		case "created", "modified":
			filteredTimes[version] = versionTime
		default:
			if _, ok := allowedVersions[version]; ok {
				filteredTimes[version] = versionTime
			}
		}
	}
	pkg.Times = filteredTimes

	// Update any missing references to the latest version
	var versions []string
	for version := range allowedVersions {
		versions = append(versions, version)
	}
	semver.Sort(versions)
	latestVersion := versions[len(versions)-1]

	for dist, tag := range pkg.DistTags {
		switch dist {
		case "latest", "beta":
		default:
			continue
		}

		if _, ok := allowedVersions[tag]; !ok {
			pkg.DistTags[dist] = latestVersion
			pkg.Times[dist] = pkg.Times[latestVersion]
		}
	}

	pkg.Versions = allowedVersions
	return pkg, nil
}
