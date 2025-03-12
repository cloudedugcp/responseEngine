package web

import (
	"html/template"
	"log"
	"net/http"

	"github.com/cloudedugcp/responseEngine/internal/db"
)

// DashboardHandler обробляє запит до дашборда
func DashboardHandler(database *db.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logs, err := database.GetActions()
		if err != nil {
			log.Printf("Failed to get actions: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Response Engine Dashboard</title>
    <style>
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid black; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>IP Logs</h1>
    <table>
        <tr>
            <th>IP</th>
            <th>Last Event</th>
            <th>Attempt Count</th>
            <th>Last Attempt Time</th>
            <th>Block Time</th>
            <th>Unblock Time</th>
            <th>Block Count</th>
            <th>Status</th>
        </tr>
        {{range .}}
        <tr>
            <td>{{.IP}}</td>
            <td>{{.LastEvent}}</td>
            <td>{{.AttemptCount}}</td>
            <td>{{.LastAttemptTime}}</td>
            <td>{{if .BlockTime}}{{.BlockTime}}{{else}}-{{end}}</td>
            <td>{{if .UnblockTime}}{{.UnblockTime}}{{else}}-{{end}}</td>
            <td>{{.BlockCount}}</td>
            <td>{{.Status}}</td>
        </tr>
        {{end}}
    </table>
</body>
</html>
`
		t, err := template.New("dashboard").Parse(tmpl)
		if err != nil {
			log.Printf("Failed to parse template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if err := t.Execute(w, logs); err != nil {
			log.Printf("Failed to execute template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}
