package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func (s *SlackNotifier) SendMessageWithButtons(text string, buttons []SlackButton) error {
	msg := SlackMessage{Text: text, Actions: buttons}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	resp, err := http.Post(s.webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack API returned non-OK status: %d", resp.StatusCode)
	}

	return nil
}
