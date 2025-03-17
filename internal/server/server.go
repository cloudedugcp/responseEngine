package server

import (
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
		"sigmahq":      actioner.NewSigmaHQActioner(cfg.Actioners["sigmahq"]),
	}

	slackNotifier := notifier.NewSlackNotifier(
		cfg.Notifier.Slack.WebhookURL,
		cfg.Notifier.Slack.CallbackURL,
		cfg.Notifier.Slack.BotToken, // Передаємо Bot Token
		cfg.Notifier.Slack.Channel,  // Передаємо Channel ID
	)
	scenarioMgr := scenario.NewManager(cfg, actioners, db, slackNotifier)

	return &Server{cfg, actioners, db, slackNotifier, scenarioMgr}, nil
}

func (s *Server) Start() {
	mux := http.NewServeMux()
	for alias, path := range s.cfg.Server.Aliases {
		log.Printf("Registering alias %s at %s", alias, path)
		mux.HandleFunc(path, s.handleEvent)
	}

	callbackURL, err := url.Parse(s.cfg.Notifier.Slack.CallbackURL)
	if err != nil {
		log.Fatalf("Failed to parse callback_url from config: %v", err)
	}
	callbackPath := callbackURL.Path
	if callbackPath == "" {
		callbackPath = "/callback"
	}
	log.Printf("Registering Slack callback at %s", callbackPath)
	mux.HandleFunc(callbackPath, s.handleSlackCallback)

	go web.StartDashboard(s.cfg.Server.DashboardPort, s.db, s.scenarios)

	log.Printf("Server starting on :%d", s.cfg.Server.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", s.cfg.Server.Port), mux))
}

func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	var falcoEvent struct {
		Rule         string `json:"rule"`
		OutputFields struct {
			RemoteIP string `json:"fd.rip"`
			Result   string `json:"evt.res"`
			SourceIP string `json:"fd.sip"`
		} `json:"output_fields"`
		Time string `json:"time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&falcoEvent); err != nil {
		log.Printf("Failed to decode event: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	ip := falcoEvent.OutputFields.RemoteIP
	if ip == "" {
		log.Printf("No remote IP found in event")
		http.Error(w, "Missing IP", http.StatusBadRequest)
		return
	}

	log.Printf("Received event for IP %s with rule %s", ip, falcoEvent.Rule)
	event := models.Event{
		IP:       ip,
		Rule:     falcoEvent.Rule,
		Result:   falcoEvent.OutputFields.Result,
		Time:     falcoEvent.Time,
		SourceIP: falcoEvent.OutputFields.SourceIP,
	}
	s.scenarios.HandleEvent("block_ip", event)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSlackCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received Slack callback: Method=%s, URL=%s", r.Method, r.URL.String())

	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Printf("Failed to parse form: %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	payloadRaw := r.FormValue("payload")
	if payloadRaw == "" {
		log.Printf("No payload found in callback")
		http.Error(w, "Missing payload", http.StatusBadRequest)
		return
	}
	log.Printf("Raw callback payload: %s", payloadRaw)

	var payload struct {
		CallbackID string `json:"callback_id"`
		Actions    []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"actions"`
		OriginalMessage struct {
			Text string `json:"text"`
		} `json:"original_message"`
	}
	if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
		log.Printf("Failed to decode payload: %v", err)
		http.Error(w, "Invalid JSON in payload", http.StatusBadRequest)
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

		response := struct {
			Text        string `json:"text"`
			Attachments []struct {
				Text string `json:"text"`
			} `json:"attachments"`
			ReplaceOriginal bool `json:"replace_original"`
		}{
			Text: payload.OriginalMessage.Text,
			Attachments: []struct {
				Text string `json:"text"`
			}{
				{Text: fmt.Sprintf("Дію %s виконано для IP %s.", action, ip)},
			},
			ReplaceOriginal: true,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
			return
		}
	} else {
		log.Printf("Invalid callback: CallbackID=%s, Actions count=%d", payload.CallbackID, len(payload.Actions))
		w.WriteHeader(http.StatusOK)
	}
}
