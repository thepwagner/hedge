package debian

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SourceConfig defines where packages are stored.
type SourceConfig struct {
	Upstream *UpstreamConfig
	// TODO: maybe you can list debs from a dir/bucket?
}

// UpstreamConfig is a Debian repository acting as a source.
type UpstreamConfig struct {
	URL           string
	Key           string
	Release       string
	Architectures []string
	Components    []string
}

type RepositoryConfig struct {
	Source SourceConfig `yaml:"source"`

	Name string
	Key  string
}

func LoadConfig(dir string) ([]RepositoryConfig, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing config directory: %w", err)
	}

	configs := make([]RepositoryConfig, 0, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		ext := path.Ext(f.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		in, err := os.Open(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, err
		}

		var config RepositoryConfig
		if err := yaml.NewDecoder(in).Decode(&config); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("decoding config file: %w", err)
		}
		if err := in.Close(); err != nil {
			return nil, err
		}

		if config.Name == "" {
			config.Name = f.Name()[:len(f.Name())-len(ext)]
		}

		configs = append(configs, config)
	}
	return configs, nil
}
