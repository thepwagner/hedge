package filter

import "context"

type HasDeprecated interface {
	GetDeprecated() bool
}

func MatchesDeprecated[T HasDeprecated](deprecated bool) Predicate[T] {
	return func(ctx context.Context, pkg T) (bool, error) {
		return pkg.GetDeprecated() == deprecated, nil
	}
}
