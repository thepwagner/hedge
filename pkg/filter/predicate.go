package filter

import "context"

type Predicate[T any] func(context.Context, T) (bool, error)

func AnyOf[P Predicate[T], T any](preds ...P) Predicate[T] {
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

func FilterSlice[P Predicate[T], T any](ctx context.Context, pred P, in ...T) ([]T, error) {
	result := make([]T, 0, len(in))
	for _, t := range in {
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
