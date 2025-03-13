package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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
	// Логуємо вхідні кнопки
	log.Printf("Sending Slack message with %d buttons: %+v", len(buttons), buttons)

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

	// Перевіряємо, чи є дії
	if len(slackActions) == 0 {
		log.Printf("No actions generated for Slack message")
	}

	// Створюємо повідомлення з вкладенням для кнопок
	msg := SlackMessage{
		Text: text,
		Attachments: []SlackAttachment{
			{
				Text:       "Choose an action:",
				CallbackID: "block_ip_action",
				Actions:    slackActions,
			},
		},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal Slack message: %v", err)
		return err
	}

	// Логуємо JSON, який надсилається
	log.Printf("Slack payload: %s", string(payload))

	resp, err := http.Post(s.webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("Failed to send Slack message: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Slack API returned non-OK status: %d", resp.StatusCode)
		return fmt.Errorf("Slack API returned non-OK status: %d", resp.StatusCode)
	}

	log.Printf("Slack message sent successfully")
	return nil
}
