package observability

import (
	"context"

	"github.com/go-logr/logr"
)

type ctxKey string

var ctxKeyRequestID = ctxKey("request_id")

const logFieldRequestID = "request_id"

func Logger(ctx context.Context, log logr.Logger) logr.Logger {
	if reqID, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return log.WithValues(logFieldRequestID, reqID)
	}
	return log
}
