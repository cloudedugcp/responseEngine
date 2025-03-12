package notifier

import (
	"net/http"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
)

type Notifier interface {
	Notify(event actioner.Event, scenario string, actioners []actioner.Actioner) (string, error)
	HandleCallback(w http.ResponseWriter, r *http.Request, actioners map[string]actioner.Actioner)
}
