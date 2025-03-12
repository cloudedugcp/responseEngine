package notifier

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/slack-go/slack"
)

// SlackNotifier відповідає за відправку повідомлень у Slack та обробку callback-запитів
type SlackNotifier struct {
	webhookURL   string
	CallbackPath string
	mu           sync.Mutex
	pending      map[string]PendingAction
}

// PendingAction зберігає інформацію про очікувані дії
type PendingAction struct {
	Event     actioner.Event
	Scenario  string
	Actioners []struct {
		Actioner actioner.Actioner
		Params   map[string]interface{}
	}
}

// NewSlackNotifier створює новий SlackNotifier
func NewSlackNotifier(webhookURL, callbackPath string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL:   webhookURL,
		CallbackPath: callbackPath,
		pending:      make(map[string]PendingAction),
	}
}

// Notify відправляє повідомлення в Slack із кнопками для виконання діячів
func (sn *SlackNotifier) Notify(event actioner.Event, scenario string, actioners []actioner.Actioner) (string, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", fmt.Errorf("failed to generate action ID: %v", err)
	}
	actionID := hex.EncodeToString(idBytes)

	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf(
				"*Suspicious Activity Detected*\nIP: %s\nRule: %s\nLog: %s\nScenario: %s",
				event.IP, event.RuleName, event.Log, scenario), false, false),
			nil, nil,
		),
	}

	var elements []slack.BlockElement
	for _, act := range actioners {
		elements = append(elements, slack.NewButtonBlockElement(
			actionID+"_"+act.Name(),
			act.Name(),
			slack.NewTextBlockObject("plain_text", "Run "+act.Name(), true, false),
		))
	}
	elements = append(elements, slack.NewButtonBlockElement(
		actionID+"_all",
		"all",
		slack.NewTextBlockObject("plain_text", "Run All", true, false),
	))
	blocks = append(blocks, slack.NewActionBlock(actionID, elements...))

	payload := slack.WebhookMessage{
		Blocks: &slack.Blocks{BlockSet: blocks},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Slack payload: %v", err)
	}

	resp, err := http.Post(sn.webhookURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to send Slack notification: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Slack returned non-OK status: %s", resp.Status)
	}

	pendingActioners := make([]struct {
		Actioner actioner.Actioner
		Params   map[string]interface{}
	}, len(actioners)) // Виправлено синтаксис
	for i, act := range actioners {
		pendingActioners[i] = struct {
			Actioner actioner.Actioner
			Params   map[string]interface{}
		}{Actioner: act, Params: map[string]interface{}{}}
	}

	sn.mu.Lock()
	sn.pending[actionID] = PendingAction{
		Event:     event,
		Scenario:  scenario,
		Actioners: pendingActioners,
	}
	log.Printf("Stored %d actioners for action ID: %s", len(pendingActioners), actionID)
	sn.mu.Unlock()

	return actionID, nil
}

// HandleCallback обробляє callback-запити від Slack
func (sn *SlackNotifier) HandleCallback(w http.ResponseWriter, r *http.Request, actioners map[string]actioner.Actioner) {
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Printf("Failed to parse form: %v", err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	payload := r.FormValue("payload")
	log.Printf("Received payload: %s", payload)
	if payload == "" {
		log.Printf("Empty payload received")
		http.Error(w, "Empty payload", http.StatusBadRequest)
		return
	}

	var slackResp struct {
		Actions []struct {
			ActionID string `json:"action_id"`
			Value    string `json:"value"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(payload), &slackResp); err != nil {
		log.Printf("Failed to unmarshal payload: %v", err)
		http.Error(w, "Invalid Slack payload", http.StatusBadRequest)
		return
	}

	if len(slackResp.Actions) == 0 {
		log.Printf("No actions in payload")
		http.Error(w, "No action provided", http.StatusBadRequest)
		return
	}

	actionID := slackResp.Actions[0].ActionID
	actionValue := slackResp.Actions[0].Value
	log.Printf("Action ID: %s, Value: %s", actionID, actionValue)

	sn.mu.Lock()
	actionIDPrefix := actionID
	for i := len(actionID) - 1; i >= 0; i-- {
		if actionID[i] == '_' {
			actionIDPrefix = actionID[:i]
			break
		}
	}
	pending, exists := sn.pending[actionIDPrefix]
	sn.mu.Unlock()

	if !exists {
		log.Printf("Unknown action ID: %s", actionID)
		http.Error(w, "Unknown action ID", http.StatusBadRequest)
		return
	}

	if actionValue == "all" {
		for _, act := range pending.Actioners {
			if err := act.Actioner.Execute(pending.Event, act.Params); err != nil {
				log.Printf("Failed to execute actioner %s: %v", act.Actioner.Name(), err)
			} else {
				log.Printf("Actioner '%s' executed successfully via Slack for IP=%s", act.Actioner.Name(), pending.Event.IP)
			}
		}
	} else {
		for _, act := range pending.Actioners {
			if act.Actioner.Name() == actionValue {
				if err := act.Actioner.Execute(pending.Event, act.Params); err != nil {
					log.Printf("Failed to execute actioner %s: %v", act.Actioner.Name(), err)
				} else {
					log.Printf("Actioner '%s' executed successfully via Slack for IP=%s", act.Actioner.Name(), pending.Event.IP)
				}
				break
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// UpdatePending оновлює параметри діячів для заданого actionID
func (sn *SlackNotifier) UpdatePending(actionID string, actioners []struct {
	Actioner actioner.Actioner
	Params   map[string]interface{}
}) {
	sn.mu.Lock()
	defer sn.mu.Unlock()

	if pending, exists := sn.pending[actionID]; exists {
		for i, act := range actioners {
			if i < len(pending.Actioners) {
				pending.Actioners[i].Params = act.Params
			}
		}
		sn.pending[actionID] = pending
	}
}
