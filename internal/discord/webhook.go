package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// WedhookClient sends messages to a Discord webhook
type WebhookClient struct {
	URL        string
	httpClient *http.Client
}

// NewWebhookClient creates a new WebhookClient
func NewWebhookClient(url string) *WebhookClient {
	return &WebhookClient{
		URL:        url,
		httpClient: &http.Client{},
	}
}

// Message represents a Discord webhook payload
type Message struct {
	Content string `json:"content"`
}

// Send sends a message to the Discord webhook
func (c *WebhookClient) Send(content string) error {
	msg := Message{Content: content}
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Failed to marshal webhook payload: %w", err)
	}

	resp, err := c.httpClient.Post(c.URL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("Failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Webhook request failed with status code %d", resp.StatusCode)
	}

	return nil
}
