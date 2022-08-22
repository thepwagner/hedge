package cached

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// Race is a parallel version of Cached that returns the first result available.
func Race[K comparable, V any](tracer trace.Tracer, race string, entrants map[string]Function[K, V]) Function[K, V] {
	return func(ctx context.Context, arg K) (V, error) {
		ctx, span := tracer.Start(ctx, fmt.Sprintf("cached.Racer.%s", race))
		defer span.End()

		res := make(chan V, len(entrants))
		eg, ctx := errgroup.WithContext(ctx)
		// eg.SetLimit(1)
		for k, f := range entrants {
			k := k
			f := f
			eg.Go(func() error {
				ctx, span := tracer.Start(ctx, fmt.Sprintf("cached.Racer.%s.%s", race, k))
				defer span.End()
				v, err := f(ctx, arg)
				if err != nil {
					return err
				}
				res <- v
				return nil
			})
		}

		var zero V
		var ret V
		select {
		case ret = <-res:
		case <-ctx.Done():
			return zero, ctx.Err()
		}

		if err := eg.Wait(); err != nil {
			return zero, err
		}
		return ret, nil
	}
}
