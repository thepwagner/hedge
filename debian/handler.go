package debian

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/pkg/filter"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Handler implements https://wiki.debian.org/DebianRepository/Format
type Handler struct {
	tracer trace.Tracer
	dists  map[string]distConfig
}

type distConfig struct {
	pk       *packet.PrivateKey
	release  ReleaseLoader
	packages PackagesLoader
}

func NewHandler(tracer trace.Tracer, client *http.Client, cfgDir string, repos map[string]*RepositoryConfig) (*Handler, error) {
	dists := make(map[string]distConfig, len(repos))
	for name, cfg := range repos {
		dist, err := newDistConfig(tracer, client, cfgDir, cfg)
		if err != nil {
			return nil, err
		}
		dists[name] = *dist
	}
	return &Handler{
		tracer: tracer,
		dists:  dists,
	}, nil
}

func (h *Handler) Register(r *mux.Router) {
	r.HandleFunc("/debian/dists/{dist}/InRelease", h.HandleInRelease)
	r.HandleFunc("/debian/dists/{dist}/{comp}/binary-{arch}/Packages{compression:(?:|.xz|.gz)}", h.HandlePackages)
	r.HandleFunc("/debian/dists/{dist}/pool/{path:.*}", h.HandlePool)
}

func (h *Handler) HandleInRelease(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "debian.HandleInRelease")
	defer span.End()
	distName := mux.Vars(r)["dist"]
	span.SetAttributes(attrDist.String(distName))

	dist, ok := h.dists[distName]
	if !ok {
		span.SetStatus(codes.Error, "dist not found")
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}

	// Load the release metadata:
	release, err := dist.release.Load(ctx)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "error loading remote release", http.StatusInternalServerError)
		return
	}
	if release == nil {
		span.SetStatus(codes.Error, "remote release not found")
		http.Error(w, "remote release not found", http.StatusInternalServerError)
		return
	}

	// The Release file contains hashes of all Packages files, so we need to load them:
	packages := map[Architecture][]Package{}
	for _, arch := range release.Architectures() {
		pkgs, err := dist.packages.LoadPackages(ctx, arch)
		if err != nil {
			span.RecordError(err)
			http.Error(w, "error loading remote packages", http.StatusInternalServerError)
			return
		}
		packages[arch] = pkgs
	}

	// Write the signed InRelease file:
	ctx, span = h.tracer.Start(ctx, "debian.clearSign")
	defer span.End()
	enc, err := clearsign.Encode(w, dist.pk, nil)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "error signing release data", http.StatusInternalServerError)
		return
	}
	if err := WriteReleaseFile(ctx, *release, packages, enc); err != nil {
		span.RecordError(err)
		return
	}
	if err := enc.Close(); err != nil {
		span.RecordError(err)
		return
	}
	if _, err = fmt.Fprintln(w); err != nil {
		span.RecordError(err)
		return
	}
}

func (h *Handler) HandlePackages(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "debian.HandlePackages")
	defer span.End()
	vars := mux.Vars(r)
	distName := vars["dist"]
	span.SetAttributes(attrDist.String(distName))

	dist, ok := h.dists[distName]
	if !ok {
		span.SetStatus(codes.Error, "dist not found")
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}

	arch := vars["arch"]
	compression := FromExtension(vars["compression"])
	span.SetAttributes(attrArchitecture.String(arch), attribute.String("compression", string(compression)))

	// Load and serve the packages list. The client expects this to match what HandleInRelease digested
	pkgs, err := dist.packages.LoadPackages(ctx, Architecture(arch))
	if err != nil {
		span.RecordError(err)
		http.Error(w, "error loading remote packages", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := WriteControlFile(&buf, pkgs...); err != nil {
		span.RecordError(err)
		http.Error(w, "error writing package file", http.StatusInternalServerError)
		return
	}
	if err := compression.Compress(w, &buf); err != nil {
		span.RecordError(err)
	}
}

func (h *Handler) HandlePool(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "debian.HandlePool")
	defer span.End()
	vars := mux.Vars(r)
	distName := vars["dist"]
	span.SetAttributes(attrDist.String(distName))

	dist, ok := h.dists[distName]
	if !ok {
		span.SetStatus(codes.Error, "dist not found")
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}

	path := vars["path"]
	url := dist.packages.BaseURL() + "pool/" + path
	r = r.WithContext(ctx)
	http.Redirect(w, r, url, http.StatusMovedPermanently)
}

func newDistConfig(tracer trace.Tracer, client *http.Client, cfgDir string, cfg *RepositoryConfig) (*distConfig, error) {
	// Load the private signing key
	if cfg.KeyPath == "" {
		return nil, fmt.Errorf("missing key")
	}
	key, err := ReadArmoredKeyRingFile(cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading key: %w", err)
	}

	// Start with a package source:
	var release ReleaseLoader
	var packages PackagesLoader
	if upCfg := cfg.Source.Upstream; upCfg != nil {
		var rpl *RemotePackagesLoader
		release, rpl, err = NewRemoteLoader(tracer, client, *cfg.Source.Upstream)
		packages = rpl
		defer func() {
			rpl.releases = release
		}()
	} else {
		return nil, fmt.Errorf("no source specified")
	}
	if err != nil {
		return nil, err
	}

	// Apply the policies to filter packages from the source:
	pkgFilter, err := filter.CueConfigToPredicate[Package](filepath.Join(cfgDir, "debian", "policies"), cfg.Policies)
	if err != nil {
		return nil, fmt.Errorf("parsing policies: %w", err)
	}
	packages = NewFilteredPackageLoader(packages, pkgFilter)

	// TODO: freeze responses in-memory for lazy caching
	release = freezeReleaseLoader(release)
	packages = freezePackagesLoader(packages)

	return &distConfig{
		pk:       key[0].PrivateKey,
		release:  release,
		packages: packages,
	}, nil
}
