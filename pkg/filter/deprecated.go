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

// TODO: is this only a debian thing?
type HasPriority interface {
	GetPriority() string
}

func MatchesPriority[T HasPriority](priority string) Predicate[T] {
	return func(ctx context.Context, pkg T) (bool, error) {
		return pkg.GetPriority() == priority, nil
	}
}
