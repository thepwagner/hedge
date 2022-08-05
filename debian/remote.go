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
	"sync"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/thepwagner/hedge/pkg/filter"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type RemoteLoader struct {
	tracer trace.Tracer
	client *http.Client

	baseURL       string
	dist          string
	keyring       openpgp.EntityList
	architectures []string
	components    []string
	pkgFilter     filter.Predicate[Package]
	parser        PackageParser

	// TODO: move this caching to redis(?)
	releaseMu    sync.Mutex
	releaseGraph Paragraph

	packagesMu sync.RWMutex
	packages   map[Architecture][]Package
}

func NewRemoteLoader(tp trace.TracerProvider, cfg UpstreamConfig, pkgFilter filter.Predicate[Package]) (*RemoteLoader, error) {
	if cfg.Release == "" {
		return nil, fmt.Errorf("missing release")
	}

	if cfg.Key == "" {
		return nil, fmt.Errorf("missing keyfile")
	}
	kr, err := openpgp.ReadArmoredKeyRing(strings.NewReader(cfg.Key))
	if err != nil {
		return nil, err
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

	tr := otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithTracerProvider(tp))
	tracer := tp.Tracer("hedge")
	l := &RemoteLoader{
		tracer:        tracer,
		pkgFilter:     pkgFilter,
		baseURL:       baseURL,
		keyring:       kr,
		dist:          cfg.Release,
		client:        &http.Client{Transport: tr},
		architectures: architectures,
		components:    components,
		packages:      map[Architecture][]Package{},
		parser:        PackageParser{tracer: tracer},
	}
	return l, nil
}

func (r *RemoteLoader) BaseURL() string {
	return r.baseURL
}

func (r *RemoteLoader) Load(ctx context.Context) (*Release, map[Architecture][]Package, error) {
	ctx, span := r.tracer.Start(ctx, "debianremote.Load")
	defer span.End()

	// Fetch the InRelease (clear-signed) file:
	releaseGraph, err := r.fetchInRelease(ctx)
	if err != nil {
		return nil, nil, err
	}
	release, err := ReleaseFromParagraph(releaseGraph)
	if err != nil {
		return nil, nil, err
	}

	// Overwrite from configuration:
	if arch := strings.Join(r.architectures, " "); arch != release.ArchitecturesRaw {
		release.ArchitecturesRaw = arch
	}
	if comp := strings.Join(r.components, " "); comp != release.ComponentsRaw {
		release.ComponentsRaw = comp
	}

	archPkgs := map[Architecture][]Package{}
	for _, arch := range release.Architectures() {
		pkgs, err := r.LoadPackages(ctx, arch)
		if err != nil {
			return nil, nil, err
		}
		archPkgs[arch] = pkgs
	}
	return release, archPkgs, nil
}

func (r *RemoteLoader) fetchInRelease(ctx context.Context) (Paragraph, error) {
	ctx, span := r.tracer.Start(ctx, "debianremote.FetchInRelease")
	defer span.End()

	r.releaseMu.Lock()
	defer r.releaseMu.Unlock()
	if r.releaseGraph != nil {
		return r.releaseGraph, nil
	}

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

	r.releaseGraph = graph
	span.SetAttributes(attribute.Int("bytes_count", len(b)))
	return graph, nil
}

func (r *RemoteLoader) LoadPackages(ctx context.Context, arch Architecture) ([]Package, error) {
	ctx, span := r.tracer.Start(ctx, "debianremote.LoadPackages")
	defer span.End()
	span.SetAttributes(attrArchitecture.String(string(arch)))

	r.packagesMu.RLock()
	if pkgsByArch, ok := r.packages[arch]; ok {
		r.packagesMu.RUnlock()
		return pkgsByArch, nil
	}
	r.packagesMu.RUnlock()

	var allPackages []Package
	for _, comp := range r.components {
		fn := fmt.Sprintf("%s/binary-%s/Packages.gz", comp, arch)
		expectedDigest, expectedSize, err := r.fileMetadata(ctx, fn)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, "GET", r.baseURL+"/dists/"+r.dist+"/"+fn, nil)
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

		if len(b) != expectedSize {
			return nil, fmt.Errorf("expected %d bytes, got %d", expectedSize, len(b))
		}
		actualDigest := sha256.Sum256(b)
		if digest := hex.EncodeToString(actualDigest[:]); digest != expectedDigest {
			return nil, fmt.Errorf("expected digest %s, got %s", expectedDigest, digest)
		}

		gzr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		pkgs, err := r.parser.ParsePackages(ctx, gzr)
		if err != nil {
			return nil, err
		}

		_, filterSpan := r.tracer.Start(ctx, "debianremote.LoadPackages.Filter")
		var filteredCount int
		mapped := make([]Package, 0, len(pkgs))
		for _, pkg := range pkgs {
			pkg.Filename = "dists/" + r.dist + "/" + pkg.Filename
			if ok, err := r.pkgFilter(ctx, pkg); err != nil {
				filterSpan.End()
				return nil, err
			} else if !ok {
				filteredCount++
				continue
			}

			allPackages = append(allPackages, pkg)
		}
		filterSpan.SetAttributes(attrPackageCount.Int(len(mapped)), attribute.Int("filtered_count", filteredCount))
		filterSpan.End()
		span.SetAttributes(attrPackageCount.Int(len(mapped)), attribute.Int("filtered_count", filteredCount))
	}

	r.packagesMu.Lock()
	defer r.packagesMu.Unlock()
	r.packages[arch] = allPackages
	return allPackages, nil
}

var digestRE = regexp.MustCompile(`([0-9a-f]{64})\s+([0-9]+)\s+([^ ]+)$`)

func (r *RemoteLoader) fileMetadata(ctx context.Context, fn string) (string, int, error) {
	rg, err := r.fetchInRelease(ctx)
	if err != nil {
		return "", 0, err
	}
	for _, line := range strings.Split(rg["SHA256"], "\n") {
		m := digestRE.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}
		if m[3] != fn {
			continue
		}
		size, err := strconv.Atoi(m[2])
		if err != nil {
			return "", 0, fmt.Errorf("parsing expected size")
		}
		return m[1], size, nil
	}
	return "", 0, fmt.Errorf("file not found: %w", err)
}
