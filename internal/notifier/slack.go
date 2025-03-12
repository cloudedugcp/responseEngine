package notifier

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
)

// Notifier - інтерфейс для нотифікаторів
type Notifier interface {
	Notify(event actioner.Event, scenario string, actioners []actioner.Actioner) (string, error)
	HandleCallback(w http.ResponseWriter, r *http.Request, actioners map[string]actioner.Actioner)
}

// SlackNotifier - реалізація нотифікатора для Slack
type SlackNotifier struct {
	webhookURL   string
	CallbackPath string // Змінено на велику літеру
	pending      map[string]PendingAction
	mu           sync.Mutex
}

// PendingAction - структура для збереження очікуваних дій
type PendingAction struct {
	Event     actioner.Event
	Scenario  string
	Actioners []actioner.Actioner
}

// NewSlackNotifier - створює новий SlackNotifier
func NewSlackNotifier(webhookURL, callbackPath string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL:   webhookURL,
		CallbackPath: callbackPath, // Оновлено
		pending:      make(map[string]PendingAction),
	}
}

// Notify - відправляє повідомлення в Slack із кнопками
func (sn *SlackNotifier) Notify(event actioner.Event, scenario string, actioners []actioner.Actioner) (string, error) {
	// Генеруємо унікальний ID для дії
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", err
	}
	actionID := hex.EncodeToString(idBytes)

	// Формуємо Slack-повідомлення з кнопками
	blocks := []interface{}{
		map[string]interface{}{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Suspicious Activity Detected*\nIP: %s\nRule: %s\nLog: %s\nScenario: %s", event.IP, event.RuleName, event.Log, scenario),
			},
		},
		map[string]interface{}{
			"type":     "actions",
			"elements": []interface{}{},
		},
	}

	actionElements := blocks[1].(map[string]interface{})["elements"].([]interface{})
	for _, act := range actioners {
		actionElements = append(actionElements, map[string]interface{}{
			"type": "button",
			"text": map[string]interface{}{
				"type": "plain_text",
				"text": fmt.Sprintf("Run %s", act.Name()),
			},
			"action_id": fmt.Sprintf("%s_%s", actionID, act.Name()),
			"value":     act.Name(),
		})
	}
	// Додаємо кнопку "Run All"
	actionElements = append(actionElements, map[string]interface{}{
		"type": "button",
		"text": map[string]interface{}{
			"type": "plain_text",
			"text": "Run All",
		},
		"action_id": fmt.Sprintf("%s_all", actionID),
		"value":     "all",
	})
	blocks[1].(map[string]interface{})["elements"] = actionElements

	payload := map[string]interface{}{
		"blocks": blocks,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Slack payload: %v", err)
	}

	// Відправляємо в Slack
	resp, err := http.Post(sn.webhookURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to send Slack notification: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Slack returned non-OK status: %d", resp.StatusCode)
	}

	// Зберігаємо очікувану дію
	sn.mu.Lock()
	sn.pending[actionID] = PendingAction{
		Event:     event,
		Scenario:  scenario,
		Actioners: actioners,
	}
	sn.mu.Unlock()

	return actionID, nil
}

// HandleCallback - обробляє відповіді від Slack
func (sn *SlackNotifier) HandleCallback(w http.ResponseWriter, r *http.Request, actioners map[string]actioner.Actioner) {
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Логуємо заголовки та сире тіло для дебагу
	log.Printf("Request headers: %v", r.Header)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	log.Printf("Raw request body: %s", string(body))

	// Парсимо form-даних
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
	delete(sn.pending, actionIDPrefix)
	sn.mu.Unlock()

	if !exists {
		log.Printf("Unknown action ID: %s", actionID)
		http.Error(w, "Unknown action ID", http.StatusBadRequest)
		return
	}

	if actionValue == "all" {
		for _, act := range pending.Actioners {
			if err := act.Execute(pending.Event, map[string]interface{}{}); err != nil {
				log.Printf("Failed to execute actioner %s: %v", act.Name(), err)
			} else {
				log.Printf("Actioner '%s' executed successfully via Slack for IP=%s", act.Name(), pending.Event.IP)
			}
		}
	} else {
		for _, act := range pending.Actioners {
			if act.Name() == actionValue {
				if err := act.Execute(pending.Event, map[string]interface{}{}); err != nil {
					log.Printf("Failed to execute actioner %s: %v", act.Name(), err)
				} else {
					log.Printf("Actioner '%s' executed successfully via Slack for IP=%s", act.Name(), pending.Event.IP)
				}
				break
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}
