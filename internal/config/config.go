package config

import (
	"github.com/cloudedugcp/responseEngine/internal/actioner" // Імпорт для ActionerConfig
	"github.com/cloudedugcp/responseEngine/internal/scenario"
	"github.com/spf13/viper"
)

// Config - структура конфігурації
type Config struct {
	Server    ServerConfig                       `mapstructure:"server"`
	Scenarios []Scenario                         `mapstructure:"scenarios"`
	Actioners map[string]actioner.ActionerConfig `mapstructure:"actioners"` // Використовуємо actioner.ActionerConfig
}

type ServerConfig struct {
	ListenPort string            `mapstructure:"port"`
	Aliases    map[string]string `mapstructure:"aliases"`
}

type Scenario struct {
	Name       string                       `mapstructure:"name"`
	FalcoRule  string                       `mapstructure:"falco_rule"`
	Conditions *scenario.ScenarioConditions `mapstructure:"conditions"`
	Actioners  []ScenarioActioner           `mapstructure:"actioners"`
}

type ScenarioActioner struct {
	Name   string                 `mapstructure:"name"`
	Params map[string]interface{} `mapstructure:"params"`
}

// LoadConfig - завантажує конфігурацію з файлу
func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
