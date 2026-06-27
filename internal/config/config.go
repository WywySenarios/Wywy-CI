package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Config struct {
	Repos []RepoEntry `json:"repos"`
}

type RepoEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func Load(paths ...string) (*Config, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no config paths provided")
	}

	var loaded bool
	var merged Config
	seen := make(map[string]int)

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			log.Printf("warning: config file %q not found, skipping", path)
			continue
		}

		loaded = true

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config %s: %w", path, err)
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
		}

		for _, r := range cfg.Repos {
			if r.Name == "" || r.Path == "" {
				return nil, fmt.Errorf("repo entry with empty name or path in %s", path)
			}
		}

		if len(paths) == 1 {
			var filtered []RepoEntry
			for _, r := range cfg.Repos {
				if _, err := os.Stat(r.Path); err == nil {
					filtered = append(filtered, r)
				} else {
					log.Printf("warning: repo %q path %q does not exist, skipping", r.Name, r.Path)
				}
			}
			cfg.Repos = filtered
		}

		for _, r := range cfg.Repos {
			if idx, ok := seen[r.Name]; ok {
				merged.Repos[idx] = r
			} else {
				seen[r.Name] = len(merged.Repos)
				merged.Repos = append(merged.Repos, r)
			}
		}
	}

	if !loaded {
		return nil, fmt.Errorf("no config files found (tried: %v)", paths)
	}

	if merged.Repos == nil {
		merged.Repos = []RepoEntry{}
	}

	return &merged, nil
}
