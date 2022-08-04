package debian

import (
	"context"
	"fmt"
	"io"

	"go.opentelemetry.io/otel/trace"
)

type PackageParser struct {
	tracer trace.Tracer
}

func (p PackageParser) ParsePackages(ctx context.Context, in io.Reader) ([]Package, error) {
	ctx, span := p.tracer.Start(ctx, "debianparser.ParsePackages")
	defer span.End()

	_, parseSpan := p.tracer.Start(ctx, "debianparser.ParseControlFile")
	graphs, err := ParseControlFile(in)
	if err != nil {
		span.RecordError(err)
		parseSpan.RecordError(err)
		parseSpan.End()
		return nil, err
	}
	parseSpan.SetAttributes(attrPackageCount.Int(len(graphs)))
	parseSpan.End()

	_, mapSpan := p.tracer.Start(ctx, "debianparser.PackageFromParagraph")
	defer mapSpan.End()
	pkgs := make([]Package, 0, len(graphs))
	for _, graph := range graphs {
		pkg, err := PackageFromParagraph(graph)
		if err != nil {
			span.RecordError(err)
			mapSpan.RecordError(err)
			return nil, fmt.Errorf("parsing package: %w", err)
		}
		pkgs = append(pkgs, pkg)
	}
	mapSpan.SetAttributes(attrPackageCount.Int(len(pkgs)))
	span.SetAttributes(attrPackageCount.Int(len(pkgs)))
	return pkgs, nil
}
