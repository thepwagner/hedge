package npm_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/registry/npm"
)

func TestParsePackage(t *testing.T) {
	in, err := os.Open("testdata/package-stable.json")
	require.NoError(t, err)
	defer in.Close()

	p, err := npm.ParsePackage(in)
	require.NoError(t, err)
	assert.Equal(t, "stable", p.Name)
	assert.Equal(t, "0.1.8", p.LatestVersion())
	assert.Len(t, p.Versions, 9)

	expectedVersions := map[string]struct {
		deprecated bool
	}{
		"0.1.0": {deprecated: true},
		"0.1.1": {deprecated: true},
		"0.1.2": {deprecated: true},
		"0.1.3": {deprecated: true},
		"0.1.4": {deprecated: true},
		"0.1.5": {deprecated: true},
		"0.1.6": {deprecated: true},
		"0.1.7": {deprecated: true},
		"0.1.8": {deprecated: true},
	}

	for _, v := range p.Versions {
		expectations, ok := expectedVersions[v.Version]
		require.Truef(t, ok, "expected version not found: %s", v.Version)
		assert.Equal(t, expectations.deprecated, v.GetDeprecated(), v.Version)
	}
}
