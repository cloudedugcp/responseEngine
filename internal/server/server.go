package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/scenario"
	"github.com/cloudedugcp/responseEngine/internal/web"
)

// Server - структура сервера
type Server struct {
	cfg       *config.Config
	db        *db.Database
	actioners map[string]actioner.Actioner
}

// NewServer - створює новий сервер
func NewServer(cfg *config.Config, database *db.Database, actioners map[string]actioner.Actioner) *Server {
	return &Server{
		cfg:       cfg,
		db:        database,
		actioners: actioners,
	}
}

// Start - запускає сервер
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.eventHandler)
	mux.HandleFunc("/dashboard", web.DashboardHandler(s.db))

	if s.cfg.Server.ListenPort == "" {
		log.Println("Warning: ListenPort is empty, defaulting to :8080")
		s.cfg.Server.ListenPort = ":8080"
	}

	return http.ListenAndServe(s.cfg.Server.ListenPort, mux)
}

// eventHandler - обробляє вхідні події
func (s *Server) eventHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if _, ok := s.cfg.Server.Aliases[path]; !ok {
		http.Error(w, "Invalid endpoint", http.StatusNotFound)
		return
	}

	var event actioner.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Received event: IP=%s, Rule=%s, Time=%s", event.IP, event.RuleName, time.Now().Format(time.RFC3339))

	// Записуємо подію в БД одразу після отримання
	if event.IP != "" { // Перевіряємо, чи IP не порожній
		s.db.LogAction(event.IP, "event", time.Now())
	} else {
		log.Printf("Warning: Event with empty IP received (Rule=%s)", event.RuleName)
	}

	for _, sc := range s.cfg.Scenarios {
		if sc.FalcoRule == event.RuleName && event.IP != "" { // Додаємо перевірку на порожній IP
			shouldExecute := true
			if sc.Conditions != nil {
				shouldExecute = scenario.ShouldTrigger(*sc.Conditions, event, s.db)
				if shouldExecute {
					log.Printf("Scenario '%s' triggered for IP=%s (conditions met)", sc.Name, event.IP)
				} else {
					log.Printf("Scenario '%s' conditions not met for IP=%s", sc.Name, event.IP)
				}
			}

			if shouldExecute {
				for _, sa := range sc.Actioners {
					if actioner, ok := s.actioners[sa.Name]; ok {
						if err := actioner.Execute(event, sa.Params); err != nil {
							log.Printf("Error executing actioner %s: %v", sa.Name, err)
						} else {
							log.Printf("Actioner '%s' executed successfully for IP=%s", sa.Name, event.IP)
						}
						actionType := "store"
						if sa.Name == "firewall" {
							actionType = "block"
						}
						s.db.LogAction(event.IP, actionType, time.Now()) // Логування дії
					}
				}
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}
