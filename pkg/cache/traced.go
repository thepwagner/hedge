package cache

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type TracedCache[V any] struct {
	tracer trace.Tracer
	cache  Cache[V]
}

var _ Cache[string] = (*TracedCache[string])(nil)

func NewTracedCache[V any](tracer trace.Tracer, cache Cache[V]) TracedCache[V] {
	return TracedCache[V]{
		tracer: tracer,
		cache:  cache,
	}
}

var (
	attrCacheHit = attribute.Key("cache_hit")
	attrCacheKey = attribute.Key("cache_key")
)

func (c TracedCache[V]) Get(ctx context.Context, key string) (V, error) {
	ctx, span := c.tracer.Start(ctx, "cache.Get", trace.WithAttributes(attrCacheKey.String(key)))
	defer span.End()

	v, err := c.cache.Get(ctx, key)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attrCacheHit.Bool(false))
		}
	} else {
		span.SetAttributes(attrCacheHit.Bool(true))
	}

	return v, err
}

func (c TracedCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	ctx, span := c.tracer.Start(ctx, "cache.Set", trace.WithAttributes(attrCacheKey.String(key)))
	defer span.End()

	err := c.cache.Set(ctx, key, value, ttl)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}
