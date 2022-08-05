package filter

import (
	"fmt"
	"path/filepath"
)

type CueConfig struct {
	AnyOf []string `yaml:"anyOf"`
}

func CueConfigToPredicate[T any](root string, cfg CueConfig) (Predicate[T], error) {
	var anyOf []Predicate[T]
	for _, s := range cfg.AnyOf {
		pred, err := MatchesCue[T](filepath.Join(root, s))
		if err != nil {
			return nil, fmt.Errorf("invalid cue %q: %w", s, err)
		}
		anyOf = append(anyOf, pred)
	}

	return AnyOf(anyOf...), nil
}
