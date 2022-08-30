package filter

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/rego"
)

func MatchesRego[T any](ctx context.Context, entrypoints ...string) (Predicate[T], error) {
	allow, err := rego.New(
		rego.Query("data.hedge.allow"),
		rego.Load(entrypoints, nil),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("preparing allow query: %w", err)
	}

	deny, err := rego.New(
		rego.Query("data.hedge.deny"),
		rego.Load(entrypoints, nil),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("preparing deny query: %w", err)
	}

	return func(ctx context.Context, value T) (bool, error) {
		if rs, err := deny.Eval(ctx, rego.EvalInput(value)); err != nil {
			return false, fmt.Errorf("evaluating deny query: %w", err)
		} else if rs.Allowed() {
			return false, nil
		}

		rs, err := allow.Eval(ctx, rego.EvalInput(value))
		if err != nil {
			return false, err
		}
		return rs.Allowed(), nil
	}, nil
}
