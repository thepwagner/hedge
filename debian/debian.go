package debian

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/clearsign"
	"gopkg.in/yaml.v3"
)

// Handler implements https://wiki.debian.org/DebianRepository/Format
type Handler struct {
	log logr.Logger

	// TODO: probably per-dist?
	keyring openpgp.EntityList

	dists    map[string]Release
	packages map[packageVersion]Package
}

type packageVersion struct {
	pkg     string
	version string
}

func NewHandler(log logr.Logger, keyring openpgp.EntityList) *Handler {
	return &Handler{
		log:      log.WithName("debian"),
		keyring:  keyring,
		dists:    make(map[string]Release),
		packages: make(map[packageVersion]Package),
	}
}

func (h *Handler) Register(r *mux.Router) {
	r.HandleFunc("/debian/dists/{dist}/InRelease", h.HandleInRelease)
	r.HandleFunc("/debian/dists/{dist}/main/binary-{arch}/Packages{compression:(?:|.xz|.gz)}", h.HandlePackages)
}

func (h *Handler) LoadDist(dist string) error {
	f, err := os.Open(filepath.Join("testconfig/debian/dists", fmt.Sprintf("%s.yaml", dist)))
	if err != nil {
		return fmt.Errorf("opening dist: %w", err)
	}
	defer f.Close()

	var r Release
	if err := yaml.NewDecoder(f).Decode(&r); err != nil {
		return fmt.Errorf("decoding dist: %w", err)
	}

	h.dists[dist] = r
	h.log.Info("loaded dist", "codename", r.Codename, "packages", len(r.Packages))

	for pkgName, version := range r.Packages {
		f, err := os.Open(filepath.Join("testconfig/debian/packages", pkgName, fmt.Sprintf("%s.yaml", version)))
		if err != nil {
			return fmt.Errorf("opening dist: %w", err)
		}
		defer f.Close()

		var pkg Package
		if err := yaml.NewDecoder(f).Decode(&pkg); err != nil {
			return fmt.Errorf("decoding package: %w", err)
		}
		h.packages[packageVersion{pkg: pkgName, version: version}] = pkg
		h.log.Info("loaded package", "name", pkgName, "version", version)
	}
	return nil
}

func (h *Handler) HandleInRelease(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dist := vars["dist"]

	pk := h.keyring[0].PrivateKey
	h.log.V(1).Info("handling InRelease", "dist", dist, "private_key", pk.KeyId)

	release, ok := h.dists[dist]
	if !ok {
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}

	enc, err := clearsign.Encode(w, pk, nil)
	if err != nil {
		h.log.Error(err, "error encoding clearsign")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := WriteReleaseFile(release, enc); err != nil {
		h.log.Error(err, "error writing release file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := enc.Close(); err != nil {
		h.log.Error(err, "error closing clearsign")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "")
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

type PackagesDigest struct {
	Path   string
	Size   int
	Digest []byte
}

func PackageHashes(arch string, packages ...Package) ([]PackagesDigest, error) {
	var buf bytes.Buffer
	if err := WritePackages(&buf, packages...); err != nil {
		return nil, err
	}
	bufLen := buf.Len()
	bufDigest := sha256.Sum256(buf.Bytes())

	var xzBuf bytes.Buffer
	if err := CompressionXZ.Compress(&xzBuf, &buf); err != nil {
		return nil, err
	}
	xzLen := xzBuf.Len()
	xzDigest := sha256.Sum256(xzBuf.Bytes())

	return []PackagesDigest{
		{Path: fmt.Sprintf("main/binary-%s/Packages", arch), Size: bufLen, Digest: bufDigest[:]},
		{Path: fmt.Sprintf("main/binary-%s/Packages.xz", arch), Size: xzLen, Digest: xzDigest[:]},
	}, nil
}
