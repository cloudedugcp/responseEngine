package models

type Event struct {
	IP string `json:"ip"`
	// Інші поля від Falco
}

type BlockRecord struct {
	ID            int
	IP            string
	BlockedAt     int64
	UnblockAfter  int64
	BlockCount    int
	TriggerCount  int
	LastEventTime int64 // Додано для trigger_window
}
