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
	dists  map[string]distHandler
}

func NewHandler(tp trace.TracerProvider, repos map[string]*RepositoryConfig) (*Handler, error) {
	dists := make(map[string]distHandler, len(repos))
	for name, cfg := range repos {
		dh, err := newDistHandler(tp, cfg)
		if err != nil {
			return nil, err
		}
		dists[name] = *dh
	}

	return &Handler{
		tracer: tp.Tracer("hedge"),
		dists:  dists,
	}, nil
}

func (h *Handler) Register(r *mux.Router) {
	r.HandleFunc("/npm/{dist}/{package}", h.GetPackageDetails)
}

func (h *Handler) GetPackageDetails(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "npm.GetPackageDetails")
	defer span.End()
	r = r.WithContext(ctx)

	distName := mux.Vars(r)["dist"]
	dist, ok := h.dists[distName]
	if !ok {
		http.Error(w, "dist not found", http.StatusNotFound)
		return
	}
	dist.GetPackageDetails(w, r)
}

type PackageLoader interface {
	GetPackage(ctx context.Context, pkg string) (*Package, error)
}

type distHandler struct {
	tracer trace.Tracer
	loader PackageLoader
}

func newDistHandler(tp trace.TracerProvider, cfg *RepositoryConfig) (*distHandler, error) {
	var loader PackageLoader
	if upCfg := cfg.Source.Upstream; upCfg != nil {
		loader = NewRemoteLoader(tp, cfg.Source.Upstream.URL)
	} else {
		return nil, fmt.Errorf("no package sources")
	}

	tracer := tp.Tracer("hedge")
	loader, err := NewPackageFilter(tracer, loader, cfg.Filters...)
	if err != nil {
		return nil, err
	}

	return &distHandler{
		tracer: tracer,
		loader: loader,
	}, nil
}

func (dh *distHandler) GetPackageDetails(w http.ResponseWriter, r *http.Request) {
	ctx, span := dh.tracer.Start(r.Context(), "GetPackageDetails")
	defer span.End()

	pkgName := mux.Vars(r)["package"]
	span.SetAttributes(attribute.String("package", pkgName))

	pkg, err := dh.loader.GetPackage(ctx, pkgName)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(pkg)
}
