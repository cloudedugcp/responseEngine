package config

import (
	"os"

	"github.com/cloudedugcp/responseEngine/internal/types"
	"gopkg.in/yaml.v3"
)

// Config представляє конфігурацію додатку
type Config struct {
	Server    ServerConfig              `yaml:"server"`
	Scenarios []ScenarioConfig          `yaml:"scenarios"`
	Actioners map[string]ActionerConfig `yaml:"actioners"`
	Notifiers map[string]NotifierConfig `yaml:"notifiers"`
}

// ServerConfig представляє конфігурацію сервера
type ServerConfig struct {
	ListenPort string            `yaml:"port"`
	Aliases    map[string]string `yaml:"aliases"`
}

// ScenarioConfig представляє конфігурацію сценарію
type ScenarioConfig struct {
	Name       string `yaml:"name"`
	FalcoRule  string `yaml:"falco_rule"`
	Conditions struct {
		TriggerCount int    `yaml:"trigger_count"`
		TimeWindow   string `yaml:"time_window"`
	} `yaml:"conditions"`
	Actioners []ActionerConfig `yaml:"actioners"`
	Notify    NotifyConfig     `yaml:"notify"`
}

// ActionerConfig представляє конфігурацію діяча
type ActionerConfig struct {
	Name   string                 `yaml:"name"`
	Type   string                 `yaml:"type,omitempty"`
	Params map[string]interface{} `yaml:"params"`
}

// NotifyConfig представляє конфігурацію нотифікацій
type NotifyConfig struct {
	Enabled bool   `yaml:"enabled"`
	Type    string `yaml:"type"`
}

// NotifierConfig представляє конфігурацію нотифікатора
type NotifierConfig struct {
	WebhookURL   string `yaml:"webhook_url"`
	CallbackPath string `yaml:"callback_path"`
}

// LoadConfig завантажує конфігурацію з файлу
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

// GetActioners повертає список діячів для сценарію
func (sc *ScenarioConfig) GetActioners() []types.Actioner {
	var acts []types.Actioner
	for _, act := range sc.Actioners {
		acts = append(acts, &dummyActioner{name: act.Name})
	}
	return acts
}

// dummyActioner використовується як заглушка для передачі імені діяча
type dummyActioner struct {
	name string
}

func (d *dummyActioner) Execute(event types.Event, params map[string]interface{}) error {
	return nil
}

func (d *dummyActioner) Name() string {
	return d.name
}
