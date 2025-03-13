package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/notifier"
	"github.com/cloudedugcp/responseEngine/internal/types"
	"github.com/cloudedugcp/responseEngine/internal/web"
)

type Server struct {
	cfg       *config.Config
	db        *db.Database
	actioners map[string]types.Actioner
	notifiers map[string]notifier.Notifier
	mu        sync.Mutex
	pending   map[string]time.Time // actionID -> expiration time
}

// NewServer створює новий сервер
func NewServer(cfg *config.Config, database *db.Database, actioners map[string]types.Actioner) *Server {
	s := &Server{
		cfg:       cfg,
		db:        database,
		actioners: actioners,
		notifiers: make(map[string]notifier.Notifier),
		pending:   make(map[string]time.Time),
	}
	if slackCfg, ok := cfg.Notifiers["slack"]; ok && slackCfg.WebhookURL != "" {
		s.notifiers["slack"] = notifier.NewSlackNotifier(slackCfg.WebhookURL, slackCfg.CallbackPath)
	}
	go s.runPendingActions()
	return s
}

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
	log.Printf("Received event: %+v", event)

	if err := s.db.LogEvent(event.IP, event.RuleName, event.Log, event.Timestamp); err != nil {
		log.Printf("Failed to log event: %v", err)
	}

	for _, scenario := range s.cfg.Scenarios {
		if !strings.Contains(event.RuleName, scenario.FalcoRule) {
			continue
		}

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
				for _, n := range s.notifiers {
					if scenario.Notify.Type == "slack" {
						if slackNotifier, ok := n.(*notifier.SlackNotifier); ok {
							actionID, err := slackNotifier.Notify(event, scenario.Name, scenario.GetActioners())
							if err != nil {
								log.Printf("Failed to send notification for scenario %s: %v", scenario.Name, err)
							} else {
								log.Printf("Notification sent for scenario %s, action ID: %s", scenario.Name, actionID)
								s.scheduleAutoRun(scenario, event, actionID)
							}
						}
					}
				}
			} else {
				s.executeAllActioners(scenario, event)
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/falco", s.eventHandler)
	mux.HandleFunc("/dashboard", web.DashboardHandler(s.db))
	for _, n := range s.notifiers {
		if slackNotifier, ok := n.(*notifier.SlackNotifier); ok {
			log.Printf("Registering Slack callback handler at %s", slackNotifier.CallbackPath)
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

func (s *Server) scheduleAutoRun(scenario config.ScenarioConfig, event types.Event, actionID string) {
	delay, err := time.ParseDuration(scenario.AutoRunDelay)
	if err != nil {
		log.Printf("Invalid auto_run_delay %s for scenario %s, defaulting to 10m", scenario.AutoRunDelay, scenario.Name)
		delay = 10 * time.Minute
	}

	s.mu.Lock()
	s.pending[actionID] = time.Now().Add(delay)
	s.mu.Unlock()

	log.Printf("Scheduled auto-run for action ID %s at %s", actionID, s.pending[actionID])
}

func (s *Server) runPendingActions() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for actionID, expiry := range s.pending {
			if now.After(expiry) {
				log.Printf("Executing auto-run for action ID %s", actionID)
				for _, n := range s.notifiers {
					if slack, ok := n.(*notifier.SlackNotifier); ok {
						slack.ExecutePending(actionID, s.actioners)
						delete(s.pending, actionID)
					}
				}
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) executeAllActioners(scenario config.ScenarioConfig, event types.Event) {
	for _, actCfg := range scenario.Actioners {
		if act, ok := s.actioners[actCfg.Name]; ok {
			if err := act.Execute(event, actCfg.Params); err != nil {
				log.Printf("Failed to execute actioner %s: %v", actCfg.Name, err)
			} else {
				log.Printf("Actioner %s executed successfully", actCfg.Name)
			}
		}
	}
}
