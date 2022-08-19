package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Named has a mutable name field.
type Named interface {
	Name() string
	SetName(string)
}

// NamedConfigs are unique Nameds.
type NamedConfigs[T Named] map[string]T

// LoadConfig loads NamedConfigs from files in a directory of YAML.
// If no name is set from the YAML contents, it is defaulted to the basename of the file.
func LoadConfig[T Named](dir string) (NamedConfigs[T], error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing config directory: %w", err)
	}

	configs := make(NamedConfigs[T], len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		ext := filepath.Ext(f.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		in, err := os.Open(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("opening file: %w", err)
		}

		var config T
		if err := yaml.NewDecoder(in).Decode(&config); err != nil {
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
