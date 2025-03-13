package actioner

import (
	"fmt"
	"log"

	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/types"
)

type SigmaActioner struct {
	db *db.Database
}

func NewSigmaActioner(cfg config.ActionerConfig, db *db.Database) (*SigmaActioner, error) {
	return &SigmaActioner{db: db}, nil
}

func (sa *SigmaActioner) Execute(event types.Event, params map[string]interface{}) error {
	log.Printf("Executing Sigma action for IP: %s, Rule: %s", event.IP, event.RuleName)
	if err := sa.db.LogEvent(event.IP, event.RuleName, event.Log, event.Timestamp); err != nil {
		return fmt.Errorf("failed to log Sigma event: %v", err)
	}
	return nil
}

func (sa *SigmaActioner) Name() string {
	return "sigma"
}
