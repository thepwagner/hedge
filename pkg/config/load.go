package config

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

type Named interface {
	Name() string
	SetName(string)
}

func LoadConfig[T Named](dir string) (map[string]T, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing config directory: %w", err)
	}

	configs := make(map[string]T, len(files))
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

		var config T
		if err := yaml.NewDecoder(in).Decode(&config); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("decoding config file: %w", err)
		}
		if err := in.Close(); err != nil {
			return nil, err
		}

		name := config.Name()
		if name == "" {
			name = f.Name()[:len(f.Name())-len(ext)]
			config.SetName(name)
		}

		if _, existing := configs[name]; existing {
			return nil, fmt.Errorf("duplicate repository")
		}
		configs[name] = config
	}

	return configs, nil
}
