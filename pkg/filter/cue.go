package filter

import (
	"context"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
)

func MatchesCue(entrypoints ...string) (Predicate[[]byte], error) {
	ctx := cuecontext.New()
	instances := load.Instances(entrypoints, nil)

	var values []cue.Value
	for _, i := range instances {
		if err := i.Err; err != nil {
			return nil, fmt.Errorf("failed to load: %w", err)
		}

		val := ctx.BuildInstance(i)
		if err := val.Err(); err != nil {
			return nil, err
		}
		values = append(values, val)

		// Don't allow policies without any constraints, they unintentionally allow everything
		if s, err := val.Struct(); err != nil {
			return nil, err
		} else if s.Len() == 0 {
			return nil, fmt.Errorf("no constraints found in %s", i.BuildFiles[0].Filename)
		}
	}

	// Don't allow predicates without policies, they unintentionally allow everything
	if len(values) == 0 {
		return nil, fmt.Errorf("no values loaded")
	}

	return func(ctx context.Context, b []byte) (bool, error) {
		for _, val := range values {
			err := val.Unify(val.Context().CompileBytes(b)).Err()
			if err != nil {
				var errs errors.Error
				if errors.As(err, &errs) {
					return false, nil
				}
				return false, err
			}
		}
		return true, nil
	}, nil
}
