package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/notifier"
	"github.com/cloudedugcp/responseEngine/internal/web"
)

// Server представляє сервер для обробки подій Falco
type Server struct {
	cfg       *config.Config
	db        *db.Database
	actioners map[string]actioner.Actioner
	notifiers map[string]notifier.Notifier
}

// NewServer створює новий сервер
func NewServer(cfg *config.Config, database *db.Database, actioners map[string]actioner.Actioner) *Server {
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
		if slackNotifier, ok := n.(*notifier.SlackNotifier); ok {
			mux.HandleFunc(slackNotifier.CallbackPath, func(w http.ResponseWriter, r *http.Request) {
				slackNotifier.HandleCallback(w, r, s.actioners)
			})
		}
	}

	if s.cfg.Server.ListenPort == "" {
		log.Println("Warning: ListenPort is empty, defaulting to :8080")
		s.cfg.Server.ListenPort = ":8080"
	}

	return http.ListenAndServe(s.cfg.Server.ListenPort, mux)
}

func (s *Server) eventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event actioner.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("Failed to decode event: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	log.Printf("Received event: IP=%s, Rule=%s, Log=%s, Timestamp=%s", event.IP, event.RuleName, event.Log, event.Timestamp.Format(time.RFC3339))

	if event.IP == "" {
		log.Printf("Warning: Event with empty IP received (Rule=%s)", event.RuleName)
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := s.db.LogEvent(event.IP, event.RuleName, event.Log, event.Timestamp); err != nil {
		log.Printf("Failed to log event in database: %v", err)
	}

	for _, sc := range s.cfg.Scenarios {
		if sc.FalcoRule != event.RuleName {
			continue
		}

		timeWindow, err := time.ParseDuration(sc.Conditions.TimeWindow)
		if err != nil {
			log.Printf("Invalid time window for scenario %s: %v", sc.Name, err)
			continue
		}
		count, err := s.db.CountEvents(event.IP, timeWindow)
		if err != nil {
			log.Printf("Failed to count events for IP %s: %v", event.IP, err)
			continue
		}
		log.Printf("IP %s: %d events in last %s (required: %d)", event.IP, count, sc.Conditions.TimeWindow, sc.Conditions.TriggerCount)

		shouldExecute := count >= sc.Conditions.TriggerCount
		if shouldExecute {
			log.Printf("Notify enabled for scenario '%s': %v", sc.Name, sc.Notify.Enabled)
			actioners := make([]actioner.Actioner, 0, len(sc.Actioners))
			for _, sa := range sc.Actioners {
				if act, ok := s.actioners[sa.Name]; ok {
					actioners = append(actioners, act)
				}
			}

			if sc.Notify.Enabled {
				if n, ok := s.notifiers[sc.Notify.Type]; ok {
					actionID, err := n.Notify(event, sc.Name, actioners)
					if err != nil {
						log.Printf("Failed to send notification for scenario %s: %v", sc.Name, err)
					} else {
						log.Printf("Notification sent for scenario %s, action ID: %s", sc.Name, actionID)
						if slackNotifier, ok := n.(*notifier.SlackNotifier); ok {
							// Оновлюємо pending через метод UpdatePending
							updatedActioners := make([]struct {
								Actioner actioner.Actioner
								Params   map[string]interface{}
							}, len(actioners))
							for i, sa := range sc.Actioners {
								if i < len(actioners) {
									updatedActioners[i] = struct {
										Actioner actioner.Actioner
										Params   map[string]interface{}
									}{Actioner: actioners[i], Params: sa.Params}
								}
							}
							slackNotifier.UpdatePending(actionID, updatedActioners)
						}
					}
				}
			} else {
				for _, sa := range sc.Actioners {
					if actioner, ok := s.actioners[sa.Name]; ok {
						err := actioner.Execute(event, sa.Params)
						if err != nil {
							log.Printf("Error executing actioner %s: %v", sa.Name, err)
						} else {
							log.Printf("Actioner '%s' executed successfully for IP=%s", sa.Name, event.IP)
							actionType := "store"
							status := "stored"
							if sa.Name == "firewall" {
								actionType = "block"
								status = "blocked"
							}
							if err := s.db.LogAction(event.IP, actionType, status, time.Now()); err != nil {
								log.Printf("Failed to log action %s to database: %v", actionType, err)
							}
						}
					}
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}
