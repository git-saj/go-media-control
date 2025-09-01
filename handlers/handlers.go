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
	basePath      string
	cfg           *config.Config
	hasAuth       bool
}

// NewHandlers creates a new Handlers instance
func NewHandlers(logger *slog.Logger, cfg *config.Config) *Handlers {
	h := &Handlers{
		logger:        logger,
		xtreamClient:  xtream.NewClient(cfg),
		discordClient: discord.NewWebhookClient(cfg.DiscordWebhook),
		commandPrefix: cfg.CommandPrefix,
		basePath:      cfg.BasePath,
		cfg:           cfg,
		hasAuth:       !cfg.DisableAuth,
	}
	h.logger.Info("Handlers initialized", "xtream_baseurl", cfg.XtreamBaseURL, "base_path", cfg.BasePath, "has_auth", h.hasAuth)
	return h
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

	// Get page and limit from query params (default: page=1, limit=15)
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 15
	}

	paginated, total := paginate(media, page, limit)

	// Check if this is an HTMX request for partial rendering
	isHTMX := r.Header.Get("HX-Request") == "true"
	if isHTMX {
		templates.Results(paginated, page, limit, total, h.basePath, "").Render(r.Context(), w)
	} else {

		templates.Results(paginated, page, limit, total, h.basePath, "").Render(r.Context(), w)
	}
}

// SearchHandler filters channels based on search query with pagination
func (h *Handlers) SearchHandler(w http.ResponseWriter, r *http.Request) {
	var query, pageStr, limitStr, categoryStr string
	if r.Method == "GET" {
		query = r.URL.Query().Get("query")
		pageStr = r.URL.Query().Get("page")
		limitStr = r.URL.Query().Get("limit")
		categoryStr = r.URL.Query().Get("category")
	} else {
		query = r.FormValue("query")
		pageStr = r.FormValue("page")
		limitStr = r.FormValue("limit")
		categoryStr = r.FormValue("category")
	}

	media, err := h.xtreamClient.GetLiveStreams()
	if err != nil {
		h.logger.Error("Failed to fetch media for search", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Filter channels
	var filtered []xtream.MediaItem
	if categoryStr != "" {
		for _, ch := range media {
			if ch.CategoryID == categoryStr {
				filtered = append(filtered, ch)
			}
		}
	} else {
		filtered = media
	}

	if query != "" {
		var nameFiltered []xtream.MediaItem
		for _, ch := range filtered {
			if strings.Contains(strings.ToLower(ch.Name), strings.ToLower(query)) {
				nameFiltered = append(nameFiltered, ch)
			}
		}
		filtered = nameFiltered
	}

	// Get page and limit (default: page=1, limit=15)
	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 15
	}
	// Reset page to 1 when searching, unless explicitly set
	if query != "" && pageStr == "" {
		page = 1
	} else if page < 1 {
		page = 1
	}

	paginated, total := paginate(filtered, page, limit)
	templates.Results(paginated, page, limit, total, h.basePath, "").Render(r.Context(), w)
}

// RefreshCacheHandler clears the cache and returns refreshed results
func (h *Handlers) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	// Clear the client's cache
	h.xtreamClient.ClearCache()
	h.xtreamClient.Cache.Clear()

	media, err := h.xtreamClient.GetLiveStreams()
	if err != nil {
		h.logger.Error("Failed to fetch media", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get page and limit from query params (default: page=1, limit=15)
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 15
	}

	paginated, total := paginate(media, page, limit)
	templates.Results(paginated, page, limit, total, h.basePath, "").Render(r.Context(), w)
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

	streamURL, ok := h.xtreamClient.GetStreamURL(req.ChannelID)
	if !ok {
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
