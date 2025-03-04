package web

import (
	"html/template"
	"net/http"

	"github.com/cloudedugcp/responseEngine/internal/db" // Коректний імпорт пакету db
)

// DashboardHandler - обробник для веб-інтерфейсу
func DashboardHandler(database *db.Database) http.HandlerFunc { // Змінено db на database для уникнення конфлікту імен
	tmpl := template.Must(template.ParseFiles("templates/dashboard.html"))
	return func(w http.ResponseWriter, r *http.Request) {
		actions, err := database.GetActions() // Використовуємо database замість db
		if err != nil {
			http.Error(w, "Failed to load actions", http.StatusInternalServerError)
			return
		}

		// Визначаємо структуру для передачі даних у шаблон
		data := struct {
			Actions []db.ActionLog // Коректний тип із пакету db
		}{Actions: actions}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
		}
	}
}
