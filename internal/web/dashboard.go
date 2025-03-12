package web

import (
	"html/template"
	"net/http"

	"github.com/cloudedugcp/responseEngine/internal/db"
)

func DashboardHandler(db *db.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := db.GetIPStats()
		if err != nil {
			http.Error(w, "Failed to fetch IP stats: "+err.Error(), http.StatusInternalServerError)
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
            <h1>IP Block Dashboard</h1>
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
                    <td>{{.LastAttemptTime.Format "2006-01-02 15:04:05 -0700 MST"}}</td>
                    <td>{{if .BlockTime.IsZero}} - {{else}}{{.BlockTime.Format "2006-01-02 15:04:05 -0700 MST"}}{{end}}</td>
                    <td>{{if .UnblockTime.IsZero}} - {{else}}{{.UnblockTime.Format "2006-01-02 15:04:05 -0700 MST"}}{{end}}</td>
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
			http.Error(w, "Failed to parse template: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := t.Execute(w, stats); err != nil {
			http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
		}
	}
}
