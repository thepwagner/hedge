package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hedge/pkg/config"
)

type TestConfig struct {
	NameRaw string `yaml:"name"`
	IntAttr int    `yaml:"intAttr"`
}

func (c *TestConfig) Name() string        { return c.NameRaw }
func (c *TestConfig) SetName(name string) { c.NameRaw = name }

func TestLoadConfig(t *testing.T) {
	expectedIntAttr := map[string]int{
		"foo":     1234,
		"douglas": 42,
	}

	cfgs, err := config.LoadConfig[*TestConfig]("testdata")
	require.NoError(t, err)

	assert.Len(t, cfgs, len(expectedIntAttr))
	for name, expected := range expectedIntAttr {
		cfg := cfgs[name]
		assert.Equal(t, name, cfg.Name())
		assert.Equal(t, expected, cfg.IntAttr)
	}
}
