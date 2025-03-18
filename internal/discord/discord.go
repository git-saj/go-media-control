package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go-media-control/internal/media"

	"github.com/spf13/viper"
)

var (
	client = &http.Client{Timeout: 10 * time.Second}
)

func SendWebhook(media media.Media) error {
	// Get webhook URL at call time, not during initialization
	webhookURL := viper.GetString("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		return fmt.Errorf("discord webhook URL is not configured")
	}

	payload := map[string]string{
		"content": fmt.Sprintf("!play --livestream --room 1333807788521951254 %s", media.URL), // TODO: should be dynamic?
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	for i := 0; i < 3; i++ { // Retry up to 3 times
		resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(body))
		if err != nil {
			slog.Error("Failed to send webhook", "attempt", i+1, "error", err)
			time.Sleep(time.Second * time.Duration(i+1)) // Simple backoff
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			return nil
		}
		return fmt.Errorf("discord webhook failed with status: %d", resp.StatusCode)
	}
	return fmt.Errorf("failed to send webhook after retries")
}
