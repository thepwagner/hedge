package server

import (
	"fmt"
	"path/filepath"

	"github.com/thepwagner/hedge/debian"
	"github.com/thepwagner/hedge/pkg/config"
	"github.com/thepwagner/hedge/pkg/npm"
)

type Config struct {
	Addr string

	Debian map[string]*debian.RepositoryConfig
	NPM    map[string]*npm.RepositoryConfig
}

func LoadConfig(dir string) (*Config, error) {
	var cfg Config
	cfg.Addr = ":8080"

	debs, err := config.LoadConfig[*debian.RepositoryConfig](filepath.Join(dir, "debian"))
	if err != nil {
		return nil, fmt.Errorf("loading debian config: %w", err)
	}
	cfg.Debian = debs

	npms, err := config.LoadConfig[*npm.RepositoryConfig](filepath.Join(dir, "npm"))
	if err != nil {
		return nil, fmt.Errorf("loading npm config: %w", err)
	}
	cfg.NPM = npms

	return &cfg, nil
}
