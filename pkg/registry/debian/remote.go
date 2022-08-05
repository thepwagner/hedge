package debian

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"go.opentelemetry.io/otel/trace"
)

type remoteLoader struct {
	tracer trace.Tracer
	client *http.Client

	baseURL    string
	dist       string
	components []string
}

type RemoteReleaseLoader struct {
	remoteLoader
	keyring       openpgp.EntityList
	architectures []string
}

type RemotePackagesLoader struct {
	remoteLoader
	releases ReleaseLoader
	parser   PackageParser
}

func NewRemoteLoader(tracer trace.Tracer, client *http.Client, cfg UpstreamConfig) (*RemoteReleaseLoader, *RemotePackagesLoader, error) {
	if cfg.Release == "" {
		return nil, nil, fmt.Errorf("missing release")
	}

	if cfg.Key == "" {
		return nil, nil, fmt.Errorf("missing keyfile")
	}
	kr, err := openpgp.ReadArmoredKeyRing(strings.NewReader(cfg.Key))
	if err != nil {
		return nil, nil, err
	}

	baseURL := cfg.URL
	if baseURL == "" {
		baseURL = "https://deb.debian.org/debian"
	}
	architectures := cfg.Architectures
	if len(architectures) == 0 {
		architectures = []string{"all", "amd64"}
	}
	components := cfg.Components
	if len(components) == 0 {
		components = []string{"main", "contrib", "non-free"}
	}

	rl := remoteLoader{
		tracer:     tracer,
		client:     client,
		baseURL:    baseURL,
		dist:       cfg.Release,
		components: components,
	}
	releases := &RemoteReleaseLoader{
		remoteLoader:  rl,
		keyring:       kr,
		architectures: architectures,
	}
	packages := &RemotePackagesLoader{
		remoteLoader: rl,
		releases:     releases,
		parser:       PackageParser{tracer: tracer},
	}
	return releases, packages, nil
}

func (r *RemoteReleaseLoader) Load(ctx context.Context) (*Release, error) {
	ctx, span := r.tracer.Start(ctx, "debianremote.LoadRelease")
	defer span.End()

	// Fetch the InRelease (clear-signed) file:
	releaseGraph, err := r.fetchInRelease(ctx)
	if err != nil {
		return nil, err
	}
	release, err := ReleaseFromParagraph(releaseGraph)
	if err != nil {
		return nil, err
	}

	// Overwrite from configuration:
	if arch := strings.Join(r.architectures, " "); arch != release.ArchitecturesRaw {
		release.ArchitecturesRaw = arch
	}
	if comp := strings.Join(r.components, " "); comp != release.ComponentsRaw {
		release.ComponentsRaw = comp
	}
	return release, nil
}

func (r *RemoteReleaseLoader) fetchInRelease(ctx context.Context) (Paragraph, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", r.baseURL+"/dists/"+r.dist+"/InRelease", nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	graph, err := ParseReleaseFile(b, r.keyring)
	if err != nil {
		return nil, err
	}

	return graph, nil
}

func (r *RemotePackagesLoader) BaseURL() string {
	return r.baseURL + "pool/"
}

func (r *RemotePackagesLoader) LoadPackages(ctx context.Context, arch Architecture) ([]Package, error) {
	ctx, span := r.tracer.Start(ctx, "debianremote.LoadPackages")
	defer span.End()
	span.SetAttributes(attrArchitecture.String(string(arch)))

	expectedDigests, err := r.fileMetadata(ctx)
	if err != nil {
		return nil, err
	}

	var allPackages []Package
	for _, comp := range r.components {
		fn := fmt.Sprintf("%s/binary-%s/Packages.gz", comp, arch)
		digest, ok := expectedDigests[fn]
		if !ok {
			return nil, fmt.Errorf("release is missing %s/%s", comp, arch)
		}
		b, err := r.fetchPackages(ctx, digest)
		if err != nil {
			return nil, err
		}
		gzr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		pkgs, err := r.parser.ParsePackages(ctx, gzr)
		if err != nil {
			return nil, err
		}
		allPackages = append(allPackages, pkgs...)
	}

	return allPackages, nil
}

var digestRE = regexp.MustCompile(`([0-9a-f]{64})\s+([0-9]+)\s+([^ ]+)$`)

func (r *RemotePackagesLoader) fileMetadata(ctx context.Context) (map[string]PackagesDigest, error) {
	release, err := r.releases.Load(ctx)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(release.SHA256, "\n")
	digests := make(map[string]PackagesDigest, len(lines))
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
		sha, err := hex.DecodeString(m[1])
		if err != nil {
			return nil, fmt.Errorf("parsing expected sha: %w", err)
		}
		digests[path] = PackagesDigest{
			Path:   path,
			Sha256: sha,
			Size:   size,
		}
	}

	return digests, nil
}

func (r *RemotePackagesLoader) fetchPackages(ctx context.Context, digest PackagesDigest) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", r.baseURL+"/dists/"+r.dist+"/"+digest.Path, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if len(b) != digest.Size {
		return nil, fmt.Errorf("expected %d bytes, got %d", digest.Size, len(b))
	}
	if actualDigest := sha256.Sum256(b); !bytes.Equal(actualDigest[:], digest.Sha256) {
		return nil, fmt.Errorf("expected digest %x, got %x", digest.Sha256, actualDigest)
	}
	return b, nil
}
