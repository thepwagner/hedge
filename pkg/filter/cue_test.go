package filter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/filter"
)

func TestMatchesCue(t *testing.T) {
	ctx := context.Background()
	cases := map[string]struct {
		expectedSuccess []TestPackage
		expectedFail    []TestPackage
	}{
		"testdata/name_foo.cue": {
			expectedSuccess: []TestPackage{
				{Name: "foo"},
				{Name: "foo", Signature: &TestSignature{KeyFingerprint: "key1"}},
				{Name: "foo", Signature: &TestSignature{KeyFingerprint: "key2"}},
			},
			expectedFail: []TestPackage{
				{Name: "bar"},                   // name mismatch
				{Name: "foo", Deprecated: true}, // deprecated
				{Name: "foo", Signature: &TestSignature{KeyFingerprint: "key3"}}, // fingerprint mismatch
			},
		},
		"testdata/package_group.cue": {
			expectedSuccess: []TestPackage{
				{Name: "test"},
				{Name: "test-common"},
				{Name: "dep1"},
				{Name: "dep2"},
			},
			expectedFail: []TestPackage{
				{Name: "test-tube"}, // does not match regex
				{Name: "dep3"},      // not in the dep list
			},
		},
		"testdata/tags.cue": {
			expectedSuccess: []TestPackage{
				{Name: "test", Tags: []string{"tag1", "tag2", "tag3", "tag4", "tag5"}},
				{Name: "test", Tags: []string{"tag2", "tag4"}},
			},
			expectedFail: []TestPackage{
				{Name: "test"},                         // no tags
				{Name: "test", Tags: []string{"tag1"}}, // no relevant tags
				{Name: "test", Tags: []string{"tag2"}}, // not both tags
				{Name: "test", Tags: []string{"tag4"}}, // not both tags
			},
		},
	}

	for fn, tc := range cases {
		t.Run(fn, func(t *testing.T) {
			pred, err := filter.MatchesCue(fn)
			require.NoError(t, err)

			for _, pkg := range tc.expectedSuccess {
				b, err := json.Marshal(pkg)
				require.NoError(t, err)
				actual, err := pred(ctx, b)
				require.NoError(t, err)
				assert.True(t, actual)
			}

			for i, pkg := range tc.expectedFail {
				b, err := json.Marshal(pkg)
				require.NoError(t, err)
				actual, err := pred(ctx, b)
				require.NoError(t, err)
				assert.False(t, actual, "failed case %d", i)
			}
		})
	}
}

func TestMatchesCue_ParseEarly(t *testing.T) {
	_, err := filter.MatchesCue("testdata/invalid.cue")
	assert.Error(t, err)

	_, err = filter.MatchesCue("testdata/empty.cue")
	assert.Error(t, err)
}
