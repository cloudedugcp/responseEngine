package actioner

// Event - подія від Falco
type Event struct {
	IP       string `json:"ip"`
	RuleName string `json:"rule"`
	Log      string `json:"log,omitempty"` // Додаємо поле для логів, опціональне
}

// Actioner - інтерфейс для виконавців дій
type Actioner interface {
	Execute(event Event, params map[string]interface{}) error
	Name() string
}

// ActionerConfig - конфігурація діяча
type ActionerConfig struct {
	Type   string                 `mapstructure:"type"`
	Params map[string]interface{} `mapstructure:"params"`
}
