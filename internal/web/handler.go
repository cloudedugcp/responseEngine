package web

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/scenario"
)

type BlockRecordWithStatus struct {
	ID            int
	IP            string
	Status        string
	BlockedAt     string
	UnblockAfter  string
	BlockCount    int
	TriggerCount  int
	LastEventTime string
}

type Dashboard struct {
	db       *db.SQLiteDB
	scenario *scenario.Manager
}

func StartDashboard(port int, db *db.SQLiteDB, mgr *scenario.Manager) {
	d := &Dashboard{db: db, scenario: mgr}
	mux := http.NewServeMux()
	mux.HandleFunc("/", d.dashboardHandler)
	mux.HandleFunc("/unblock", d.unblockHandler) // Новий обробник
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

func (d *Dashboard) unblockHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.FormValue("ip")
	if ip == "" {
		http.Error(w, "IP not provided", http.StatusBadRequest)
		return
	}

	log.Printf("Manual unblock requested for IP %s", ip)
	err := d.scenario.ManualUnblock(ip)
	if err != nil {
		log.Printf("Failed to manually unblock IP %s: %v", ip, err)
		http.Error(w, "Failed to unblock IP", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
