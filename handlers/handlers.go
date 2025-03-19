package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/git-saj/go-media-control/internal/config"
	"github.com/git-saj/go-media-control/internal/discord"
	"github.com/git-saj/go-media-control/internal/xtream"
	"github.com/git-saj/go-media-control/templates"
)

// Handlers holds dependencies for HTTP handlers
type Handlers struct {
	logger        *slog.Logger
	xtreamClient  *xtream.Client
	discordClient *discord.WebhookClient
	commandPrefix string
}

// NewHandlers creates a new Handlers instance
func NewHandlers(logger *slog.Logger, cfg *config.Config) *Handlers {
	return &Handlers{
		logger:        logger,
		xtreamClient:  xtream.NewClient(cfg),
		discordClient: discord.NewWebhookClient(cfg.DiscordWebhook),
		commandPrefix: cfg.CommandPrefix,
	}
}

// paginate slices a channel list based on page and limit
func paginate(channels []xtream.MediaItem, page, limit int) ([]xtream.MediaItem, int) {
	total := len(channels)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return nil, total
	}
	if end > total {
		end = total
	}

	return channels[start:end], total
}

// HomeHandler serves the main UI at / with pagination
func (h *Handlers) HomeHandler(w http.ResponseWriter, r *http.Request) {
	media, err := h.xtreamClient.GetLiveStreams()
	if err != nil {
		h.logger.Error("Failed to fetch media for home", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get page and limit from query params (default: page=1, limit=25)
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 25
	}

	paginated, total := paginate(media, page, limit)
	templates.Home(paginated, page, limit, total).Render(r.Context(), w)
}

// SearchHandler filters channels based on search query with pagination
func (h *Handlers) SearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("query")
	media, err := h.xtreamClient.GetLiveStreams()
	if err != nil {
		h.logger.Error("Failed to fetch media for search", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Filter channels
	var filtered []xtream.MediaItem
	if query != "" { // Only filter if query is provided
		for _, ch := range media {
			if strings.Contains(strings.ToLower(ch.Name), strings.ToLower(query)) {
				filtered = append(filtered, ch)
			}
		}
	} else {
		filtered = media // Use full list if no query
	}

	// Get page and limit from form values (default: page=1, limit=25)
	pageStr := r.FormValue("page")
	limitStr := r.FormValue("limit")
	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 25
	}
	// Reset page to 1 when searching, unless explicitly set
	if query != "" && pageStr == "" {
		page = 1
	} else if page < 1 {
		page = 1
	}

	paginated, total := paginate(filtered, page, limit)
	templates.Results(paginated, page, limit, total).Render(r.Context(), w) // Render only results
}

// MediaHandler handles GET /api/media requests
func (h *Handlers) MediaHandler(w http.ResponseWriter, r *http.Request) {
	media, err := h.xtreamClient.GetLiveStreams()
	if err != nil {
		h.logger.Error("Failed to fetch media", "error", err)
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
	ChannelID int `json:"channel_id"`
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
		h.logger.Error("Failed to fetch media", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var streamURL string
	for _, item := range media {
		if item.StreamID == req.ChannelID {
			streamURL = item.StreamURL
			break
		}
	}

	if streamURL == "" {
		h.logger.Warn("Channel not found", "channel_id", req.ChannelID)
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	message := h.commandPrefix + " " + streamURL
	if err := h.discordClient.Send(message); err != nil {
		h.logger.Error("Failed to send to Discord", "error", err)
		http.Error(w, "Failed to send to Discord", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Message sent to Discord"))
}
