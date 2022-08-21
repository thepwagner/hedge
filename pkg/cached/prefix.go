package cached

import (
	"context"
	"fmt"
	"time"
)

type Prefixed[V any] struct {
	prefix string
	cache  Cache[string, V]
}

var _ Cache[string, int] = (*Prefixed[int])(nil)

func WithPrefix[V any](prefix string, cache Cache[string, V]) Prefixed[V] {
	return Prefixed[V]{prefix: prefix, cache: cache}
}

func (c Prefixed[V]) Get(ctx context.Context, key string) (*V, error) {
	return c.cache.Get(ctx, fmt.Sprintf("%s:%s", c.prefix, key))
}

func (c Prefixed[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	return c.cache.Set(ctx, fmt.Sprintf("%s:%s", c.prefix, key), value, ttl)
}
