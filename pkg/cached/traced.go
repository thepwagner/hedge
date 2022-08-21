package cached

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Traced[K comparable, V any] struct {
	tracer trace.Tracer
	cache  Cache[K, V]
}

var _ Cache[string, int] = (*Traced[string, int])(nil)

func WithTracer[K comparable, V any](tracer trace.Tracer, cache Cache[K, V]) Traced[K, V] {
	return Traced[K, V]{
		tracer: tracer,
		cache:  cache,
	}
}

func (c Traced[K, V]) Get(ctx context.Context, key K) (*V, error) {
	ctx, span := c.tracer.Start(ctx, "cache.Get", trace.WithAttributes(attrCacheKey(key)))
	defer span.End()

	v, err := c.cache.Get(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attrCacheHit(v != nil))

	return v, err
}

func (c Traced[K, V]) Set(ctx context.Context, key K, value V, ttl time.Duration) error {
	ctx, span := c.tracer.Start(ctx, "cache.Set", trace.WithAttributes(attrCacheKey(key), attrCacheTTL(ctx, ttl)))
	defer span.End()

	err := c.cache.Set(ctx, key, value, ttl)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func attrCacheKey(key any) attribute.KeyValue {
	return attribute.KeyValue{
		Key:   "cache_key",
		Value: attribute.StringValue(fmt.Sprintf("%v", key)),
	}
}

func attrCacheHit(hit bool) attribute.KeyValue {
	return attribute.KeyValue{
		Key:   "cache_hit",
		Value: attribute.BoolValue(hit),
	}
}

func attrCacheTTL(ctx context.Context, ttl time.Duration) attribute.KeyValue {
	return attribute.KeyValue{
		Key:   "cache_ttl",
		Value: attribute.Int64Value(int64(durationFromContext(ctx, ttl).Seconds())),
	}
}
