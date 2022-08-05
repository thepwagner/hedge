package debian

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/mitchellh/mapstructure"
)

// Architecture is a machine architecture, like `amd64` or `arm64`.
// Reference: https://www.debian.org/doc/debian-policy/ch-customized-programs.html#s-arch-spec
type Architecture string

// Component is a Debian component, like `main` or `contrib`.
type Component string

// Release is metadata about a Debian version.
type Release struct {
	ComponentsRaw    string `mapstructure:"Components" yaml:"components"`
	ArchitecturesRaw string `mapstructure:"Architectures" yaml:"architectures"`
	DateRaw          string `mapstructure:"Date"`
	Description      string
	Origin           string `yaml:"origin"`
	Label            string
	Version          string
	Codename         string `yaml:"codename"`
	Suite            string `yaml:"suite"`
	Changelogs       string
	SHA256           string
}

func (r Release) Date() time.Time {
	t, _ := time.Parse(time.RFC1123, r.DateRaw)
	return t
}

func (r Release) Architectures() []Architecture {
	split := strings.Split(r.ArchitecturesRaw, " ")
	ret := make([]Architecture, 0, len(split))
	for _, s := range split {
		ret = append(ret, Architecture(s))
	}
	return ret
}

func (r Release) Components() []Component {
	split := strings.Split(r.ComponentsRaw, " ")
	ret := make([]Component, 0, len(split))
	for _, s := range split {
		ret = append(ret, Component(s))
	}
	return ret
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

func ReleaseFromParagraph(graph Paragraph) (*Release, error) {
	var r Release
	if err := mapstructure.Decode(graph, &r); err != nil {
		return nil, fmt.Errorf("parsing release: %w", err)
	}
	return &r, nil
}

func ParseReleaseFile(data []byte, key openpgp.EntityList) (Paragraph, error) {
	// Verify signature:
	block, _ := clearsign.Decode(data)
	_, err := openpgp.CheckDetachedSignature(key, bytes.NewReader(block.Bytes), block.ArmoredSignature.Body, nil)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	// Parse file:
	graphs, err := ParseControlFile(bytes.NewReader(block.Plaintext))
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}
	if len(graphs) != 1 {
		return nil, fmt.Errorf("no paragraphs found")
	}
	return graphs[0], nil
}

func WriteReleaseFile(ctx context.Context, r Release, pkgs map[Architecture][]Package, w io.Writer) error {
	// Conver the basic release to a Paragraph:
	graph, err := r.Paragraph()
	if err != nil {
		return fmt.Errorf("creating paragraph: %w", err)
	}

	// Digest and render all Packages files:
	var pkgDigests []PackagesDigest
	for arch, packages := range pkgs {
		digests, err := PackageHashes(ctx, arch, "main", packages...)
		if err != nil {
			return fmt.Errorf("calculating package hashes: %w", err)
		}
		pkgDigests = append(pkgDigests, digests...)
	}
	sort.Slice(pkgDigests, func(i, j int) bool {
		return pkgDigests[i].Path < pkgDigests[j].Path
	})
	shas := make([]string, 0, len(pkgDigests))
	md5s := make([]string, 0, len(pkgDigests))
	for _, d := range pkgDigests {
		shas = append(shas, fmt.Sprintf(" %x %d %s", d.Sha256, d.Size, d.Path))
		md5s = append(md5s, fmt.Sprintf(" %x %d %s", d.Md5, d.Size, d.Path))
	}
	graph["SHA256"] = strings.Join(shas, "\n")
	graph["MD5Sum"] = strings.Join(md5s, "\n")

	if err := WriteControlFile(w, graph); err != nil {
		return err
	}
	return nil
}

type PackagesDigest struct {
	Path   string
	Size   int
	Sha256 []byte
	Md5    []byte
}

func PackageHashes(ctx context.Context, arch Architecture, component Component, packages ...Package) ([]PackagesDigest, error) {
	var buf strings.Builder
	if err := WriteControlFile(&buf, packages...); err != nil {
		return nil, err
	}
	pkgFile := buf.String()

	// XZ compression is supported, but slooooow.
	// Only use XZ if we are rendering the repository to the filesystem for static hosting.
	var digests []PackagesDigest
	for _, compression := range []Compression{CompressionNone, CompressionGZIP} {
		var buf bytes.Buffer
		if err := compression.Compress(&buf, strings.NewReader(pkgFile)); err != nil {
			return nil, err
		}
		sha := sha256.Sum256(buf.Bytes())

		// TODO: should we use MD5? It's nice to have some resistance to SHA-256 attacks... but it is MD5 🤡
		md := md5.Sum(buf.Bytes())
		digests = append(digests, PackagesDigest{
			Path:   fmt.Sprintf("%s/binary-%s/Packages%s", component, arch, compression.Extension()),
			Size:   buf.Len(),
			Sha256: sha[:],
			Md5:    md[:],
		})
	}
	return digests, nil
}
