package cached

import (
	"context"
	"time"
)

type Function[K any, V any] func(context.Context, K) (V, error)

func Cached[K any, V any](cache Cache[K, V], ttl time.Duration, wrapped Function[K, V]) Function[K, V] {
	return func(ctx context.Context, arg K) (V, error) {
		var zero V
		if cached, err := cache.Get(ctx, arg); err != nil {
			return zero, err
		} else if cached != nil {
			return *cached, nil
		}

		calculated, err := wrapped(ctx, arg)
		if err != nil {
			return zero, err
		}
		if err := cache.Set(ctx, arg, calculated, ttl); err != nil {
			return zero, err
		}
		return calculated, nil
	}
}
