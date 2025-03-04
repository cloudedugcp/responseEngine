package actioner

// Event - подія від Falco
type Event struct {
	IP       string `json:"ip"`
	RuleName string `json:"rule"`
}

// Actioner - інтерфейс для виконавців дій
type Actioner interface {
	Execute(event Event, params map[string]interface{}) error
	Name() string
}

// ActionerConfig - конфігурація діяча (тепер тут)
type ActionerConfig struct {
	Type   string                 `mapstructure:"type"`
	Params map[string]interface{} `mapstructure:"params"`
}
