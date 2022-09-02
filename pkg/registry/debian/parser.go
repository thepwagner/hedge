package debian

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/blakesmith/ar"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/proto/hedge/v1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Parser extracts hedge models from Debian control files
type Parser struct {
	tracer trace.Tracer
	now    func() time.Time
}

func NewParser(tracer trace.Tracer) Parser {
	return Parser{tracer: tracer, now: time.Now}
}

func (p Parser) Packages(ctx context.Context, in io.Reader) ([]*hedge.DebianPackage, error) {
	ctx, span := p.tracer.Start(ctx, "debian.Parser.Packages")
	defer span.End()

	graphs, err := p.parseControlFile(ctx, in)
	if err != nil {
		return nil, observability.CaptureError(span, err)
	}

	pkgs, err := p.packagesFromParagraphs(ctx, graphs)
	if err != nil {
		return nil, observability.CaptureError(span, err)
	}
	span.SetAttributes(attrPackageCount(len(pkgs)))
	return pkgs, nil
}

func (p Parser) parseControlFile(ctx context.Context, in io.Reader) ([]Paragraph, error) {
	_, span := p.tracer.Start(ctx, "debian.parser.parseControlFile")
	defer span.End()

	graphs, err := ParseControlFile(in)
	if err != nil {
		return nil, observability.CaptureError(span, err)
	}
	span.SetAttributes(attribute.Int("debian.paragraph.count", len(graphs)))
	return graphs, nil
}

func (p Parser) packagesFromParagraphs(ctx context.Context, graphs []Paragraph) ([]*hedge.DebianPackage, error) {
	_, span := p.tracer.Start(ctx, "debian.Parser.packagesFromParagraphs")
	defer span.End()

	pkgs := make([]*hedge.DebianPackage, 0, len(graphs))
	for _, graph := range graphs {
		pkg, err := PackageFromParagraph(graph)
		if err != nil {
			return nil, observability.CaptureError(span, fmt.Errorf("parsing package: %w", err))
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

func (p Parser) Release(ctx context.Context, in io.Reader, key openpgp.EntityList) (*hedge.DebianRelease, error) {
	_, span := p.tracer.Start(ctx, "debian.Parser.Release")
	defer span.End()

	// Verify the payload matches the signature for the provided key:
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, observability.CaptureError(span, err)
	}
	block, _ := clearsign.Decode(data)
	if _, err := openpgp.CheckDetachedSignature(key, bytes.NewReader(block.Bytes), block.ArmoredSignature.Body, &packet.Config{Time: p.now}); err != nil {
		return nil, observability.CaptureError(span, fmt.Errorf("signature verification failed: %w", err))
	}

	// Parse file into a single release:
	graphs, err := ParseControlFile(bytes.NewReader(block.Plaintext))
	if err != nil {
		return nil, observability.CaptureError(span, fmt.Errorf("parsing paragraphs from signed text: %w", err))
	}
	if len(graphs) != 1 {
		return nil, observability.CaptureError(span, fmt.Errorf("no paragraphs found"))
	}
	release, err := ReleaseFromParagraph(graphs[0])
	if err != nil {
		return nil, observability.CaptureError(span, fmt.Errorf("parsing release from paragraph: %w", err))
	}
	return release, nil
}

func (p Parser) PackageFromDeb(ctx context.Context, in io.Reader) (*hedge.DebianPackage, error) {
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

			pkgs, err := p.Packages(ctx, tarR)
			if err != nil {
				return nil, fmt.Errorf("parsing control file: %w", err)
			}
			if len(pkgs) == 1 {
				return pkgs[0], nil
			}
		}
	}
	return nil, nil
}
