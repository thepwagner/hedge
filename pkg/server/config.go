package server

import (
	"fmt"
	"path/filepath"

	"github.com/thepwagner/hedge/pkg/config"
	"github.com/thepwagner/hedge/pkg/registry/debian"
	"github.com/thepwagner/hedge/pkg/registry/npm"
	"github.com/thepwagner/hedge/pkg/registry/oci"
)

type Config struct {
	Addr           string
	ConfigDir      string
	TracerEndpoint string

	Debian config.NamedConfigs[*debian.RepositoryConfig]
	NPM    config.NamedConfigs[*npm.RepositoryConfig]
	OCI    config.NamedConfigs[*oci.RepositoryConfig]
}

const (
	repositoriesDir = "repositories"
	policiesDir     = "policies"
)

func LoadConfig(dir string) (*Config, error) {
	cfg := Config{
		Addr:           ":8080",
		ConfigDir:      dir,
		TracerEndpoint: "http://riker.pwagner.net:14268/api/traces",
	}

	var err error
	cfg.Debian, err = config.LoadConfig[*debian.RepositoryConfig](filepath.Join(dir, "debian", repositoriesDir))
	if err != nil {
		return nil, fmt.Errorf("loading debian config: %w", err)
	}
	cfg.NPM, err = config.LoadConfig[*npm.RepositoryConfig](filepath.Join(dir, "npm", repositoriesDir))
	if err != nil {
		return nil, fmt.Errorf("loading npm config: %w", err)
	}
	cfg.OCI, err = config.LoadConfig[*oci.RepositoryConfig](filepath.Join(dir, "oci", repositoriesDir))
	if err != nil {
		return nil, fmt.Errorf("loading oci config: %w", err)
	}

	return &cfg, nil
}
