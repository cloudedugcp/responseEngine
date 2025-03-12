package notifier

import (
	"net/http"

	"github.com/cloudedugcp/responseEngine/internal/types"
)

// Notifier визначає інтерфейс для нотифікаторів
type Notifier interface {
	Notify(event types.Event, scenario string, actioners []types.Actioner) (string, error)
	HandleCallback(w http.ResponseWriter, r *http.Request, actioners map[string]types.Actioner)
}
