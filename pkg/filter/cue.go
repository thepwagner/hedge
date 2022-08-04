package filter

import (
	"context"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/encoding/gocode/gocodec"
)

func MatchesCue[T any](entrypoints ...string) (Predicate[T], error) {
	ctx := cuecontext.New()
	instances := load.Instances(entrypoints, nil)

	var values []cue.Value
	for _, i := range instances {
		if err := i.Err; err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", i.BuildFiles[0].Filename, err)
		}

		val := ctx.BuildInstance(i)
		if err := val.Err(); err != nil {
			return nil, err
		}
		values = append(values, val)
	}
	codec := gocodec.New((*cue.Runtime)(ctx), nil)

	return func(ctx context.Context, pkg T) (bool, error) {
		for _, val := range values {
			if err := codec.Validate(val, pkg); err != nil {
				var errs errors.Error
				if errors.As(err, &errs) {
					return false, nil
				}

				return false, err
			}
			fmt.Printf("passed %+v\n", val)
		}
		return true, nil
	}, nil
}
