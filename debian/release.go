package debian

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/mitchellh/mapstructure"
)

type Release struct {
	ComponentsRaw    string `mapstructure:"Components"`
	ArchitecturesRaw string `mapstructure:"Architectures"`
	DateRaw          string `mapstructure:"Date"`
	Description      string
	Origin           string
	Label            string
	Version          string
	Codename         string
	Suite            string
	Changelogs       string
}

func (r Release) Date() time.Time {
	t, _ := time.Parse(time.RFC1123, r.DateRaw)
	return t
}

func (r Release) Architectures() []string {
	return strings.Split(r.ArchitecturesRaw, " ")
}

func (r Release) Components() []string {
	return strings.Split(r.ComponentsRaw, " ")
}

func (r Release) Paragraph() (Paragraph, error) {
	graph := Paragraph{}
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &graph,
	})
	if err != nil {
		return nil, err
	}
	if err := dec.Decode(r); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}
	return graph, nil
}

type ParseReleaseOptions struct {
	SigningKey    openpgp.EntityList
	Components    []string
	Architectures []string
}

func ParseReleaseFile(data []byte, opts ParseReleaseOptions) (*Release, error) {
	// Verify signature:
	block, _ := clearsign.Decode(data)
	_, err := openpgp.CheckDetachedSignature(opts.SigningKey, bytes.NewReader(block.Bytes), block.ArmoredSignature.Body, nil)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	// Parse file:
	graphs, err := ParseControlFile(bytes.NewReader(block.Plaintext))
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}
	if len(graphs) == 0 {
		return nil, fmt.Errorf("no paragraphs found")
	}
	return ReleaseFromParagraph(graphs[0])
}

func ReleaseFromParagraph(graph Paragraph) (*Release, error) {
	var r Release
	if err := mapstructure.Decode(graph, &r); err != nil {
		return nil, fmt.Errorf("parsing release: %w", err)
	}
	return &r, nil
}

func WriteReleaseFile(r Release, w io.Writer) error {
	graph, err := r.Paragraph()
	if err != nil {
		return fmt.Errorf("creating paragraph: %w", err)
	}

	pkgDigests, err := PackageHashes("amd64", hackPackages...)
	if err != nil {
		return fmt.Errorf("calculating package hashes: %w", err)
	}
	digests := make([]string, 0, len(pkgDigests))
	for _, d := range pkgDigests {
		digests = append(digests, fmt.Sprintf(" %x %d %s", d.Digest, d.Size, d.Path))
	}
	graph["SHA256"] = strings.Join(digests, "\n")

	if err := WriteControlFile(w, graph); err != nil {
		return err
	}
	return nil
}

type PackagesDigest struct {
	Path   string
	Size   int
	Digest []byte
}

func PackageHashes(arch string, packages ...Package) ([]PackagesDigest, error) {
	var buf bytes.Buffer
	if err := WritePackages(&buf, packages...); err != nil {
		return nil, err
	}
	b := buf.Bytes()

	var digests []PackagesDigest
	for _, compression := range []Compression{CompressionNone, CompressionXZ} {
		var buf bytes.Buffer
		if err := compression.Compress(&buf, bytes.NewReader(b)); err != nil {
			return nil, err
		}
		size := buf.Len()
		sha := sha256.Sum256(buf.Bytes())
		digests = append(digests, PackagesDigest{
			Path:   fmt.Sprintf("main/binary-%s/Packages%s", arch, compression.Extension()),
			Size:   size,
			Digest: sha[:],
		})
	}
	return digests, nil
}
