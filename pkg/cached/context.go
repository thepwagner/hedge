package cached

import (
	"context"
	"time"
)

type ctxKey string

const duration ctxKey = "duration"

func For(ctx context.Context, ttl time.Duration) context.Context {
	return context.WithValue(ctx, duration, ttl)
}

func durationFromContext(ctx context.Context, defaultTTL time.Duration) time.Duration {
	if d, ok := ctx.Value(duration).(time.Duration); ok {
		return d
	}
	return defaultTTL
}
