package debian

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
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
	r.HandleFunc("/debian/dists/{dist}/main/binary-{arch}/Packages{compression:(?:|.xz|.gz)}", h.HandlePackages)
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

type ReleaseLoader interface {
	Load(context.Context) (*Release, error)
}

type distHandler struct {
	log     logr.Logger
	keyring openpgp.EntityList
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
		keyring: keyring,
		release: release,
	}, nil
}

func (dh *distHandler) HandleInRelease(w http.ResponseWriter, r *http.Request) {
	pk := dh.keyring[0].PrivateKey
	log := observability.Logger(r.Context(), dh.log)
	log.V(1).Info("handling InRelease", "private_key", pk.KeyId)

	release, err := dh.release.Load(r.Context())
	if err != nil {
		log.Error(err, "release loading error")
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}
	if release == nil {
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}

	enc, err := clearsign.Encode(w, pk, nil)
	if err != nil {
		log.Error(err, "error encoding clearsign")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := WriteReleaseFile(*release, enc); err != nil {
		log.Error(err, "error writing release file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := enc.Close(); err != nil {
		log.Error(err, "error closing clearsign")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// TODO: dist-ify
func (h *Handler) HandlePackages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dist := vars["dist"]
	arch := vars["arch"]
	compression := FromExtension(vars["compression"])

	h.log.V(1).Info("handling Packages", "dist", dist, "arch", arch)

	var buf bytes.Buffer
	if err := WritePackages(&buf, hackPackages...); err != nil {
		h.log.Error(err, "writing packages")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := compression.Compress(w, &buf); err != nil {
		h.log.Error(err, "writing packages")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
