package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Server struct {
		Port          int
		DashboardPort int `yaml:"dashboard_port"`
		Aliases       map[string]string
	}
	Scenarios map[string]struct {
		Rule   string         `yaml:"rule"`
		Params ScenarioParams `yaml:"params"`
		Action ScenarioAction `yaml:"action"`
	}
	Actioners map[string]ActionerConfig
	Notifier  struct {
		Slack struct {
			WebhookURL  string `yaml:"webhook_url"`
			CallbackURL string `yaml:"callback_url"`
		}
	}
}

type ScenarioParams struct {
	TriggerCount  int `yaml:"trigger_count"`
	TriggerWindow int `yaml:"trigger_window"`
	UnblockAfter  int `yaml:"unblock_after"`
}

type ScenarioAction struct {
	Actioners []string       `yaml:"actioners"`
	Notifier  NotifierConfig `yaml:"notifier"`
}

type NotifierConfig struct {
	Enabled bool   `yaml:"enabled"`
	Name    string `yaml:"name"`
	Timeout int    `yaml:"timeout"`
}

type ActionerConfig struct {
	ProjectID       string `yaml:"project_id"`
	BucketName      string `yaml:"bucket_name"`
	LogCount        int    `yaml:"log_count"`
	CredentialsFile string `yaml:"credentials_file"`
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
