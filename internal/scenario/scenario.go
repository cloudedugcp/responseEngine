package scenario

import (
	"time"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/cloudedugcp/responseEngine/internal/db"
)

// ScenarioConditions - умови спрацювання
type ScenarioConditions struct {
	TriggerCount int           `mapstructure:"trigger_count"`
	TimeWindow   time.Duration `mapstructure:"time_window"`
}

// ShouldTrigger - перевіряє, чи потрібно спрацьовувати діячу
func ShouldTrigger(conditions ScenarioConditions, event actioner.Event, db *db.Database) bool {
	count, err := db.CountEvents(event.IP, conditions.TimeWindow)
	if err != nil {
		return false
	}
	return count >= conditions.TriggerCount
}
