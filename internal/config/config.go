// Package config loads per-project defaults from .radiant.yaml (or
// .radiant.yml). Values are used as defaults when CLI flags are not set.
// The config file is optional — radiant works fine without it.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds project-level defaults that can be overridden by CLI flags.
type Config struct {
	// Model is the default LLM model ID.
	Model string `yaml:"model" json:"model"`

	// MaxIter is the default maximum iterations for loop start.
	MaxIter int `yaml:"max_iter" json:"max_iter"`

	// Profile is the budget profile: lean, standard, thorough.
	Profile string `yaml:"profile" json:"profile"`

	// WebhookURL is the default webhook URL for loop start and fleet dispatch.
	WebhookURL string `yaml:"webhook_url" json:"webhook_url"`

	// FleetConcurrency is the default --concurrency for fleet dispatch.
	FleetConcurrency int `yaml:"fleet_concurrency" json:"fleet_concurrency"`

	// FleetMaxRetries is the default --max-retries for fleet dispatch.
	FleetMaxRetries int `yaml:"fleet_max_retries" json:"fleet_max_retries"`

	// AutoRoute enables AutoRoute by default for loop start.
	AutoRoute bool `yaml:"auto_route" json:"auto_route"`
}

// Load reads .radiant.yaml (or .radiant.yml) from projectDir.
// Returns an empty Config if the file does not exist.
func Load(projectDir string) (*Config, error) {
	for _, name := range []string{".radiant.yaml", ".radiant.yml"} {
		path := filepath.Join(projectDir, name)
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		return &cfg, nil
	}
	return &Config{}, nil
}

// MarshalJSON implements json.Marshaler for use in `radiant doctor --json`.
func (c *Config) MarshalJSON() ([]byte, error) {
	type alias Config
	return json.Marshal((*alias)(c))
}
