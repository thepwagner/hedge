package filter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/filter"
)

type TestPackage struct {
	Name       string
	Deprecated bool
}

func (p TestPackage) GetName() string     { return p.Name }
func (p TestPackage) GetDeprecated() bool { return p.Deprecated }

func TestPredicates(t *testing.T) {
	preds := filter.AnyOf(
		filter.MatchesName[TestPackage]("foo"),
		filter.MatchesDeprecated[TestPackage](true),
	)

	ctx := context.Background()
	ok, err := preds(ctx, TestPackage{Name: "foo", Deprecated: true})
	require.NoError(t, err)
	assert.True(t, ok)
}
