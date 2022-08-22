package cached

import (
	"context"
	"fmt"
	"time"
)

// WithPrefix applys a prefix to every entry of a string-keyed cache.
func WithPrefix[K ~string, V any](prefix string, cache Cache[K, V]) Prefixed[K, V] {
	return Prefixed[K, V]{prefix: prefix, cache: cache}
}

type Prefixed[K ~string, V any] struct {
	prefix string
	cache  Cache[K, V]
}

var _ Cache[string, int] = (*Prefixed[string, int])(nil)

func (c Prefixed[K, V]) Get(ctx context.Context, key K) (*V, error) {
	return c.cache.Get(ctx, c.withPrefix(key))
}

func (c Prefixed[K, V]) Set(ctx context.Context, key K, value V, ttl time.Duration) error {
	return c.cache.Set(ctx, c.withPrefix(key), value, ttl)
}

func (c Prefixed[K, V]) withPrefix(key K) K {
	return K(fmt.Sprintf("%s:%s", c.prefix, key))
}
