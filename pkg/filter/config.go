package filter

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

type Config struct {
	AnyOf []string `yaml:"anyOf"`
}

func (c Config) PolicyNames() []string {
	return c.AnyOf
}

func CueConfigToPredicate[T any](root string, cfg Config) (Predicate[T], error) {
	var anyOf []Predicate[[]byte]
	for _, s := range cfg.AnyOf {
		pred, err := MatchesCue(filepath.Join(root, s))
		if err != nil {
			return nil, fmt.Errorf("invalid cue %q: %w", s, err)
		}
		anyOf = append(anyOf, pred)
	}
	pred := AnyOf(anyOf...)

	return func(ctx context.Context, t T) (bool, error) {
		b, err := json.Marshal(t)
		if err != nil {
			return false, fmt.Errorf("json error: %w", err)
		}
		fmt.Println(string(b))
		return pred(ctx, b)
	}, nil
}
