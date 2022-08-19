package cache

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("not found")

type Storage interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type Cache[V any] interface {
	Get(ctx context.Context, key string) (V, error)
	Set(ctx context.Context, key string, value V, ttl time.Duration) error
}

type PrefixCache[V any] struct {
	prefix string
	cache  Cache[V]
}

func Prefix[V any](prefix string, cache Cache[V]) PrefixCache[V] {
	return PrefixCache[V]{prefix: prefix, cache: cache}
}

func (c PrefixCache[V]) Get(ctx context.Context, key string) (V, error) {
	return c.cache.Get(ctx, fmt.Sprintf("%s:%s", c.prefix, key))
}

func (c PrefixCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	return c.cache.Set(ctx, fmt.Sprintf("%s:%s", c.prefix, key), value, ttl)
}
