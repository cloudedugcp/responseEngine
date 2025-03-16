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
		Rule   string         `yaml:"rule"`
		Params ScenarioParams `yaml:"params"`
		Action ScenarioAction `yaml:"action"`
	} `yaml:"scenarios"`
	Actioners map[string]ActionerConfig `yaml:"actioners"`
	Notifier  struct {
		Slack struct {
			WebhookURL  string `yaml:"webhook_url"`
			CallbackURL string `yaml:"callback_url"`
			BotToken    string `yaml:"bot_token"`
			Channel     string `yaml:"channel"`
		} `yaml:"slack"`
	} `yaml:"notifier"`
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
