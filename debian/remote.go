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
	packageFilter filter.Predicate[Package]

	releaseMu    sync.Mutex
	releaseGraph Paragraph

	packagesMu sync.RWMutex
	packages   map[Component]map[Architecture][]Package
}

func NewRemoteLoader(tp trace.TracerProvider, cfg UpstreamConfig, filters []FilterRule) (*RemoteLoader, error) {
	if cfg.Release == "" {
		return nil, fmt.Errorf("missing release")
	}

	if cfg.Key == "" {
		return nil, fmt.Errorf("missing keyfile")
	}
	kr, err := ReadArmoredKeyRingFile(cfg.Key)
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
	l := &RemoteLoader{
		tracer:        tp.Tracer("hedge"),
		baseURL:       baseURL,
		keyring:       kr,
		dist:          cfg.Release,
		client:        &http.Client{Transport: tr},
		architectures: architectures,
		components:    components,
		packages:      map[Component]map[Architecture][]Package{},
	}

	var predicates []filter.Predicate[Package]
	for _, f := range filters {
		if f.Priority != "" {
			predicates = append(predicates, filter.MatchesPriority[Package](f.Name))
		}
		if f.Name != "" {
			predicates = append(predicates, filter.MatchesName[Package](f.Name))
		}
		if f.Pattern != "" {
			predicate, err := filter.MatchesPattern[Package](f.Pattern)
			if err != nil {
				return nil, err
			}
			predicates = append(predicates, predicate)
		}
	}
	l.packageFilter = filter.AnyOf(predicates...)
	return l, nil
}

func (r *RemoteLoader) BaseURL() string {
	return r.baseURL
}

func (r *RemoteLoader) Load(ctx context.Context) (*Release, map[Component]map[Architecture][]Package, error) {
	ctx, span := r.tracer.Start(ctx, "debian-loader.Load")
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

	pkgs := map[Component]map[Architecture][]Package{}
	for _, comp := range release.Components() {
		compPkgs := map[Architecture][]Package{}
		for _, arch := range release.Architectures() {
			pkgs, err := r.LoadPackages(ctx, comp, arch)
			if err != nil {
				return nil, nil, err
			}
			compPkgs[arch] = pkgs
		}
		pkgs[comp] = compPkgs
	}
	return release, pkgs, nil
}

func (r *RemoteLoader) fetchInRelease(ctx context.Context) (Paragraph, error) {
	ctx, span := r.tracer.Start(ctx, "debian-loader.FetchInRelease")
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

func (r *RemoteLoader) LoadPackages(ctx context.Context, comp Component, arch Architecture) ([]Package, error) {
	ctx, span := r.tracer.Start(ctx, "debian-loader.LoadPackages")
	defer span.End()
	span.SetAttributes(attribute.String("component", string(comp)), attribute.String("architecture", string(arch)))

	r.packagesMu.RLock()
	if pkgsByArch, ok := r.packages[comp]; ok {
		if pkgs, ok := pkgsByArch[arch]; ok {
			r.packagesMu.RUnlock()
			return pkgs, nil
		}
	}
	r.packagesMu.RUnlock()
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

	_, parseSpan := r.tracer.Start(ctx, "debian-loader.LoadPackages.Parse")
	gzr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		parseSpan.End()
		return nil, err
	}
	pkgs, err := ParsePackages(gzr)
	if err != nil {
		parseSpan.End()
		return nil, err
	}
	parseSpan.End()

	_, filterSpan := r.tracer.Start(ctx, "debian-loader.LoadPackages.Filter")
	var filteredCount int
	mapped := make([]Package, 0, len(pkgs))
	for _, pkg := range pkgs {
		pkg.Filename = "dists/" + r.dist + "/" + pkg.Filename

		if ok, err := r.packageFilter(ctx, pkg); err != nil {
			filterSpan.End()
			return nil, err
		} else if !ok {
			filteredCount++
			continue
		}

		mapped = append(mapped, pkg)
	}
	filterSpan.End()
	span.SetAttributes(attribute.Int("package_count", len(mapped)), attribute.Int("filtered_count", filteredCount))

	r.packagesMu.Lock()
	defer r.packagesMu.Unlock()
	pkgsByArch, ok := r.packages[comp]
	if !ok {
		pkgsByArch = map[Architecture][]Package{}
	}
	pkgsByArch[arch] = mapped
	r.packages[comp] = pkgsByArch
	return mapped, nil
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
