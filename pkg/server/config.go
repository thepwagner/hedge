package server

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/thepwagner/hedge/pkg/registry"
	"gopkg.in/yaml.v3"
)

// Config is the configuration for the server.
type Config struct {
	Addr           string
	ConfigDir      string
	TracerEndpoint string
	RedisURL       string

	Ecosystems map[registry.Ecosystem]registry.EcosystemConfig
}

const (
	repositoriesDir = "repositories"
	policiesDir     = "policies"
)

// LoadConfig loads server configuration from the given directory.
func LoadConfig(dir string) (*Config, error) {
	cfg := Config{
		Addr:           ":8080",
		ConfigDir:      dir,
		TracerEndpoint: "http://riker.pwagner.net:14268/api/traces",
		RedisURL:       "localhost:6379",
		Ecosystems:     make(map[registry.Ecosystem]registry.EcosystemConfig, len(ecosystems)),
	}

	for _, ep := range ecosystems {
		eco := ep.Ecosystem()
		ecoBase := filepath.Join(dir, string(eco))

		repos, err := loadRepositories(ep, filepath.Join(ecoBase, repositoriesDir))
		if err != nil {
			return nil, fmt.Errorf("loading %s repositories: %w", eco, err)
		}

		policies, err := loadPolicyFiles(repos, filepath.Join(ecoBase, policiesDir))
		if err != nil {
			return nil, fmt.Errorf("loading %s policies: %w", eco, err)
		}

		cfg.Ecosystems[eco] = registry.EcosystemConfig{
			Repositories: repos,
			Policies:     policies,
		}
	}

	return &cfg, nil
}

func loadRepositories(ep registry.EcosystemProvider, repoDir string) (map[string]registry.RepositoryConfig, error) {
	files, err := os.ReadDir(repoDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing config directory: %w", err)
	}

	configs := make(map[string]registry.RepositoryConfig, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		ext := filepath.Ext(f.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		in, err := os.Open(filepath.Join(repoDir, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("opening file: %w", err)
		}

		config := ep.BlankRepositoryConfig()
		if err := yaml.NewDecoder(in).Decode(config); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("decoding config file: %w", err)
		}
		if err := in.Close(); err != nil {
			return nil, fmt.Errorf("closing file: %w", err)
		}

		name := config.Name()
		if name == "" {
			name = f.Name()[:len(f.Name())-len(ext)]
			config.SetName(name)
		}

		if _, existing := configs[name]; existing {
			return nil, fmt.Errorf("duplicate repository: %q", name)
		}
		configs[name] = config
	}

	return configs, nil
}

func loadPolicyFiles(repos map[string]registry.RepositoryConfig, policyDir string) (map[string]string, error) {
	policies := make(map[string]string)
	for _, repoCfg := range repos {
		for _, policyName := range repoCfg.PolicyNames() {
			b, err := os.ReadFile(filepath.Join(policyDir, policyName))
			if err != nil {
				return nil, err
			}
			policies[policyName] = string(b)
		}
	}
	return policies, nil
}
