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
	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry"
	"github.com/thepwagner/hedge/pkg/registry/base"
	"github.com/thepwagner/hedge/proto/hedge/v1"
	"go.opentelemetry.io/otel/trace"
)

// Handler implements https://wiki.debian.org/DebianRepository/Format
type Handler struct {
	tracer trace.Tracer
	repos  map[string]*repositoryHandler

	releaseLoader  cached.Function[LoadReleaseArgs, *hedge.DebianRelease]
	packagesLoader cached.Function[LoadPackagesArgs, *hedge.DebianPackages]
}

type repositoryHandler struct {
	pk          *packet.PrivateKey
	releaseArgs LoadReleaseArgs
}

func NewHandler(base *base.CachedMux, tracer trace.Tracer, cache cached.ByteStorage, client *http.Client, cfg registry.EcosystemConfig) (*Handler, error) {
	h := &Handler{
		tracer: tracer,
		repos:  map[string]*repositoryHandler{},
	}
	for repo, repoCfg := range cfg.Repositories {
		debCfg := repoCfg.(*RepositoryConfig)

		key, err := readKey(debCfg)
		if err != nil {
			return nil, fmt.Errorf("reading key for %s: %w", repo, err)
		}

		var releaseArgs LoadReleaseArgs
		if debCfg.Source.Upstream != nil {
			releaseArgs.MirrorURL = debCfg.Source.Upstream.URL
			releaseArgs.Dist = debCfg.Source.Upstream.Release
			releaseArgs.Architectures = debCfg.Source.Upstream.Architectures
			releaseArgs.Components = debCfg.Source.Upstream.Components
			releaseArgs.SigningKey = debCfg.Source.Upstream.Key
		}

		h.repos[repo] = &repositoryHandler{
			pk:          key[0].PrivateKey,
			releaseArgs: releaseArgs,
		}
	}

	cachedFetch := cached.Wrap(cached.WithPrefix[string, []byte]("debian_urls", cache), cached.URLFetcher(client))
	repo := NewRemoteRepository(tracer, cachedFetch)
	h.releaseLoader = observability.TracedFunc(tracer, "debian.LoadRelease", cached.Wrap(cached.WithPrefix[string, []byte]("debian_releases", cache), repo.LoadRelease, cached.AsProtoBuf[LoadReleaseArgs, *hedge.DebianRelease]()))
	h.packagesLoader = observability.TracedFunc(tracer, "debian.LoadPackages", cached.Wrap(cached.WithPrefix[string, []byte]("debian_packages", cache), repo.LoadPackages, cached.AsProtoBuf[LoadPackagesArgs, *hedge.DebianPackages]()))

	base.Register("/debian/dists/{repository}/InRelease", 0, h.HandleInRelease)
	base.Register("/debian/dists/{repository}/main/binary-{arch}/Packages{compression:(?:|.xz|.gz)}", 0, h.HandlePackages)
	// r.HandleFunc("/debian/dists/{repository}/pool/{path:.*}", h.HandlePool)
	return h, nil
}

func (h Handler) HandleInRelease(ctx context.Context, req base.HttpRequest) (*hedge.HttpResponse, error) {
	rh, ok := h.repos[req.PathVars["repository"]]
	if !ok {
		return &hedge.HttpResponse{
			StatusCode: http.StatusNotFound,
		}, nil
	}

	// Load the release metadata:
	release, err := h.releaseLoader(ctx, rh.releaseArgs)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, fmt.Errorf("remote release not found")
	}

	// The Release file contains hashes of all Packages files, so we need to load them:
	packages := map[Architecture][]*hedge.DebianPackage{}
	for _, a := range release.Architectures {
		arch := Architecture(a)
		pkgs, err := h.packagesLoader(ctx, LoadPackagesArgs{
			Release:      release,
			Architecture: arch,
		})
		if err != nil {
			return nil, err
		}
		packages[arch] = pkgs.Packages
	}

	// Write the signed InRelease file:
	ctx, span := h.tracer.Start(ctx, "debian.clearSign")
	defer span.End()
	var buf bytes.Buffer
	enc, err := clearsign.Encode(&buf, rh.pk, nil)
	if err != nil {
		return nil, err
	}

	if err := WriteReleaseFile(ctx, release, packages, enc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	if _, err = fmt.Fprintln(&buf); err != nil {
		return nil, err
	}

	return &hedge.HttpResponse{
		Body: buf.Bytes(),
	}, nil
}

func (h Handler) HandlePackages(ctx context.Context, req base.HttpRequest) (*hedge.HttpResponse, error) {
	rh, ok := h.repos[req.PathVars["repository"]]
	if !ok {
		return &hedge.HttpResponse{
			StatusCode: http.StatusNotFound,
		}, nil
	}
	arch := Architecture(req.PathVars["arch"])
	compression := CompressionFromExtension(req.PathVars["compression"])

	// Load the release metadata:
	release, err := h.releaseLoader(ctx, rh.releaseArgs)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, fmt.Errorf("remote release not found")
	}

	// Load and serve the packages list. The client expects this to match what HandleInRelease digested
	pkgs, err := h.packagesLoader(ctx, LoadPackagesArgs{
		Release:      release,
		Architecture: arch,
	})
	if err != nil {
		return nil, err
	}

	graphs := make([]Paragraph, 0, len(pkgs.Packages))
	for _, pkg := range pkgs.Packages {
		graphs = append(graphs, ParagraphFromPackage(pkg))
	}

	var buf bytes.Buffer
	if err := WriteControlFile(&buf, graphs...); err != nil {
		return nil, err
	}

	var compressed bytes.Buffer
	if err := compression.Compress(&compressed, &buf); err != nil {
		return nil, err
	}
	return &hedge.HttpResponse{
		Body: compressed.Bytes(),
	}, nil
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
