package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	tracer trace.Tracer
	dists  map[string]PackageLoader
}

type PackageLoader interface {
	GetPackage(ctx context.Context, pkg string) (*Package, error)
}

func NewHandler(tracer trace.Tracer, client *http.Client, repos map[string]*RepositoryConfig) (*Handler, error) {
	dists := make(map[string]PackageLoader, len(repos))
	for name, cfg := range repos {
		dl, err := newDistLoader(tracer, client, cfg)
		if err != nil {
			return nil, err
		}
		dists[name] = dl
	}

	return &Handler{
		tracer: tracer,
		dists:  dists,
	}, nil
}

func (h *Handler) Register(r *mux.Router) {
	r.HandleFunc("/npm/{dist}/{package}", h.GetPackageDetails)
}

func (h *Handler) GetPackageDetails(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "debian.HandleInRelease")
	defer span.End()
	vars := mux.Vars(r)
	distName := vars["dist"]
	pkgName := vars["package"]
	span.SetAttributes(attribute.String("dist", distName), attribute.String("package", pkgName))

	distLoader, ok := h.dists[distName]
	if !ok {
		span.SetStatus(codes.Error, "dist not found")
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}

	pkg, err := distLoader.GetPackage(ctx, pkgName)
	if err != nil {
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(pkg); err != nil {
		span.RecordError(err)
	}
}

func newDistLoader(tracer trace.Tracer, client *http.Client, cfg *RepositoryConfig) (PackageLoader, error) {
	var loader PackageLoader
	if upCfg := cfg.Source.Upstream; upCfg != nil {
		loader = NewRemoteLoader(tracer, client, cfg.Source.Upstream.URL)
	} else {
		return nil, fmt.Errorf("no package sources")
	}

	return NewPackageFilter(tracer, loader, cfg.Filters...)
}
