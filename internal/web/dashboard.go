package web

import (
	"html/template"
	"log"
	"net/http"

	"github.com/cloudedugcp/responseEngine/internal/db"
)

// DashboardHandler - обробник для веб-інтерфейсу
func DashboardHandler(database *db.Database) http.HandlerFunc {
	tmpl := template.Must(template.ParseFiles("internal/web/templates/dashboard.html")) // Оновлений шлях
	return func(w http.ResponseWriter, r *http.Request) {
		actions, err := database.GetActions()
		if err != nil {
			log.Printf("Failed to load actions from database: %v", err)
			http.Error(w, "Failed to load actions", http.StatusInternalServerError)
			return
		}

		data := struct {
			Actions []db.ActionLog
		}{Actions: actions}

		if err := tmpl.Execute(w, data); err != nil {
			log.Printf("Failed to render dashboard template: %v", err)
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
		}
	}
}
