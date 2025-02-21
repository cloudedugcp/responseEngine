package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration.
type Config struct {
	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
}

// Load configuration from a YAML file.
func Load(configPath string) (Config, error) {
	var cfg Config
	cfg.Server.Addr = "0.0.0.0:8080"

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}
