package scenario

import (
	"log"
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
	if conditions.TimeWindow == 0 {
		log.Printf("Warning: TimeWindow is 0, conditions will always fail")
	}
	count, err := db.CountEvents(event.IP, conditions.TimeWindow)
	if err != nil {
		log.Printf("Error counting events for IP %s: %v", event.IP, err)
		return false
	}
	log.Printf("IP %s: %d events in last %d seconds (required: %d)", event.IP, count, conditions.TimeWindow/time.Second, conditions.TriggerCount)
	return count >= conditions.TriggerCount
}
