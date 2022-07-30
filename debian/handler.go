package debian

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

// Handler implements https://wiki.debian.org/DebianRepository/Format
type Handler struct {
	log logr.Logger

	// TODO: probably per-dist?
	keyring openpgp.EntityList

	loadRelease ReleaseLoader
	packages    map[packageVersion]Package
}

type ReleaseLoader func(string) *Release

type packageVersion struct {
	pkg     string
	version string
}

func NewHandler(log logr.Logger, loadRelease ReleaseLoader, keyring openpgp.EntityList) *Handler {
	return &Handler{
		log:         log.WithName("debian"),
		loadRelease: loadRelease,
		keyring:     keyring,
		packages:    make(map[packageVersion]Package),
	}
}

func (h *Handler) Register(r *mux.Router) {
	r.HandleFunc("/debian/dists/{dist}/InRelease", h.HandleInRelease)
	r.HandleFunc("/debian/dists/{dist}/main/binary-{arch}/Packages{compression:(?:|.xz|.gz)}", h.HandlePackages)
}

func ReleaseFileLoader(base string) ReleaseLoader {
	return func(dist string) *Release {
		f, err := os.Open(filepath.Join(base, fmt.Sprintf("%s.yaml", dist)))
		if err != nil {
			return nil
		}
		defer f.Close()

		var r Release
		if err := yaml.NewDecoder(f).Decode(&r); err != nil {
			return nil
		}

		return &r
	}
}

func (h *Handler) HandleInRelease(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dist := vars["dist"]

	pk := h.keyring[0].PrivateKey
	h.log.V(1).Info("handling InRelease", "dist", dist, "private_key", pk.KeyId)

	release := h.loadRelease(dist)
	if release == nil {
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}

	enc, err := clearsign.Encode(w, pk, nil)
	if err != nil {
		h.log.Error(err, "error encoding clearsign")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := WriteReleaseFile(*release, enc); err != nil {
		h.log.Error(err, "error writing release file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := enc.Close(); err != nil {
		h.log.Error(err, "error closing clearsign")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

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
