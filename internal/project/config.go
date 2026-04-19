// Package project loads and validates a user's leaf site configuration.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config mirrors the leaf: section of config.yml. Fields match LeafConfig in
// zephyrus-leaf-core so the PHP side sees identical values.
type Config struct {
	Name          string            `yaml:"name"`
	Version       string            `yaml:"version"`
	Description   string            `yaml:"description"`
	GithubURL     string            `yaml:"github_url"`
	Author        string            `yaml:"author"`
	AuthorURL     string            `yaml:"author_url"`
	License       string            `yaml:"license"`
	ContentPath   string            `yaml:"content_path"`
	OutputPath    string            `yaml:"output_path"`
	BaseURL       string            `yaml:"base_url"`
	ProductionURL string            `yaml:"production_url"`
	Sections      map[string]string `yaml:"sections"`
}

type yamlRoot struct {
	Leaf Config `yaml:"leaf"`
}

// Load reads config.yml from projectRoot, applies defaults, and returns the
// parsed config.
func Load(projectRoot string) (*Config, error) {
	path := filepath.Join(projectRoot, "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config.yml not found at %s", path)
		}
		return nil, fmt.Errorf("read config.yml: %w", err)
	}
	var root yamlRoot
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse config.yml: %w", err)
	}
	cfg := &root.Leaf
	cfg.applyDefaults()
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.ContentPath == "" {
		c.ContentPath = "content"
	}
	if c.OutputPath == "" {
		c.OutputPath = "dist"
	}
}

// Validate returns an error describing any missing required fields.
func (c *Config) Validate() error {
	if c.Name == "" {
		return errors.New("leaf.name is required")
	}
	return nil
}
