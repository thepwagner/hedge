package debian

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/pkg/observability"
	"go.opentelemetry.io/otel/trace"
)

// Handler implements https://wiki.debian.org/DebianRepository/Format
type Handler struct {
	log    logr.Logger
	tracer trace.Tracer
	dists  map[string]distHandler
}

func NewHandler(log logr.Logger, tp trace.TracerProvider, repos ...RepositoryConfig) (*Handler, error) {
	dists := make(map[string]distHandler, len(repos))
	for _, cfg := range repos {
		dh, err := newDistHandler(log, tp, cfg)
		if err != nil {
			return nil, err
		}
		dists[cfg.Name] = *dh
	}
	return &Handler{
		log:    log.WithName("debian.Handler"),
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
	r = r.WithContext(ctx)

	distName := mux.Vars(r)["dist"]
	dist, ok := h.dists[distName]
	if !ok {
		h.log.Info("dist not found", "dist", distName)
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}
	dist.HandleInRelease(w, r)
}

func (h *Handler) HandlePackages(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "debian.HandlePackages")
	defer span.End()
	r = r.WithContext(ctx)

	distName := mux.Vars(r)["dist"]
	dist, ok := h.dists[distName]
	if !ok {
		h.log.Info("dist not found", "dist", distName)
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}
	dist.HandlePackages(w, r)
}

func (h *Handler) HandlePool(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "debian.HandlePool")
	defer span.End()
	r = r.WithContext(ctx)

	distName := mux.Vars(r)["dist"]
	dist, ok := h.dists[distName]
	if !ok {
		h.log.Info("dist not found", "dist", distName)
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}
	dist.HandlePool(w, r)
}

type ReleaseLoader interface {
	BaseURL() string
	Load(context.Context) (*Release, map[Component]map[Architecture][]Package, error)
	LoadPackages(ctx context.Context, comp Component, arch Architecture) ([]Package, error)
}

type distHandler struct {
	log     logr.Logger
	tracer  trace.Tracer
	pk      *packet.PrivateKey
	release ReleaseLoader
}

func newDistHandler(log logr.Logger, tp trace.TracerProvider, cfg RepositoryConfig) (*distHandler, error) {
	if cfg.Key == "" {
		return nil, fmt.Errorf("missing key")
	}
	keyring, err := ReadArmoredKeyRingFile(cfg.Key)
	if err != nil {
		return nil, err
	}

	var release ReleaseLoader
	if upCfg := cfg.Source.Upstream; upCfg != nil {
		release, err = NewRemoteLoader(log, tp, *cfg.Source.Upstream, cfg.Filters)
	} else {
		return nil, fmt.Errorf("no source specified")
	}
	if err != nil {
		return nil, err
	}

	return &distHandler{
		log:     log,
		tracer:  tp.Tracer("hedge"),
		pk:      keyring[0].PrivateKey,
		release: release,
	}, nil
}

func (dh *distHandler) HandleInRelease(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := observability.Logger(ctx, dh.log)
	log.V(1).Info("handling InRelease", "private_key", hex.EncodeToString(dh.pk.Fingerprint))

	release, packages, err := dh.release.Load(ctx)
	if err != nil {
		log.Error(err, "release loading error")
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}
	if release == nil {
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}

	if err := dh.clearSign(ctx, w, func(ctx context.Context, out io.Writer) error {
		return WriteReleaseFile(ctx, *release, packages, out)
	}); err != nil {
		log.Error(err, "writing signed release")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func (dh *distHandler) clearSign(ctx context.Context, out io.Writer, writer func(context.Context, io.Writer) error) error {
	ctx, span := dh.tracer.Start(ctx, "debian.clearSign")
	defer span.End()

	enc, err := clearsign.Encode(out, dh.pk, nil)
	if err != nil {
		return err
	}
	if err := writer(ctx, enc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out)
	return err
}

func (dh *distHandler) HandlePackages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	arch := Architecture(vars["arch"])
	comp := Component(vars["comp"])
	compression := FromExtension(vars["compression"])
	ctx := r.Context()

	log := observability.Logger(ctx, dh.log)
	log.V(1).Info("handling Packages", "arch", arch)

	pkgs, err := dh.release.LoadPackages(ctx, comp, arch)
	if err != nil {
		log.Error(err, "loading packages")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := WritePackages(&buf, pkgs...); err != nil {
		log.Error(err, "writing packages")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := compression.Compress(w, &buf); err != nil {
		log.Error(err, "writing packages")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (dh *distHandler) HandlePool(w http.ResponseWriter, r *http.Request) {
	path := mux.Vars(r)["path"]
	log := observability.Logger(r.Context(), dh.log)
	log.V(1).Info("handling pool", "path", path)
	url := dh.release.BaseURL() + "pool/" + path
	http.Redirect(w, r, url, http.StatusMovedPermanently)
}
