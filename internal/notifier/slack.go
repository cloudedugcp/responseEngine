package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type SlackNotifier struct {
	WebhookURL  string
	CallbackURL string
	BotToken    string // Додано Bot Token
	Channel     string // Додано Channel ID
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

func NewSlackNotifier(webhookURL, callbackURL, botToken, channel string) *SlackNotifier {
	return &SlackNotifier{
		WebhookURL:  webhookURL,
		CallbackURL: callbackURL,
		BotToken:    botToken,
		Channel:     channel,
	}
}

func (s *SlackNotifier) SendMessageWithButtons(text string, buttons []SlackButton) (string, error) {
	log.Printf("Sending Slack message with %d buttons: %+v", len(buttons), buttons)

	var slackActions []SlackAction
	for _, button := range buttons {
		slackActions = append(slackActions, SlackAction{
			Name:  button.Name,
			Text:  button.Name,
			Type:  "button",
			Value: button.Value,
		})
	}

	if len(slackActions) == 0 {
		log.Printf("No actions generated for Slack message")
	}

	payload := struct {
		Channel     string            `json:"channel"`
		Text        string            `json:"text"`
		Attachments []SlackAttachment `json:"attachments"`
	}{
		Channel: s.Channel,
		Text:    text,
		Attachments: []SlackAttachment{
			{
				Text:       "Choose an action:",
				CallbackID: "block_ip_action",
				Actions:    slackActions,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal Slack message: %v", err)
		return "", err
	}

	req, err := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create Slack request: %v", err)
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.BotToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send Slack message: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Ok    bool   `json:"ok"`
		Ts    string `json:"ts"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to decode Slack response: %v", err)
		return "", err
	}

	if !result.Ok {
		log.Printf("Slack API error: %s", result.Error)
		return "", fmt.Errorf("Slack API error: %s", result.Error)
	}

	log.Printf("Slack message sent successfully with ts: %s", result.Ts)
	return result.Ts, nil
}

func (s *SlackNotifier) UpdateMessage(ts, newText string) error {
	payload := struct {
		Channel string `json:"channel"`
		Ts      string `json:"ts"`
		Text    string `json:"text"`
	}{
		Channel: s.Channel,
		Ts:      ts,
		Text:    newText,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal Slack update payload: %v", err)
		return fmt.Errorf("failed to marshal Slack update payload: %v", err)
	}

	req, err := http.NewRequest("POST", "https://slack.com/api/chat.update", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create Slack update request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.BotToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to update Slack message: %v", err)
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to decode Slack update response: %v", err)
		return err
	}

	if !result.Ok {
		log.Printf("Slack API update error: %s", result.Error)
		return fmt.Errorf("Slack API update error: %s", result.Error)
	}

	log.Printf("Slack message updated successfully for ts: %s", ts)
	return nil
}
