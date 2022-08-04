package filter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/filter"
)

func TestMatchesCue(t *testing.T) {
	pred, err := filter.MatchesCue[TestPackage]("testdata/name_foo.cue")
	require.NoError(t, err)

	ctx := context.Background()
	cases := []struct {
		pkg      TestPackage
		expected bool
	}{
		{pkg: TestPackage{Name: "foo"}, expected: true},
		{pkg: TestPackage{Name: "bar"}, expected: false},
		{pkg: TestPackage{Name: "foo", Deprecated: true}, expected: false},
		{pkg: TestPackage{Name: "foo", Deprecated: true, Signature: &TestSignature{KeyFingerprint: "key1"}}, expected: false},
	}
	for _, tc := range cases {
		t.Run(tc.pkg.Name, func(t *testing.T) {
			actual, err := pred(ctx, tc.pkg)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestMatchesCue_ParseEarly(t *testing.T) {
	_, err := filter.MatchesCue[TestPackage]("testdata/invalid.cue")
	assert.EqualError(t, err, "expected label or ':', found 'IDENT' is")
}
