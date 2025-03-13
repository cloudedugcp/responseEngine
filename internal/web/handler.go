package web

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/db"
)

// BlockRecordWithStatus - структура з доданим полем Status
type BlockRecordWithStatus struct {
	ID            int
	IP            string
	Status        string
	BlockedAt     int64
	UnblockAfter  int64
	BlockCount    int
	TriggerCount  int
	LastEventTime int64
}

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

	// Перетворюємо записи в формат із статусом
	currentTime := time.Now().Unix()
	var recordsWithStatus []BlockRecordWithStatus
	for _, record := range records {
		status := "Not Blocked"
		if record.BlockedAt > 0 && record.UnblockAfter > currentTime {
			status = "Blocked"
		}
		recordsWithStatus = append(recordsWithStatus, BlockRecordWithStatus{
			ID:            record.ID,
			IP:            record.IP,
			Status:        status,
			BlockedAt:     record.BlockedAt,
			UnblockAfter:  record.UnblockAfter,
			BlockCount:    record.BlockCount,
			TriggerCount:  record.TriggerCount,
			LastEventTime: record.LastEventTime,
		})
	}

	tmpl, err := template.ParseFiles("internal/web/templates/dashboard.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, recordsWithStatus)
}
