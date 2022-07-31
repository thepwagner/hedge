package debian

import (
	"bytes"
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

type Architecture string
type Component string

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

func ReleaseFromParagraph(graph Paragraph) (*Release, error) {
	var r Release
	if err := mapstructure.Decode(graph, &r); err != nil {
		return nil, fmt.Errorf("parsing release: %w", err)
	}
	return &r, nil
}

func WriteReleaseFile(r Release, pkgs map[Component]map[Architecture][]Package, w io.Writer) error {
	graph, err := r.Paragraph()
	if err != nil {
		return fmt.Errorf("creating paragraph: %w", err)
	}

	var pkgDigests []PackagesDigest
	for component, archPkgs := range pkgs {
		for arch, packages := range archPkgs {
			archDigests, err := PackageHashes(arch, component, packages...)
			if err != nil {
				return fmt.Errorf("calculating package hashes: %w", err)
			}
			pkgDigests = append(pkgDigests, archDigests...)
		}

	}
	sort.Slice(pkgDigests, func(i, j int) bool {
		return pkgDigests[i].Path < pkgDigests[j].Path
	})

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

func PackageHashes(arch Architecture, component Component, packages ...Package) ([]PackagesDigest, error) {
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
			Path:   fmt.Sprintf("%s/binary-%s/Packages%s", component, arch, compression.Extension()),
			Size:   size,
			Digest: sha[:],
		})
	}
	return digests, nil
}
