package debian

import (
	"github.com/thepwagner/hedge/pkg/filter"
	"github.com/thepwagner/hedge/pkg/registry"
)

type RepositoryConfig struct {
	Source   SourceConfig  `yaml:"source"`
	Policies filter.Config `yaml:"policies"`

	NameRaw string `yaml:"name"`
	KeyPath string `yaml:"keyPath"`
}

var _ registry.RepositoryConfig = (*RepositoryConfig)(nil)

func (c RepositoryConfig) Name() string                { return c.NameRaw }
func (c *RepositoryConfig) SetName(name string)        { c.NameRaw = name }
func (c RepositoryConfig) FilterConfig() filter.Config { return c.Policies }

// SourceConfig defines where packages are stored.
type SourceConfig struct {
	Upstream *UpstreamConfig
	GitHub   *GitHubConfig
}

// UpstreamConfig is a Debian repository acting as a source.
type UpstreamConfig struct {
	URL           string
	Key           string
	Release       string
	Architectures []string
	Components    []string
}

// GitHubConfig polls GitHub releases for packages.
type GitHubConfig struct {
	Release      Release
	Repositories []string
}
