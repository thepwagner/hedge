package debian

import (
	"bytes"
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
	"github.com/go-logr/logr"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/ulikunitz/xz"
)

type RemoteLoader struct {
	log    logr.Logger
	client *http.Client

	baseURL       string
	dist          string
	keyring       openpgp.EntityList
	architectures []string
	components    []string

	releaseMu    sync.Mutex
	releaseGraph Paragraph

	packagesMu sync.RWMutex
	packages   map[Component]map[Architecture][]Package
}

func NewRemoteLoader(log logr.Logger, cfg UpstreamConfig) (*RemoteLoader, error) {
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

	l := &RemoteLoader{
		log:           log.WithName("debian-loader").WithValues("release", cfg.Release),
		baseURL:       baseURL,
		keyring:       kr,
		dist:          cfg.Release,
		client:        http.DefaultClient,
		architectures: architectures,
		components:    components,
		packages:      map[Component]map[Architecture][]Package{},
	}
	l.log.Info("created remote debian loader", "base_url", l.baseURL, "architectures", l.architectures, "components", l.components)
	return l, nil
}

func (r *RemoteLoader) BaseURL() string {
	return r.baseURL
}

func (r *RemoteLoader) Load(ctx context.Context) (*Release, map[Component]map[Architecture][]Package, error) {
	log := observability.Logger(ctx, r.log).V(1)
	log.Info("loading release")

	// Fetch the InRelease (clear-signed) file:
	releaseGraph, err := r.fetchInRelease(ctx, log)
	if err != nil {
		return nil, nil, err
	}
	release, err := ReleaseFromParagraph(releaseGraph)
	if err != nil {
		return nil, nil, err
	}

	// Overwrite from configuration:
	if arch := strings.Join(r.architectures, " "); arch != release.ArchitecturesRaw {
		log.Info("overwriting architectures", "old", release.ArchitecturesRaw, "new", arch)
		release.ArchitecturesRaw = arch
	}
	if comp := strings.Join(r.components, " "); comp != release.ComponentsRaw {
		log.Info("overwriting components", "old", release.ComponentsRaw, "new", comp)
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

func (r *RemoteLoader) fetchInRelease(ctx context.Context, log logr.Logger) (Paragraph, error) {
	r.releaseMu.Lock()
	defer r.releaseMu.Unlock()
	if r.releaseGraph != nil {
		log.Info("returning Release from cache")
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
	log.Info("fetched InRelease from remote", "bytes_count", len(b))
	return graph, nil
}

func (r *RemoteLoader) LoadPackages(ctx context.Context, comp Component, arch Architecture) ([]Package, error) {
	log := observability.Logger(ctx, r.log).V(1).WithValues("arch", arch, "component", comp)

	r.packagesMu.RLock()
	if pkgsByArch, ok := r.packages[comp]; ok {
		if pkgs, ok := pkgsByArch[arch]; ok {
			r.packagesMu.RUnlock()
			log.Info("returning packages from cache")
			return pkgs, nil
		}
	}
	r.packagesMu.RUnlock()

	log.Info("fetching Packages")

	fn := fmt.Sprintf("%s/binary-%s/Packages.xz", comp, arch)
	expectedDigest, expectedSize, err := r.fileMetadata(ctx, log, fn)
	if err != nil {
		return nil, err
	}
	log.Info("found expected digest and size", "digest", expectedDigest, "size", expectedSize)

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
	log.Info("verified expected digest and size")

	xzD, err := xz.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	pkgs, err := ParsePackages(xzD)
	if err != nil {
		return nil, err
	}

	mapped := make([]Package, 0, len(pkgs))
	for _, pkg := range pkgs {
		pkg.Filename = "dists/" + r.dist + "/" + pkg.Filename
		mapped = append(mapped, pkg)
	}

	log.Info("parsed packages", "package_count", len(pkgs))

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

func (r *RemoteLoader) fileMetadata(ctx context.Context, log logr.Logger, fn string) (string, int, error) {
	rg, err := r.fetchInRelease(ctx, log)
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
