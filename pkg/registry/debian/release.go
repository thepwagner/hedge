package debian

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/thepwagner/hedge/proto/hedge/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Architecture is a machine architecture, like `amd64` or `arm64`.
// Reference: https://www.debian.org/doc/debian-policy/ch-customized-programs.html#s-arch-spec
type Architecture string

// Component is a Debian component, like `main` or `contrib`.
type Component string

func ParagraphFromRelease(r *hedge.DebianRelease) (Paragraph, error) {
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

func ReleaseFromParagraph(graph Paragraph) (*hedge.DebianRelease, error) {
	ret := hedge.DebianRelease{
		AcquireByHash: graph["Acquire-By-Hash"] == "yes",
		Architectures: strings.Split(graph["Architectures"], " "),
	}

	for k, v := range graph {
		switch k {
		case "Acquire-By-Hash", "Architectures":
			continue
		case "Changelogs":
			ret.Changelogs = v
		case "Codename":
			ret.Codename = v
		case "Components":
			ret.Components = strings.Split(v, " ")
		case "Date":
			t, err := time.Parse(time.RFC1123, v)
			if err != nil {
				return nil, fmt.Errorf("parsing date: %w", err)
			}
			ret.Date = timestamppb.New(t)
		case "Description":
			ret.Description = v
		case "Label":
			ret.Label = v
		case "MD5Sum", "SHA256":
			// skipped, as these are calculated below
		case "No-Support-for-Architecture-all":
			ret.NoSupportForArchitectureAll = v == "yes"
		case "Origin":
			ret.Origin = v
		case "Suite":
			ret.Suite = v
		case "Version":
			ret.Version = v
		default:
			return nil, fmt.Errorf("unknown key: %s", k)
		}
	}

	digests, err := parseDigests(graph)
	if err != nil {
		return nil, err
	}
	ret.Digests = digests

	return &ret, nil
}

var digestRE = regexp.MustCompile(`([0-9a-f]{32,64})\s+([0-9]+)\s+([^ ]+)$`)

func parseDigests(graph Paragraph) (map[string]*hedge.DebianRelease_DigestedFile, error) {
	lines := strings.Split(graph["SHA256"], "\n")
	digests := make(map[string]*hedge.DebianRelease_DigestedFile, len(lines))
	for _, line := range lines {
		m := digestRE.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}
		path := m[3]
		size, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, fmt.Errorf("parsing expected size: %w", err)
		}
		digest, err := hex.DecodeString(m[1])
		if err != nil {
			return nil, fmt.Errorf("parsing expected sha: %w", err)
		}
		digests[path] = &hedge.DebianRelease_DigestedFile{
			Path:      fmt.Sprintf("%s/by-hash/SHA256/%x", filepath.Dir(path), digest),
			Sha256Sum: digest,
			Size:      uint64(size),
		}
	}

	for _, line := range strings.Split(graph["MD5Sum"], "\n") {
		m := digestRE.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}
		path := m[3]
		df, ok := digests[path]
		if !ok {
			continue
		}
		digest, err := hex.DecodeString(m[1])
		if err != nil {
			return nil, fmt.Errorf("parsing expected md5: %w", err)
		}
		df.Md5Sum = digest
	}

	return digests, nil
}

func WriteReleaseFile(ctx context.Context, r *hedge.DebianRelease, pkgs map[Architecture][]Package, w io.Writer) error {
	// Conver the basic release to a Paragraph:
	graph, err := ParagraphFromRelease(r)
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
