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
	var payload slack.InteractionCallback
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Failed to decode Slack payload", http.StatusBadRequest)
		return
	}
	log.Printf("Received payload: %+v", payload)

	for _, action := range payload.ActionCallback.BlockActions {
		log.Printf("Action ID: %s, Value: %s", action.ActionID, action.Value)
		actionerName := action.Value
		if act, ok := actioners[actionerName]; ok {
			event := types.Event{ // Змінено на types.Event
				IP:        extractIPFromMessage(payload.Message.Text),
				RuleName:  "Suspicious Network Activity",
				Log:       "Slack triggered",
				Timestamp: time.Now(),
			}
			if err := act.Execute(event, map[string]interface{}{"priority": 1000, "timeout": "5m"}); err != nil {
				log.Printf("Failed to execute actioner %s: %v", actionerName, err)
			} else {
				log.Printf("Actioner %s executed successfully via Slack", actionerName)
			}
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
