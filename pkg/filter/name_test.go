package filter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/filter"
)

func TestMatchesName(t *testing.T) {
	pkg := TestPackage{Name: "foo"}
	cases := map[string]bool{
		"foo":           true,
		"bar":           false,
		"anything else": false,
	}

	ctx := context.Background()
	for in, expected := range cases {
		ok, err := filter.MatchesName[TestPackage](in)(ctx, pkg)
		require.NoError(t, err)
		assert.Equal(t, expected, ok)
	}
}

func TestMatchesPattern(t *testing.T) {
	pkg := TestPackage{Name: "foo"}
	cases := map[string]bool{
		"foo":  true,
		"bar":  false,
		"^f.*": true,
	}

	ctx := context.Background()
	for in, expected := range cases {
		f, err := filter.MatchesPattern[TestPackage](in)
		require.NoError(t, err)
		ok, err := f(ctx, pkg)
		require.NoError(t, err)
		assert.Equal(t, expected, ok)
	}
}
