package web

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/db"
)

// DashboardData представляє дані для відображення в дашборді
type DashboardData struct {
	IP              string
	LastEvent       string
	AttemptCount    int
	LastAttemptTime time.Time
	BlockTime       time.Time
	UnblockTime     time.Time
	BlockCount      int
	Status          string
}

// DashboardHandler обробляє запит до дашборду
func DashboardHandler(database *db.Database) http.HandlerFunc {
	tmpl, err := template.ParseFiles("internal/web/templates/dashboard.html")
	if err != nil {
		log.Fatalf("Failed to parse dashboard template: %v", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := database.DB().Query(`
            SELECT 
                e.ip,
                e.rule_name AS last_event,
                (SELECT COUNT(*) FROM events WHERE ip = e.ip) AS attempt_count,
                MAX(e.timestamp) AS last_attempt_time,
                a.block_time,
                a.unblock_time,
                (SELECT COUNT(*) FROM actions WHERE ip = e.ip AND action = 'block') AS block_count,
                CASE 
                    WHEN a.unblock_time > CURRENT_TIMESTAMP THEN 'Blocked'
                    ELSE 'Unblocked'
                END AS status
            FROM events e
            LEFT JOIN actions a ON e.ip = a.ip AND a.action = 'block'
            GROUP BY e.ip, e.rule_name, a.block_time, a.unblock_time
        `)
		if err != nil {
			log.Printf("Failed to query dashboard data: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var data []DashboardData
		for rows.Next() {
			var d DashboardData
			var lastAttemptTimeStr string
			var blockTime, unblockTime sql.NullTime

			if err := rows.Scan(&d.IP, &d.LastEvent, &d.AttemptCount, &lastAttemptTimeStr, &blockTime, &unblockTime, &d.BlockCount, &d.Status); err != nil {
				log.Printf("Failed to scan dashboard row: %v", err)
				continue
			}

			// Парсимо рядок last_attempt_time у форматі RFC 3339
			if lastAttemptTimeStr != "" {
				d.LastAttemptTime, err = time.Parse(time.RFC3339, lastAttemptTimeStr)
				if err != nil {
					log.Printf("Failed to parse last_attempt_time '%s': %v", lastAttemptTimeStr, err)
					continue
				}
			}

			// Обробка block_time і unblock_time
			if blockTime.Valid {
				d.BlockTime = blockTime.Time
			}
			if unblockTime.Valid {
				d.UnblockTime = unblockTime.Time
			}

			data = append(data, d)
		}

		if err := tmpl.Execute(w, data); err != nil {
			log.Printf("Failed to render dashboard: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}
