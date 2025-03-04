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

	// Обробник подій
	mux.HandleFunc("/", s.eventHandler)

	// Веб-інтерфейс
	mux.HandleFunc("/dashboard", web.DashboardHandler(s.db))

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

	for _, sc := range s.cfg.Scenarios {
		if sc.FalcoRule == event.RuleName {
			shouldExecute := true
			if sc.Conditions != nil {
				shouldExecute = scenario.ShouldTrigger(*sc.Conditions, event, s.db) // Тип уже співпадає
			}

			if shouldExecute {
				for _, sa := range sc.Actioners {
					if actioner, ok := s.actioners[sa.Name]; ok {
						if err := actioner.Execute(event, sa.Params); err != nil {
							log.Printf("Error executing actioner %s: %v", sa.Name, err)
						}
						actionType := "store"
						if sa.Name == "firewall" {
							actionType = "block"
						}
						s.db.LogAction(event.IP, actionType, time.Now())
					}
				}
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}
