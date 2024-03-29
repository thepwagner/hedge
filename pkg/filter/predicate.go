package filter

import (
	"context"
	"fmt"
)

type Predicate[T any] func(context.Context, T) (bool, error)

func AnyOf[T any](preds ...Predicate[T]) Predicate[T] {
	return func(ctx context.Context, t T) (bool, error) {
		for _, pred := range preds {
			ok, err := pred(ctx, t)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}
}

func FilterSlice[T any](ctx context.Context, pred Predicate[T], in ...T) ([]T, error) {
	var result []T
	for i, t := range in {
		if i%100 == 0 {
			fmt.Println(i)
		}
		ok, err := pred(ctx, t)
		if err != nil {
			return nil, err
		}
		if ok {
			result = append(result, t)
		}
	}
	return result, nil
}

func FilterMap[P Predicate[T], T any, K comparable](ctx context.Context, pred P, in map[K]T) (map[K]T, error) {
	result := make(map[K]T, len(in))
	for k, v := range in {
		ok, err := pred(ctx, v)
		if err != nil {
			return nil, err
		}
		if ok {
			result[k] = v
		}
	}
	return result, nil
}
