package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

func TracedFunc[K any, V any](tracer trace.Tracer, spanName string, wrapped func(context.Context, K) (V, error)) func(context.Context, K) (V, error) {
	return func(ctx context.Context, k K) (V, error) {
		ctx, span := tracer.Start(ctx, spanName)
		defer span.End()
		v, err := wrapped(ctx, k)
		if err != nil {
			return v, CaptureError(span, err)
		}
		return v, nil
	}
}
