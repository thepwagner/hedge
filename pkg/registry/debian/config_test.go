package debian_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/config"
	"github.com/thepwagner/hedge/pkg/registry/debian"
)

func TestLoadConfig(t *testing.T) {
	cfgs, err := config.LoadConfig[*debian.RepositoryConfig]("testdata/config")
	require.NoError(t, err)
	assert.Len(t, cfgs, 1)

	cfg := cfgs["rafal"]
	assert.Equal(t, "rafal", cfg.Name())
	assert.Equal(t, "my-private-key.gpg", cfg.KeyPath)

	require.NotNil(t, cfg.Source.Upstream)
	assert.Equal(t, "https://debian.mirror.rafal.ca/debian/", cfg.Source.Upstream.URL)
	assert.Equal(t, []string{"all", "amd64", "arm64"}, cfg.Source.Upstream.Architectures)
}
