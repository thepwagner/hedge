package debian

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/thepwagner/hedge/pkg/cached"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

type remoteLoader struct {
	tracer trace.Tracer
	client *http.Client

	fetchURL cached.Function[string, []byte]

	baseURL    string
	dist       string
	components []string
}

type RemoteReleaseLoader struct {
	remoteLoader
	keyring       openpgp.EntityList
	architectures []string
	cacheDuration time.Duration
}

type RemotePackagesLoader struct {
	remoteLoader
	releases ReleaseLoader
	parser   PackageParser
}

func NewRemoteLoader(tracer trace.Tracer, client *http.Client, storage cached.ByteStorage, cfg UpstreamConfig) (*RemoteReleaseLoader, *RemotePackagesLoader, error) {
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
	rl.fetchURL = cached.Cached[string, []byte](cached.WithPrefix[[]byte]("debian_urls", storage), 5*time.Minute, rl.FetchURL)

	releases := &RemoteReleaseLoader{
		remoteLoader:  rl,
		keyring:       kr,
		architectures: architectures,
		cacheDuration: 5 * time.Minute,
	}
	packages := &RemotePackagesLoader{
		remoteLoader: rl,
		releases:     releases,
		parser:       PackageParser{tracer: tracer},
	}
	return releases, packages, nil
}

func (r remoteLoader) FetchURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *RemoteReleaseLoader) Load(ctx context.Context) (*Release, error) {
	ctx, span := r.tracer.Start(ctx, "debianremote.LoadRelease", trace.WithAttributes(attrDist(r.dist)))
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

	span.SetAttributes(
		attribute.StringSlice("debian_architectures_remote", strings.Split(release.ArchitecturesRaw, " ")),
		attribute.StringSlice("debian_components_remote", strings.Split(release.ComponentsRaw, " ")),
		attribute.StringSlice("debian_architectures", r.architectures),
		attrComponents(r.components),
	)
	release.ArchitecturesRaw = strings.Join(r.architectures, " ")
	release.ComponentsRaw = strings.Join(r.components, " ")

	return release, nil
}

func (r *RemoteReleaseLoader) fetchInRelease(ctx context.Context) (Paragraph, error) {
	ctx, span := r.tracer.Start(ctx, "debianremote.fetchInRelease")
	defer span.End()

	b, err := r.fetchURL(ctx, r.baseURL+"/dists/"+r.dist+"/InRelease")
	if err != nil {
		return nil, fmt.Errorf("fetching release file: %w", err)
	}

	graph, err := ParseReleaseFile(b, r.keyring)
	if err != nil {
		return nil, fmt.Errorf("parsing release file: %w", err)
	}
	return graph, nil
}

func (r *RemotePackagesLoader) BaseURL() string {
	return r.baseURL + "pool/"
}

func (r *RemotePackagesLoader) LoadPackages(ctx context.Context, release *Release, arch Architecture) ([]Package, error) {
	ctx, span := r.tracer.Start(ctx, "debianremote.LoadPackages", trace.WithAttributes(attrArchitecture(arch), attrComponents(r.components)))
	defer span.End()

	expectedDigests, err := r.fileMetadata(ctx, release)
	if err != nil {
		return nil, err
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(4)
	res := make(chan []Package)
	for _, comp := range r.components {
		comp := comp
		eg.Go(func() error {
			ctx, span := r.tracer.Start(ctx, "debianremote.LoadPackages.component", trace.WithAttributes(attrComponent(comp)))
			defer span.End()

			fn := fmt.Sprintf("%s/binary-%s/Packages.gz", comp, arch)
			digest, ok := expectedDigests[fn]
			if !ok {
				return fmt.Errorf("release is missing %s/%s", comp, arch)
			}
			b, err := r.fetchPackages(ctx, digest)
			if err != nil {
				return err
			}
			gzr, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				return err
			}
			pkgs, err := r.parser.ParsePackages(ctx, gzr)
			if err != nil {
				return err
			}
			res <- pkgs
			return nil
		})
	}
	go func() {
		_ = eg.Wait()
		close(res)
	}()

	var allPackages []Package
	for pkgs := range res {
		allPackages = append(allPackages, pkgs...)
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return allPackages, nil
}

var digestRE = regexp.MustCompile(`([0-9a-f]{64})\s+([0-9]+)\s+([^ ]+)$`)

func (r *RemotePackagesLoader) fileMetadata(ctx context.Context, release *Release) (map[string]PackagesDigest, error) {
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
			Path:   fmt.Sprintf("%s/by-hash/SHA256/%x", filepath.Dir(path), sha),
			Sha256: sha,
			Size:   size,
		}
	}

	return digests, nil
}

func (r *RemotePackagesLoader) fetchPackages(ctx context.Context, digest PackagesDigest) ([]byte, error) {
	p, err := url.JoinPath("dists", r.dist, digest.Path)
	if err != nil {
		return nil, err
	}

	// Extend the TTL, since the URL contains the expected digest
	b, err := r.fetchURL(cached.For(ctx, 7*24*time.Hour), r.baseURL+p)
	if err != nil {
		return nil, err
	}

	// Verify and store to cache, without expiry since the key contains content hash
	if len(b) != digest.Size {
		return nil, fmt.Errorf("expected %d bytes, got %d", digest.Size, len(b))
	}
	if actualDigest := sha256.Sum256(b); !bytes.Equal(actualDigest[:], digest.Sha256) {
		return nil, fmt.Errorf("expected digest %x, got %x", digest.Sha256, actualDigest)
	}
	return b, nil
}
