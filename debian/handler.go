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
)

// Handler implements https://wiki.debian.org/DebianRepository/Format
type Handler struct {
	log   logr.Logger
	dists map[string]distHandler
}

func NewHandler(log logr.Logger, repos ...RepositoryConfig) (*Handler, error) {
	dists := make(map[string]distHandler, len(repos))
	for _, cfg := range repos {
		dh, err := newDistHandler(log, cfg)
		if err != nil {
			return nil, err
		}
		dists[cfg.Name] = *dh
	}
	return &Handler{
		log:   log.WithName("debian.Handler"),
		dists: dists,
	}, nil
}

func (h *Handler) Register(r *mux.Router) {
	r.HandleFunc("/debian/dists/{dist}/InRelease", h.HandleInRelease)
	r.HandleFunc("/debian/dists/{dist}/{comp}/binary-{arch}/Packages{compression:(?:|.xz|.gz)}", h.HandlePackages)
	r.HandleFunc("/debian/dists/{dist}/pool/{path:.*}", h.HandlePool)
}

func (h *Handler) HandleInRelease(w http.ResponseWriter, r *http.Request) {
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
	pk      *packet.PrivateKey
	release ReleaseLoader
}

func newDistHandler(log logr.Logger, cfg RepositoryConfig) (*distHandler, error) {
	if cfg.Key == "" {
		return nil, fmt.Errorf("missing key")
	}
	keyring, err := ReadArmoredKeyRingFile(cfg.Key)
	if err != nil {
		return nil, err
	}

	var release ReleaseLoader
	if upCfg := cfg.Source.Upstream; upCfg != nil {
		release, err = NewRemoteLoader(log, *cfg.Source.Upstream)
	} else {
		return nil, fmt.Errorf("no source specified")
	}
	if err != nil {
		return nil, err
	}

	// TODO: configure the filters here

	return &distHandler{
		log:     log,
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

	if err := clearSign(w, dh.pk, func(out io.Writer) error { return WriteReleaseFile(*release, packages, out) }); err != nil {
		log.Error(err, "writing signed release")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func clearSign(out io.Writer, pk *packet.PrivateKey, writer func(io.Writer) error) error {
	enc, err := clearsign.Encode(out, pk, nil)
	if err != nil {
		return err
	}
	if err := writer(enc); err != nil {
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
