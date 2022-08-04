package debian

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/gorilla/mux"
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
	pk      *packet.PrivateKey
	release ReleaseLoader
}

type ReleaseLoader interface {
	BaseURL() string
	Load(context.Context) (*Release, map[Component]map[Architecture][]Package, error)
	LoadPackages(ctx context.Context, comp Component, arch Architecture) ([]Package, error)
}

func NewHandler(tp trace.TracerProvider, repos map[string]*RepositoryConfig) (*Handler, error) {
	dists := make(map[string]distConfig, len(repos))
	for name, cfg := range repos {
		dc, err := newDistConfig(tp, cfg)
		if err != nil {
			return nil, err
		}
		dists[name] = *dc
	}
	return &Handler{
		tracer: tp.Tracer("hedge"),
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

	release, packages, err := dist.release.Load(ctx)
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
	var components []string
	var architectures []string
	for comp, arches := range packages {
		components = append(components, string(comp))

		// Assume every component has the same architectures, use the first.
		if len(architectures) == 0 {
			for arch := range arches {
				architectures = append(architectures, string(arch))
			}
		}
	}
	span.SetAttributes(attrComponents.StringSlice(components), attrArchitectures.StringSlice(architectures))

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
	comp := vars["comp"]
	compression := FromExtension(vars["compression"])
	span.SetAttributes(attrArchitecture.String(arch), attrComponent.String(comp), attribute.String("compression", string(compression)))

	pkgs, err := dist.release.LoadPackages(ctx, Component(comp), Architecture(arch))
	if err != nil {
		span.RecordError(err)
		http.Error(w, "error loading remote packages", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := WritePackages(&buf, pkgs...); err != nil {
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
	url := dist.release.BaseURL() + "pool/" + path
	r = r.WithContext(ctx)
	http.Redirect(w, r, url, http.StatusMovedPermanently)
}

func newDistConfig(tp trace.TracerProvider, cfg *RepositoryConfig) (*distConfig, error) {
	if cfg.Key == "" {
		return nil, fmt.Errorf("missing key")
	}
	key, err := ReadArmoredKeyRingFile(cfg.Key)
	if err != nil {
		return nil, fmt.Errorf("reading key: %w", err)
	}

	var release ReleaseLoader
	if upCfg := cfg.Source.Upstream; upCfg != nil {
		release, err = NewRemoteLoader(tp, *cfg.Source.Upstream, cfg.Filters)
	} else {
		return nil, fmt.Errorf("no source specified")
	}
	if err != nil {
		return nil, err
	}

	return &distConfig{
		pk:      key[0].PrivateKey,
		release: release,
	}, nil
}
