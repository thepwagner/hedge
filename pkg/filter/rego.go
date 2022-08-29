package filter

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/rego"
)

func MatchesRego[T any](ctx context.Context, entrypoints ...string) (Predicate[T], error) {
	// TODO: what if i want to allow _OR_ deny?
	r := rego.New(
		rego.Query("data.hedge.allow"),
		rego.Load(entrypoints, nil),
	)

	query, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("preparing query: %w", err)
	}

	return func(ctx context.Context, value T) (bool, error) {
		rs, err := query.Eval(ctx, rego.EvalInput(value))
		if err != nil {
			return false, err
		}

		return rs.Allowed(), nil
	}, nil
}
