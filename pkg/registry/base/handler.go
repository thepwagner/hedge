package base

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Handler[R any] struct {
	Tracer       trace.Tracer
	Repositories map[string]R
}

func NewHandler[R any, C registry.RepositoryConfig](args registry.HandlerArgs, convert func(C) (R, error)) (*Handler[R], error) {
	h := &Handler[R]{
		Tracer:       args.Tracer,
		Repositories: make(map[string]R, len(args.Ecosystem.Repositories)),
	}

	for name, repoCfg := range args.Ecosystem.Repositories {
		casted, ok := repoCfg.(C)
		if !ok {
			return nil, fmt.Errorf("repository %s is unexpected type", name)
		}
		repo, err := convert(casted)
		if err != nil {
			return nil, err
		}
		h.Repositories[name] = repo
	}
	return h, nil
}

func (h *Handler[R]) RepositoryHandler(w http.ResponseWriter, r *http.Request, spanName string, op func(context.Context, map[string]string, R) error) {
	ctx, span := h.Tracer.Start(r.Context(), spanName)
	defer span.End()

	vars := mux.Vars(r)
	repoName := vars["repository"]
	span.SetAttributes(observability.RepositoryName(repoName))
	for k, v := range vars {
		if k == "repository" {
			continue
		}
		span.SetAttributes(attribute.String(k, v))
	}

	rh, ok := h.Repositories[repoName]
	if !ok {
		span.SetStatus(codes.Error, "repository not found")
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	if err := op(ctx, vars, rh); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "error handling request")
		http.Error(w, "error handling request", http.StatusInternalServerError)
	}
}
