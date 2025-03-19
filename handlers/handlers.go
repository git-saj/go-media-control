package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/git-saj/go-media-control/internal/config"
	"github.com/git-saj/go-media-control/internal/discord"
	"github.com/git-saj/go-media-control/internal/xtream"
)

// Handlers holds dependencies for HTTP handlers.
type Handlers struct {
	logger        *slog.Logger
	xtreamClient  *xtream.Client
	discordClient *discord.WebhookClient
	commandPrefix string
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(logger *slog.Logger, cfg *config.Config) *Handlers {
	return &Handlers{
		logger:        logger,
		xtreamClient:  xtream.NewClient(cfg),
		discordClient: discord.NewWebhookClient(cfg.DiscordWebhook),
		commandPrefix: cfg.CommandPrefix,
	}
}

// MediaHandler handles GET /api/media requests
func (h *Handlers) MediaHandler(w http.ResponseWriter, r *http.Request) {
	media, err := h.xtreamClient.GetLiveStreams()
	if err != nil {
		h.logger.Error("Failed to fetch live streams", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(media); err != nil {
		h.logger.Error("Failed to encode media response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// SendRequest represents the expected JSON body for /api/send
type SendRequest struct {
	ChannelName string `json:"channel_name"`
}

// SendHandler handles POST /api/send requests
func (h *Handlers) SendHandler(w http.ResponseWriter, r *http.Request) {
	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Invalid request body", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	media, err := h.xtreamClient.GetLiveStreams()
	if err != nil {
		h.logger.Error("Failed to fetch live streams", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var streamURL string
	for _, item := range media {
		if item.Name == req.ChannelName {
			streamURL = item.StreamURL
			break
		}
	}

	if streamURL == "" {
		h.logger.Warn("Channel not found", "channel_name", req.ChannelName)
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	message := h.commandPrefix + " " + streamURL
	if err := h.discordClient.Send(message); err != nil {
		h.logger.Error("Failed to send message", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

}
