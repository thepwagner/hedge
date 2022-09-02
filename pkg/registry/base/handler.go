package base

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/proto/hedge/v1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type HttpRequest struct {
	Path     string
	PathVars map[string]string
}

type Handler struct {
	Tracer trace.Tracer
	mux    *mux.Router
	cache  cached.Cache[string, []byte]
}

func NewHandler(tracer trace.Tracer, cache cached.ByteStorage) *Handler {
	return &Handler{
		Tracer: tracer,
		mux:    mux.NewRouter(),
		cache:  cache,
	}
}

func (h Handler) Register(path string, ttl time.Duration, handler cached.Function[HttpRequest, *hedge.HttpResponse]) {
	if ttl > 0 {
		cache := cached.WithPrefix(fmt.Sprintf("mux:%s", path), h.cache)
		handler = cached.Wrap(cache, handler, cached.AsProtoBuf[HttpRequest, *hedge.HttpResponse](), cached.WithTTL[HttpRequest, *hedge.HttpResponse](ttl))
	}

	h.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		ctx, span := h.Tracer.Start(r.Context(), path)
		defer span.End()
		vars := mux.Vars(r)
		for k, v := range vars {
			span.SetAttributes(attribute.String(fmt.Sprintf("mux.vars.%s", k), v))
		}

		res, err := handler(ctx, HttpRequest{
			Path:     path,
			PathVars: vars,
		})
		if err != nil {
			_ = observability.CaptureError(span, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if res.ContentType != "" {
			w.Header().Add("Content-Type", res.ContentType)
		}
		if res.StatusCode != 0 {
			w.WriteHeader(int(res.StatusCode))
		}
		_, _ = w.Write(res.Body)
	})
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}
