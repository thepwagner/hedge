package server

import (
	"fmt"
	"path/filepath"

	"github.com/thepwagner/hedge/debian"
)

type Config struct {
	Addr   string
	Debian []debian.RepositoryConfig
}

func LoadConfig(dir string) (*Config, error) {
	var cfg Config

	cfg.Addr = ":8080"

	debs, err := debian.LoadConfig(filepath.Join(dir, "debian"))
	if err != nil {
		return nil, fmt.Errorf("loading debian config: %w", err)
	}
	cfg.Debian = debs

	return &cfg, nil
}
