package notifier

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/types"
	"github.com/slack-go/slack"
)

// SlackNotifier реалізує нотифікацію через Slack
type SlackNotifier struct {
	WebhookURL   string
	CallbackPath string
}

// NewSlackNotifier створює новий SlackNotifier
func NewSlackNotifier(webhookURL, callbackPath string) *SlackNotifier {
	return &SlackNotifier{
		WebhookURL:   webhookURL,
		CallbackPath: callbackPath,
	}
}

// Notify відправляє повідомлення в Slack
func (sn *SlackNotifier) Notify(event types.Event, scenario string, actioners []types.Actioner) (string, error) {
	actionID := fmt.Sprintf("%x", time.Now().UnixNano())
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Suspicious Activity Detected*\nIP: %s\nRule: %s\nLog: %s\nScenario: %s", event.IP, event.RuleName, event.Log, scenario), false, false),
			nil, nil,
		),
		slack.NewActionBlock(actionID, generateButtons(actioners, actionID)...),
	}

	msg := slack.WebhookMessage{
		Blocks: &slack.Blocks{BlockSet: blocks},
	}
	if err := slack.PostWebhook(sn.WebhookURL, &msg); err != nil {
		return "", err
	}
	return actionID, nil
}

// HandleCallback обробляє callback від Slack
func (sn *SlackNotifier) HandleCallback(w http.ResponseWriter, r *http.Request, actioners map[string]types.Actioner) {
	log.Printf("Received callback request: method=%s, path=%s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		log.Printf("Invalid method for callback: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Отримуємо form-данні від Slack
	if err := r.ParseForm(); err != nil {
		log.Printf("Failed to parse form: %v", err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Витягуємо поле 'payload' і декодуємо його як JSON
	payloadStr := r.FormValue("payload")
	if payloadStr == "" {
		log.Printf("No payload found in request")
		http.Error(w, "No payload in request", http.StatusBadRequest)
		return
	}
	log.Printf("Raw payload: %s", payloadStr)

	var payload slack.InteractionCallback
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		log.Printf("Failed to decode Slack payload: %v", err)
		http.Error(w, "Failed to decode payload", http.StatusBadRequest)
		return
	}
	log.Printf("Decoded Slack payload: action_id=%s, actions=%+v", payload.ActionID, payload.ActionCallback.BlockActions)

	for _, action := range payload.ActionCallback.BlockActions {
		log.Printf("Processing action: ActionID=%s, Value=%s", action.ActionID, action.Value)
		actionerName := action.Value
		if act, ok := actioners[actionerName]; ok {
			event := types.Event{
				IP:        extractIPFromMessage(payload.Message.Text),
				RuleName:  "Suspicious Network Activity",
				Log:       "Slack triggered",
				Timestamp: time.Now(),
			}
			log.Printf("Executing actioner %s with event: %+v", actionerName, event)
			if err := act.Execute(event, map[string]interface{}{"priority": 1000, "bantime": "5m"}); err != nil {
				log.Printf("Failed to execute actioner %s: %v", actionerName, err)
			} else {
				log.Printf("Actioner %s executed successfully via Slack", actionerName)
			}
		} else {
			log.Printf("Actioner %s not found in actioners map", actionerName)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func generateButtons(actioners []types.Actioner, actionID string) []slack.BlockElement {
	var elements []slack.BlockElement
	for _, act := range actioners {
		btn := slack.NewButtonBlockElement(
			fmt.Sprintf("%s_%s", actionID, act.Name()),
			act.Name(),
			slack.NewTextBlockObject("plain_text", fmt.Sprintf("Run %s", act.Name()), true, false),
		)
		elements = append(elements, btn)
	}
	return elements
}

func extractIPFromMessage(text string) string {
	parts := strings.Split(text, "\n")
	for _, part := range parts {
		if strings.Contains(part, "IP:") {
			return strings.TrimSpace(strings.Split(part, "IP:")[1])
		}
	}
	return ""
}
