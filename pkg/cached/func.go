package cached

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

type Function[K comparable, V any] func(context.Context, K) (V, error)

func Cached[K comparable, V any](cache Cache[K, V], ttl time.Duration, wrapped Function[K, V]) Function[K, V] {
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

// Raced is a parallel version of Cached that returns the first result available.
func Raced[K comparable, V any](cache Cache[K, V], ttl time.Duration, wrapped Function[K, V]) Function[K, V] {
	return func(ctx context.Context, arg K) (V, error) {
		start := time.Now()
		res := make(chan V, 2)

		eg, ctx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			if cached, err := cache.Get(ctx, arg); err != nil {
				return err
			} else if cached != nil {
				res <- *cached
			}
			fmt.Println("cache", arg, time.Since(start).Truncate(time.Millisecond).Milliseconds())
			return nil
		})

		eg.Go(func() error {
			calculated, err := wrapped(ctx, arg)
			if err != nil {
				return err
			}
			res <- calculated
			fmt.Println("calculated", arg, time.Since(start).Truncate(time.Millisecond).Milliseconds())

			if err := cache.Set(ctx, arg, calculated, ttl); err != nil {
				return err
			}
			return nil
		})

		r := <-res
		if err := eg.Wait(); err != nil {
			var zero V
			return zero, err
		}

		return r, nil
	}
}
