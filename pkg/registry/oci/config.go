package oci

type RepositoryConfig struct {
	NameRaw string `yaml:"name"`
}

func (c RepositoryConfig) Name() string         { return c.NameRaw }
func (c *RepositoryConfig) SetName(name string) { c.NameRaw = name }
