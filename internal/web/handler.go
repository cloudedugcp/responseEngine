package web

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/db"
)

type BlockRecordWithStatus struct {
	ID            int
	IP            string
	Status        string
	BlockedAt     string // Змінено з int64 на string для форматування
	UnblockAfter  string // Змінено з int64 на string для форматування
	BlockCount    int
	TriggerCount  int
	LastEventTime string // Змінено з int64 на string для форматування
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

	currentTime := time.Now().Unix()
	var recordsWithStatus []BlockRecordWithStatus
	for _, record := range records {
		status := "Not Blocked"
		if record.BlockedAt > 0 && record.UnblockAfter > currentTime {
			status = "Blocked"
		}

		// Форматуємо час у зрозумілому вигляді
		blockedAt := "N/A"
		if record.BlockedAt > 0 {
			blockedAt = time.Unix(record.BlockedAt, 0).Format("2006-01-02 15:04:05")
		}
		unblockAfter := "N/A"
		if record.UnblockAfter > 0 {
			unblockAfter = time.Unix(record.UnblockAfter, 0).Format("2006-01-02 15:04:05")
		}
		lastEventTime := "N/A"
		if record.LastEventTime > 0 {
			lastEventTime = time.Unix(record.LastEventTime, 0).Format("2006-01-02 15:04:05")
		}

		recordsWithStatus = append(recordsWithStatus, BlockRecordWithStatus{
			ID:            record.ID,
			IP:            record.IP,
			Status:        status,
			BlockedAt:     blockedAt,
			UnblockAfter:  unblockAfter,
			BlockCount:    record.BlockCount,
			TriggerCount:  record.TriggerCount,
			LastEventTime: lastEventTime,
		})
	}

	tmpl, err := template.ParseFiles("internal/web/templates/dashboard.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, recordsWithStatus)
}
