package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/notifier"
	"github.com/cloudedugcp/responseEngine/internal/scenario"
	"github.com/cloudedugcp/responseEngine/internal/web"
	"github.com/cloudedugcp/responseEngine/pkg/models"
)

type Server struct {
	cfg       *config.Config
	actioners map[string]actioner.Actioner
	db        *db.SQLiteDB
	notifier  *notifier.SlackNotifier
	scenarios *scenario.Manager
}

func NewServer(cfg *config.Config) (*Server, error) {
	db, err := db.NewSQLiteDB("blocks.db")
	if err != nil {
		return nil, err
	}

	actioners := map[string]actioner.Actioner{
		"gcp_firewall": actioner.NewGCPFirewall(cfg.Actioners["gcp_firewall"]),
		"gcp_storage":  actioner.NewGCPStorage(cfg.Actioners["gcp_storage"]),
	}

	slackNotifier := notifier.NewSlackNotifier(cfg.Notifier.Slack.WebhookURL, cfg.Notifier.Slack.CallbackURL)
	scenarioMgr := scenario.NewManager(cfg, actioners, db, slackNotifier)

	return &Server{cfg, actioners, db, slackNotifier, scenarioMgr}, nil
}

func (s *Server) Start() {
	mux := http.NewServeMux()
	for _, alias := range s.cfg.Server.Aliases {
		mux.HandleFunc(alias, s.handleEvent)
	}
	mux.HandleFunc("/callback", s.handleSlackCallback)

	go web.StartDashboard(s.cfg.Server.DashboardPort, s.db)

	log.Printf("Server starting on :%d", s.cfg.Server.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", s.cfg.Server.Port), mux))
}

func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	s.scenarios.HandleEvent("block_ip", event)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSlackCallback(w http.ResponseWriter, r *http.Request) {
	// Обробка callback від Slack
	action := r.FormValue("action")
	ip := r.FormValue("ip")
	s.scenarios.ExecuteAction(action, ip)
	w.WriteHeader(http.StatusOK)
}
