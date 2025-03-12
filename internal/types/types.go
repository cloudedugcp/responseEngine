package types

import "time"

// Actioner визначає інтерфейс для діячів
type Actioner interface {
	Execute(event Event, params map[string]interface{}) error
	Name() string
}

// Event представляє подію Falco
type Event struct {
	IP        string    `json:"ip"`
	RuleName  string    `json:"rule"`
	Log       string    `json:"log"`
	Timestamp time.Time `json:"timestamp"`
}
