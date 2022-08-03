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

// func (p Predicates[T])
