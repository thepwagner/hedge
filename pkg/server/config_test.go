package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/registry/debian"
	"github.com/thepwagner/hedge/pkg/server"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := server.LoadConfig("testdata/config")
	require.NoError(t, err)

	debCfg, ok := cfg.Ecosystems[debian.Ecosystem]
	require.True(t, ok)
	assert.Len(t, debCfg.Repositories, 2)

	bullseyeCfg, ok := debCfg.Repositories["bullseye"].(*debian.RepositoryConfig)
	require.True(t, ok)
	assert.Equal(t, "https://debian.mirror.rafal.ca/debian/", bullseyeCfg.Source.Upstream.URL)

	assert.Contains(t, debCfg.Policies["nethack.cue"], "Games")
}
