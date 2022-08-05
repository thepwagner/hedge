package debian

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/blakesmith/ar"
	"go.opentelemetry.io/otel/trace"
)

type PackageParser struct {
	tracer trace.Tracer
}

func NewPackageParser(tracer trace.Tracer) PackageParser {
	return PackageParser{tracer: tracer}
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

func (p PackageParser) PackageFromDeb(ctx context.Context, in io.Reader) (*Package, error) {
	for reader := ar.NewReader(in); ; {
		hdr, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("reading archive: %w", err)
		}
		if hdr.Name != "control.tar.gz" {
			continue
		}

		gzR, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("creating gzip reader: %w", err)
		}
		defer gzR.Close()

		for tarR := tar.NewReader(gzR); ; {
			hdr, err := tarR.Next()
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return nil, fmt.Errorf("reading archive: %w", err)
			}

			if hdr.Name != "./control" {
				continue
			}

			pkgs, err := p.ParsePackages(ctx, tarR)
			if err != nil {
				return nil, fmt.Errorf("parsing control file: %w", err)
			}
			if len(pkgs) == 1 {
				return &pkgs[0], nil
			}
		}
	}
	return nil, nil
}
