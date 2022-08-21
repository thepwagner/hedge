package npm

import "github.com/thepwagner/hedge/pkg/filter"

type RepositoryConfig struct {
	Source   SourceConfig  `yaml:"source"`
	Policies filter.Config `yaml:"policies"`

	NameRaw string `yaml:"name"`
	Key     string
}

func (c RepositoryConfig) Name() string         { return c.NameRaw }
func (c *RepositoryConfig) SetName(name string) { c.NameRaw = name }

// SourceConfig defines where packages are stored.
type SourceConfig struct {
	Upstream *UpstreamConfig
}

// UpstreamConfig is an NPM repository acting as a source.
type UpstreamConfig struct {
	URL string
}
