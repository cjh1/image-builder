package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ImageConfig holds the image configuration loaded from YAML
type ImageConfig struct {
	Options        map[string]interface{} `yaml:"options"`
	Modules        []string               `yaml:"modules"`
	Packages       []string               `yaml:"packages"`
	PackageGroups  []string               `yaml:"package_groups"`
	RemovePackages []string               `yaml:"remove_packages"`
	Commands       []string               `yaml:"cmds"`
	CopyFiles      []map[string]string    `yaml:"copy_files"`
	Repositories   []Repository           `yaml:"repositories"`
}

// Repository represents a package repository configuration
type Repository struct {
	Alias    string `yaml:"alias"`
	URL      string `yaml:"url"`
	Priority int    `yaml:"priority,omitempty"`
	GPGKey   string `yaml:"gpg_key,omitempty"`
}

// LoadConfig loads and parses a YAML configuration file
func LoadConfig(yamlFile string) (*ImageConfig, error) {
	if !filepath.IsAbs(yamlFile) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
		yamlFile = filepath.Join(cwd, yamlFile)
	}

	if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", yamlFile)
	}

	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ImageConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Initialize maps/slices if they're nil
	if config.Options == nil {
		config.Options = make(map[string]interface{})
	}
	if config.Modules == nil {
		config.Modules = []string{}
	}
	if config.Packages == nil {
		config.Packages = []string{}
	}
	if config.PackageGroups == nil {
		config.PackageGroups = []string{}
	}
	if config.RemovePackages == nil {
		config.RemovePackages = []string{}
	}
	if config.Commands == nil {
		config.Commands = []string{}
	}
	if config.CopyFiles == nil {
		config.CopyFiles = []map[string]string{}
	}
	if config.Repositories == nil {
		config.Repositories = []Repository{}
	}

	return &config, nil
}
