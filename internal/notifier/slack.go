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

type SlackAction struct {
	Name  string `json:"name"`
	Text  string `json:"text"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type SlackAttachment struct {
	Text       string        `json:"text"`
	CallbackID string        `json:"callback_id"`
	Actions    []SlackAction `json:"actions"`
}

type SlackMessage struct {
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments"`
}

func NewSlackNotifier(webhookURL, callbackURL string) *SlackNotifier {
	return &SlackNotifier{webhookURL, callbackURL}
}

func (s *SlackNotifier) SendMessageWithButtons(text string, buttons []SlackButton) error {
	// Формуємо дії (кнопки) для Slack
	var slackActions []SlackAction
	for _, button := range buttons {
		slackActions = append(slackActions, SlackAction{
			Name:  button.Name,
			Text:  button.Name, // Текст на кнопці
			Type:  "button",
			Value: button.Value,
		})
	}

	// Створюємо повідомлення з вкладенням для кнопок
	msg := SlackMessage{
		Text: text,
		Attachments: []SlackAttachment{
			{
				Text:       "Choose an action:",
				CallbackID: "block_ip_action", // Унікальний ID для callback
				Actions:    slackActions,
			},
		},
	}

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
