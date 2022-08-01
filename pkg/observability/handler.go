package observability

import (
	"context"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-uuid"
)

type LoggingHandler struct {
	log     logr.Logger
	wrapped http.Handler
}

func NewLoggingHandler(log logr.Logger, wrapped http.Handler) *LoggingHandler {
	return &LoggingHandler{
		log:     log,
		wrapped: wrapped,
	}
}

func (h *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	requestID, _ := uuid.GenerateUUID()
	log := h.log.V(1).WithValues(logFieldRequestID, requestID)
	log.Info("request received", "method", r.Method, "url", r.URL.String())

	ctx := r.Context()
	r = r.WithContext(context.WithValue(ctx, ctxKeyRequestID, requestID))
	h.wrapped.ServeHTTP(w, r)

	dur := time.Since(t)
	log.Info("response sent", "method", r.Method, "url", r.URL.String(), "dur", dur.Truncate(time.Millisecond).Milliseconds())
}
