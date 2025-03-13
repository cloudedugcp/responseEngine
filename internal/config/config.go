package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Server struct {
		Port          int               `yaml:"port"`
		DashboardPort int               `yaml:"dashboard_port"`
		Aliases       map[string]string `yaml:"aliases"`
	} `yaml:"server"`
	Scenarios map[string]struct {
		WaitTimeout   int      `yaml:"wait_timeout"`
		TriggerCount  int      `yaml:"trigger_count"`
		TriggerWindow int      `yaml:"trigger_window"`
		UnblockAfter  int      `yaml:"unblock_after"`
		Actioners     []string `yaml:"actioners"`
	} `yaml:"scenarios"`
	Actioners map[string]ActionerConfig `yaml:"actioners"`
	Notifier  struct {
		Slack struct {
			WebhookURL  string `yaml:"webhook_url"`
			CallbackURL string `yaml:"callback_url"`
		} `yaml:"slack"`
	} `yaml:"notifier"`
}

type ActionerConfig struct {
	ProjectID       string `yaml:"project_id"`
	CredentialsFile string `yaml:"credentials_file"`
	BucketName      string `yaml:"bucket_name,omitempty"`
	LogCount        int    `yaml:"log_count,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	return &cfg, err
}
