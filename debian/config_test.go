package debian_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/debian"
	"github.com/thepwagner/hedge/pkg/config"
)

func TestLoadConfig(t *testing.T) {
	cfgs, err := config.LoadConfig[debian.RepositoryConfig]("testdata/config/rafal")
	require.NoError(t, err)
	assert.Len(t, cfgs, 1)

	cfg := cfgs["stable"]
	assert.Equal(t, "", cfg.Name)
	assert.Equal(t, "debian/dists/bullseye.gpg", cfg.Key)

	require.NotNil(t, cfg.Source.Upstream)
	assert.Equal(t, "https://debian.mirror.rafal.ca/debian/", cfg.Source.Upstream.URL)
	assert.Equal(t, []string{"all", "amd64", "arm64"}, cfg.Source.Upstream.Architectures)
}
