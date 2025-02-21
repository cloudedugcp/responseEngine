package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration.
type Config struct {
	Server struct {
		Addr     string `yaml:"addr"`
		Fail2Ban bool   `yaml:"fail2ban_enabled"`
		BanTime  int    `yaml:"ban_time"`
		LogPath  string `yaml:"log_path"`
		JailName string `yaml:"jail_name"`
	} `yaml:"server"`
}

// Load configuration from a YAML file.
func Load(configPath string) (Config, error) {
	var cfg Config
	//default values
	cfg.Server.Addr = "0.0.0.0:8080"
	cfg.Server.Fail2Ban = true
	cfg.Server.BanTime = 3600
	cfg.Server.LogPath = "/var/log/falco.log"
	cfg.Server.JailName = "falco"

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
