package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/notifier"
	"github.com/cloudedugcp/responseEngine/internal/scenario"
	"github.com/cloudedugcp/responseEngine/pkg/models"

	"github.com/cloudedugcp/responseEngine/internal/web"
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
	for alias, path := range s.cfg.Server.Aliases {
		log.Printf("Registering alias %s at %s", alias, path)
		mux.HandleFunc(path, s.handleEvent)
	}

	// Витягуємо шлях із callback_url
	callbackURL, err := url.Parse(s.cfg.Notifier.Slack.CallbackURL)
	if err != nil {
		log.Fatalf("Failed to parse callback_url from config: %v", err)
	}
	callbackPath := callbackURL.Path
	if callbackPath == "" {
		callbackPath = "/callback" // Запасний варіант, якщо шлях не вказано
	}
	log.Printf("Registering Slack callback at %s", callbackPath)
	mux.HandleFunc(callbackPath, s.handleSlackCallback)

	go web.StartDashboard(s.cfg.Server.DashboardPort, s.db)

	log.Printf("Server starting on :%d", s.cfg.Server.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", s.cfg.Server.Port), mux))
}

func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("Failed to decode event: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	log.Printf("Received event for IP %s", event.IP)
	s.scenarios.HandleEvent("block_ip", event)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSlackCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received Slack callback: Method=%s, URL=%s", r.Method, r.URL.String())

	var rawBody bytes.Buffer
	if _, err := rawBody.ReadFrom(r.Body); err != nil {
		log.Printf("Failed to read callback body: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	log.Printf("Raw callback body: %s", rawBody.String())

	var slackPayload struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(rawBody.Bytes(), &slackPayload); err != nil {
		log.Printf("Failed to decode outer payload: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var payload struct {
		CallbackID string `json:"callback_id"`
		Actions    []struct {
			Value string `json:"value"`
		} `json:"actions"`
		OriginalMessage struct {
			Text string `json:"text"`
		} `json:"original_message"`
	}
	if err := json.Unmarshal([]byte(slackPayload.Payload), &payload); err != nil {
		log.Printf("Failed to decode inner payload: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Parsed callback: CallbackID=%s, Actions=%+v, OriginalMessage=%s",
		payload.CallbackID, payload.Actions, payload.OriginalMessage.Text)

	if payload.CallbackID == "block_ip_action" && len(payload.Actions) > 0 {
		action := payload.Actions[0].Value
		var ip string
		if _, err := fmt.Sscanf(payload.OriginalMessage.Text, "IP %s triggered scenario block_ip", &ip); err != nil {
			log.Printf("Failed to extract IP from message: %v", err)
			http.Error(w, "Cannot parse IP", http.StatusBadRequest)
			return
		}

		log.Printf("Executing action %s for IP %s from Slack callback", action, ip)
		s.scenarios.ExecuteAction(action, ip)
	} else {
		log.Printf("Invalid callback: CallbackID=%s, Actions count=%d", payload.CallbackID, len(payload.Actions))
	}

	w.WriteHeader(http.StatusOK)
}
