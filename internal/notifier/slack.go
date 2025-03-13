package notifier

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type SlackNotifier struct {
	webhookURL  string
	callbackURL string
}

type SlackButton struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SlackMessage struct {
	Text    string        `json:"text"`
	Actions []SlackButton `json:"actions"`
}

func NewSlackNotifier(webhookURL, callbackURL string) *SlackNotifier {
	return &SlackNotifier{webhookURL, callbackURL}
}

func (s *SlackNotifier) SendMessageWithButtons(text string, buttons []SlackButton) {
	msg := SlackMessage{Text: text, Actions: buttons}
	payload, _ := json.Marshal(msg)
	http.Post(s.webhookURL, "application/json", bytes.NewBuffer(payload))
}
