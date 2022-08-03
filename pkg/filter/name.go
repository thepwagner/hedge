package filter

import (
	"context"
	"fmt"
	"regexp"
)

type HasName interface {
	GetName() string
}

func MatchesName[T HasName](name string) Predicate[T] {
	return func(ctx context.Context, pkg T) (bool, error) {
		return pkg.GetName() == name, nil
	}
}

func MatchesPattern[T HasName](pattern string) (Predicate[T], error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compiling regex: %w", err)
	}
	return func(ctx context.Context, pkg T) (bool, error) {
		return re.MatchString(pkg.GetName()), nil
	}, nil
}
