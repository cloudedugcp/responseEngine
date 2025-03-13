package web

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/cloudedugcp/responseEngine/internal/db"
)

type Dashboard struct {
	db *db.SQLiteDB
}

func StartDashboard(port int, db *db.SQLiteDB) {
	d := &Dashboard{db}
	mux := http.NewServeMux()
	mux.HandleFunc("/", d.dashboardHandler)
	log.Printf("Dashboard starting on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), mux))
}

func (d *Dashboard) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	records, err := d.db.GetAllRecords()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("internal/web/templates/dashboard.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, records)
}
