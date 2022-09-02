package debian_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry/debian"
)

func TestPackageParser_PackageFromDeb(t *testing.T) {
	f, err := os.Open("testdata/testpkg_1.2.3_amd64.deb")
	require.NoError(t, err)
	defer f.Close()

	parser := debian.NewPackageParser(observability.NoopTracer)

	pkg, err := parser.PackageFromDeb(context.Background(), f)
	require.NoError(t, err)
	assert.Equal(t, "testpkg", pkg.Name)
	assert.Equal(t, "1.2.3", pkg.Version)
	assert.Equal(t, "pwagner", pkg.Maintainer)
	assert.Equal(t, "hansel virtual package", pkg.Description)
	assert.Equal(t, "optional", pkg.Priority)
	assert.Equal(t, "amd64", pkg.Architecture)
}
