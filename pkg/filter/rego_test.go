package filter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/filter"
)

func TestMatchesRego(t *testing.T) {
	ctx := context.Background()
	cases := map[string]struct {
		expectedSuccess []TestPackage
		expectedFail    []TestPackage
	}{
		"testdata/name_foo.rego": {
			expectedSuccess: []TestPackage{
				{Name: "foo"},
				{Name: "foo", Signature: &TestSignature{KeyFingerprint: "key1"}},
				{Name: "foo", Signature: &TestSignature{KeyFingerprint: "key2"}},
			},
			expectedFail: []TestPackage{
				{Name: "bar"},                   // name mismatch
				{Name: "foo", Deprecated: true}, // deprecated
				{Name: "foo", Signature: &TestSignature{KeyFingerprint: "key3"}},            // fingerprint mismatch
				{Name: "foo", Signature: &TestSignature{KeyFingerprint: "compromised-key"}}, // valid signer, but denied key
			},
		},
	}

	for fn, tc := range cases {
		t.Run(fn, func(t *testing.T) {
			pred, err := filter.MatchesRego[TestPackage](ctx, fn)
			require.NoError(t, err)

			for i, pkg := range tc.expectedSuccess {
				actual, err := pred(ctx, pkg)
				require.NoError(t, err, "case %d", i)
				assert.True(t, actual, "case %d", i)
			}

			for i, pkg := range tc.expectedFail {
				actual, err := pred(ctx, pkg)
				require.NoError(t, err, "failed case %d", i)
				assert.False(t, actual, "failed case %d", i)
			}
		})
	}
}
