package models

type Event struct {
	IP       string `json:"ip"`
	Rule     string
	Result   string
	Time     string
	SourceIP string
}

type BlockRecord struct {
	ID            int
	IP            string
	BlockedAt     int64
	UnblockAfter  int64
	BlockCount    int
	TriggerCount  int
	LastEventTime int64
	ActionTaken   bool
}
