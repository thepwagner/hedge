package debian

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/pkg/registry"
	"github.com/thepwagner/hedge/pkg/registry/base"
)

// Handler implements https://wiki.debian.org/DebianRepository/Format
type Handler struct {
	base.Handler[*repositoryHandler]
}

func (h Handler) Register(r *mux.Router) {
	r.HandleFunc("/debian/dists/{repository}/InRelease", h.HandleInRelease)
	r.HandleFunc("/debian/dists/{repository}/main/binary-{arch}/Packages{compression:(?:|.xz|.gz)}", h.HandlePackages)
	// r.HandleFunc("/debian/dists/{repository}/pool/{path:.*}", h.HandlePool)
}

func (h Handler) HandleInRelease(w http.ResponseWriter, r *http.Request) {
	h.RepositoryHandler(w, r, "debian.HandleInRelease", func(ctx context.Context, vars map[string]string, rh *repositoryHandler) error {
		// Load the release metadata:
		release, err := rh.release.Load(ctx)
		if err != nil {
			return err
		}
		if release == nil {
			return fmt.Errorf("remote release not found")
		}

		// The Release file contains hashes of all Packages files, so we need to load them:
		packages := map[Architecture][]Package{}
		for _, arch := range release.Architectures() {
			pkgs, err := rh.packages.LoadPackages(ctx, release, arch)
			if err != nil {
				return err
			}
			packages[arch] = pkgs
		}

		// Write the signed InRelease file:
		ctx, span := h.Tracer.Start(ctx, "debian.clearSign")
		defer span.End()
		enc, err := clearsign.Encode(w, rh.pk, nil)
		if err != nil {
			return err
		}
		if err := WriteReleaseFile(ctx, *release, packages, enc); err != nil {
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
		if _, err = fmt.Fprintln(w); err != nil {
			return err
		}
		return nil
	})
}

func (h Handler) HandlePackages(w http.ResponseWriter, r *http.Request) {
	h.RepositoryHandler(w, r, "debian.HandlePackages", func(ctx context.Context, vars map[string]string, rh *repositoryHandler) error {
		arch := Architecture(vars["arch"])
		compression := FromExtension(vars["compression"])

		release, err := rh.release.Load(ctx)
		if err != nil {
			return err
		}
		if release == nil {
			return fmt.Errorf("remote release not found")
		}

		// Load and serve the packages list. The client expects this to match what HandleInRelease digested
		pkgs, err := rh.packages.LoadPackages(ctx, release, arch)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := WriteControlFile(&buf, pkgs...); err != nil {
			return err
		}
		return compression.Compress(w, &buf)
	})
}

// func (h *Handler) HandlePool(w http.ResponseWriter, r *http.Request) {
// 	h.RepositoryHandler(w, r, "debian.HandlePool", func(ctx context.Context, vars map[string]string, rh *repositoryHandler) error {
// 		path := vars["path"]
// 		url := rh.packages.BaseURL() + path
// 		r = r.WithContext(ctx)
// 		http.Redirect(w, r, url, http.StatusMovedPermanently)
// 		return nil
// 	})
// }

type repositoryHandler struct {
	pk       *packet.PrivateKey
	release  ReleaseLoader
	packages PackagesLoader
}

func newRepositoryHandler(args registry.HandlerArgs, cfg *RepositoryConfig) (*repositoryHandler, error) {
	// Load the private signing key
	key, err := readKey(cfg)
	if err != nil {
		return nil, err
	}

	// Start with a package source:
	var release ReleaseLoader
	var packages PackagesLoader
	if upCfg := cfg.Source.Upstream; upCfg != nil {
		var rpl *RemotePackagesLoader
		release, rpl, err = NewRemoteLoader(args.Tracer, args.Client, args.ByteStorage, *cfg.Source.Upstream)
		packages = rpl
		if err != nil {
			return nil, err
		}
		defer func() {
			rpl.releases = release
		}()
	} else if ghCfg := cfg.Source.GitHub; ghCfg != nil {
		release = &FixedReleaseLoader{release: ghCfg.Release}
		// packages = NewGitHubPackagesLoader(args.Tracer, args.Client, *cfg.Source.GitHub)
	} else {
		return nil, fmt.Errorf("no source specified")
	}
	if err != nil {
		return nil, err
	}

	// Apply the policies to filter packages from the source:
	// pkgFilter, err := filter.CueConfigToPredicate[Package](filepath.Join(cfgDir, "debian", "policies"), cfg.Policies)
	// if err != nil {
	// 	return nil, fmt.Errorf("parsing policies: %w", err)
	// }
	// rekor, err := signature.NewRekorFinder(client)
	// if err != nil {
	// 	return nil, fmt.Errorf("creating rekor: %w", err)
	// }

	// packages = NewFilteredPackageLoader(tracer, packages, *rekor, pkgFilter)

	return &repositoryHandler{
		pk:       key[0].PrivateKey,
		release:  release,
		packages: packages,
	}, nil
}

func readKey(cfg *RepositoryConfig) (openpgp.EntityList, error) {
	if cfg.KeyPath == "" {
		return nil, fmt.Errorf("missing key")
	}
	keyIn, err := os.Open(cfg.KeyPath)
	if err != nil {
		return nil, err
	}
	defer keyIn.Close()
	key, err := openpgp.ReadArmoredKeyRing(keyIn)
	if err != nil {
		return nil, fmt.Errorf("decoding key: %w", err)
	}
	return key, nil
}
