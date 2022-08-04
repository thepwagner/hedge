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
	Signature  *TestSignature
}

type TestSignature struct {
	KeyFingerprint string
}

func (p TestPackage) GetName() string     { return p.Name }
func (p TestPackage) GetDeprecated() bool { return p.Deprecated }

type TestPackageVersion struct {
	Deprecated bool
}

func (v TestPackageVersion) GetDeprecated() bool { return v.Deprecated }

func TestAnyOf(t *testing.T) {
	preds := filter.AnyOf(
		filter.MatchesName[TestPackage]("foo"),
		filter.MatchesDeprecated[TestPackage](true),
	)

	ctx := context.Background()
	ok, err := preds(ctx, TestPackage{Name: "foo", Deprecated: true})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestFilterSlice(t *testing.T) {
	pred := filter.MatchesDeprecated[TestPackageVersion](true)

	ctx := context.Background()
	result, err := filter.FilterSlice(ctx, pred, TestPackageVersion{Deprecated: true}, TestPackageVersion{Deprecated: false})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.True(t, result[0].Deprecated)
}
