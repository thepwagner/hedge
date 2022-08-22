package cached

import (
	"context"
	"fmt"
	"time"
)

// WithPrefix applys a prefix to every entry of a string-keyed cache.
func WithPrefix[V any](prefix string, cache Cache[string, V]) Prefixed[V] {
	return Prefixed[V]{prefix: prefix, cache: cache}
}

type Prefixed[V any] struct {
	prefix string
	cache  Cache[string, V]
}

var _ Cache[string, int] = (*Prefixed[int])(nil)

func (c Prefixed[V]) Get(ctx context.Context, key string) (*V, error) {
	return c.cache.Get(ctx, keyWithPrefix(c.prefix, key))
}

func (c Prefixed[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	return c.cache.Set(ctx, keyWithPrefix(c.prefix, key), value, ttl)
}

func keyWithPrefix(prefix, key string) string {
	return fmt.Sprintf("%s:%s", prefix, key)
}
