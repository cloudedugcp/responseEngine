package web

import (
	"html/template"
	"net/http"

	"github.com/cloudedugcp/responseEngine/internal/db"
)

// DashboardHandler - обробник для веб-інтерфейсу
func DashboardHandler(database *db.Database) http.HandlerFunc {
	tmpl := template.Must(template.ParseFiles("templates/dashboard.html"))
	return func(w http.ResponseWriter, r *http.Request) {
		actions, err := database.GetActions()
		if err != nil {
			http.Error(w, "Failed to load actions", http.StatusInternalServerError)
			return
		}

		data := struct {
			Actions []db.ActionLog
		}{Actions: actions}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
		}
	}
}
