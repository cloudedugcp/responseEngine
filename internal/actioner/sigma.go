package actioner

import (
	"fmt"
	"log"

	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/types"
)

// SigmaActioner реалізує обробку Sigma-подій
type SigmaActioner struct {
	db *db.Database
}

// NewSigmaActioner створює новий SigmaActioner із конфігурацією
func NewSigmaActioner(cfg config.ActionerConfig, db *db.Database) (*SigmaActioner, error) {
	// Тут можна додати обробку параметрів із cfg.Params, якщо потрібно
	return &SigmaActioner{db: db}, nil
}

// Execute виконує дію для Sigma-події
func (sa *SigmaActioner) Execute(event types.Event, params map[string]interface{}) error {
	log.Printf("Executing Sigma action for IP: %s, Rule: %s", event.IP, event.RuleName)
	if err := sa.db.LogEvent(event.IP, event.RuleName, event.Log, event.Timestamp); err != nil {
		return fmt.Errorf("failed to log Sigma event: %v", err)
	}
	return nil
}

// Name повертає ім’я діяча
func (sa *SigmaActioner) Name() string {
	return "sigma"
}
