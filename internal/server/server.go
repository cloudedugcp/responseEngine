package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/notifier"
	"github.com/cloudedugcp/responseEngine/internal/types"
	"github.com/cloudedugcp/responseEngine/internal/web"
)

// Server представляє сервер для обробки подій Falco
type Server struct {
	cfg       *config.Config
	db        *db.Database
	actioners map[string]types.Actioner
	notifiers map[string]notifier.Notifier
}

// NewServer створює новий сервер
func NewServer(cfg *config.Config, database *db.Database, actioners map[string]types.Actioner) *Server {
	notifiers := make(map[string]notifier.Notifier)
	if slackCfg, ok := cfg.Notifiers["slack"]; ok && slackCfg.WebhookURL != "" {
		notifiers["slack"] = notifier.NewSlackNotifier(slackCfg.WebhookURL, slackCfg.CallbackPath)
	}
	return &Server{
		cfg:       cfg,
		db:        database,
		actioners: actioners,
		notifiers: notifiers,
	}
}

// Start запускає сервер
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.eventHandler)
	mux.HandleFunc("/dashboard", web.DashboardHandler(s.db))
	for _, n := range s.notifiers {
		if slackNotifier, ok := n.(*notifier.SlackNotifier); ok { // Коректний type assertion
			mux.HandleFunc(slackNotifier.CallbackPath, func(w http.ResponseWriter, r *http.Request) {
				slackNotifier.HandleCallback(w, r, s.actioners)
			})
		}
	}

	if s.cfg.Server.ListenPort == "" {
		log.Println("Warning: ListenPort is empty, defaulting to :2808")
		s.cfg.Server.ListenPort = ":2808"
	}

	log.Printf("Server starting on %s", s.cfg.Server.ListenPort)
	return http.ListenAndServe(s.cfg.Server.ListenPort, mux)
}

// eventHandler обробляє вхідні події
func (s *Server) eventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event types.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Failed to decode event: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Received event: IP=%s, Rule=%s, Log=%s, Timestamp=%s", event.IP, event.RuleName, event.Log, event.Timestamp)
	if err := s.db.LogEvent(event.IP, event.RuleName, event.Log, event.Timestamp); err != nil {
		log.Printf("Failed to log event: %v", err)
		http.Error(w, "Failed to log event", http.StatusInternalServerError)
		return
	}

	for _, scenario := range s.cfg.Scenarios {
		if strings.Contains(event.RuleName, scenario.FalcoRule) {
			window, err := time.ParseDuration(scenario.Conditions.TimeWindow)
			if err != nil {
				log.Printf("Invalid time window for scenario %s: %v", scenario.Name, err)
				continue
			}

			count, err := s.db.CountEvents(event.IP, window)
			if err != nil {
				log.Printf("Failed to count events for IP %s: %v", event.IP, err)
				continue
			}
			log.Printf("IP %s: %d events in last %s (required: %d)", event.IP, count, window, scenario.Conditions.TriggerCount)

			if count >= scenario.Conditions.TriggerCount {
				log.Printf("Scenario '%s' triggered for IP=%s (conditions met)", scenario.Name, event.IP)
				if scenario.Notify.Enabled {
					log.Printf("Notify enabled for scenario '%s': %v", scenario.Name, scenario.Notify.Enabled)
					for _, n := range s.notifiers {
						if scenario.Notify.Type == "slack" {
							if slackNotifier, ok := n.(*notifier.SlackNotifier); ok {
								actionID, err := slackNotifier.Notify(event, scenario.Name, scenario.GetActioners())
								if err != nil {
									log.Printf("Failed to send notification for scenario %s: %v", scenario.Name, err)
								} else {
									log.Printf("Notification sent for scenario %s, action ID: %s", scenario.Name, actionID)
								}
							}
						}
					}
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}
