package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config - структура конфігурації
type Config struct {
	Server    ServerConfig              `yaml:"server"`
	Scenarios []ScenarioConfig          `yaml:"scenarios"`
	Actioners map[string]ActionerConfig `yaml:"actioners"`
	Notifiers map[string]NotifierConfig `yaml:"notifiers"` // Новий розділ
}

// ServerConfig - конфігурація сервера
type ServerConfig struct {
	ListenPort string            `yaml:"port"`
	Aliases    map[string]string `yaml:"aliases"`
}

// ScenarioConfig - конфігурація сценарію
type ScenarioConfig struct {
	Name       string        `yaml:"name"`
	FalcoRule  string        `yaml:"falco_rule"`
	Conditions *Conditions   `yaml:"conditions"`
	Actioners  []ActionerRef `yaml:"actioners"`
	Notify     NotifyConfig  `yaml:"notify"` // Новий параметр
}

// ActionerRef - посилання на діяч у сценарії
type ActionerRef struct {
	Name   string                 `yaml:"name"`
	Params map[string]interface{} `yaml:"params"`
}

// ActionerConfig - конфігурація діяча
type ActionerConfig struct {
	Type   string                 `yaml:"type"`
	Params map[string]interface{} `yaml:"params"`
}

// NotifyConfig - конфігурація нотифікатора для сценарію
type NotifyConfig struct {
	Enabled bool   `yaml:"enabled"`
	Type    string `yaml:"type"`
}

// NotifierConfig - конфігурація нотифікатора
type NotifierConfig struct {
	WebhookURL   string `yaml:"webhook_url"`
	CallbackPath string `yaml:"callback_path"`
}

// Conditions - умови спрацьовування сценарію
type Conditions struct {
	TriggerCount int    `yaml:"trigger_count"`
	TimeWindow   string `yaml:"time_window"`
}

// LoadConfig - завантажує конфігурацію з файлу
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
