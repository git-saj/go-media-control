package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

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
	h.logger.Info("Handlers initialized", "xtream_baseurl", cfg.XtreamBaseURL, "base_path", cfg.BasePath, "has_auth", h.hasAuth, "disable_epg_prefetch", h.cfg.DisableEpgPrefetch)
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

	// Fetch EPG for paginated channels concurrently
	var wg sync.WaitGroup
	for i := range paginated {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			now := time.Now().Unix()
			epg, _, err := h.xtreamClient.GetEpgForStream(paginated[idx].StreamID)
			if err != nil {
				h.logger.Warn("Failed to fetch EPG for stream", "stream_id", paginated[idx].StreamID, "error", err)
				return
			}
			h.logger.Info("Fetched EPG", "stream_id", paginated[idx].StreamID, "program_count", len(epg))
			for _, program := range epg {
				if now >= program.Start && now <= program.End {
					current := program
					if len(current.Title) > 20 {
						current.Title = current.Title[:20] + "..."
					}
					paginated[idx].CurrentProgram = &current
				} else if now < program.Start && paginated[idx].NextProgram == nil {
					next := program
					if len(next.Title) > 20 {
						next.Title = next.Title[:20] + "..."
					}
					paginated[idx].NextProgram = &next
				}
			}
			if paginated[idx].CurrentProgram != nil {
				h.logger.Info("Set current program", "stream_id", paginated[idx].StreamID, "title", paginated[idx].CurrentProgram.Title)
			}
			if paginated[idx].NextProgram != nil {
				h.logger.Info("Set next program", "stream_id", paginated[idx].StreamID, "title", paginated[idx].NextProgram.Title)
			}
			if paginated[idx].CurrentProgram == nil && paginated[idx].NextProgram == nil {
				h.logger.Info("No current or next program found", "stream_id", paginated[idx].StreamID)
			}
		}(i)
	}
	wg.Wait()

	categories, err := h.xtreamClient.GetCategories()
	if err != nil {
		h.logger.Error("Failed to fetch categories", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Check if this is an HTMX request for partial rendering
	isHTMX := r.Header.Get("HX-Request") == "true"
	if isHTMX {
		templates.Results(paginated, page, limit, total, h.basePath, "", "").Render(r.Context(), w)
	} else {

		templates.Home(paginated, page, limit, total, h.basePath, h.hasAuth, categories, "", "").Render(r.Context(), w)
	}
}

// SearchHandler filters channels based on search query with pagination
func (h *Handlers) SearchHandler(w http.ResponseWriter, r *http.Request) {
	totalStart := time.Now()
	h.logger.Info("SearchHandler started", "method", r.Method, "query", r.URL.Query().Get("query"))

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
	h.logger.Info("GetLiveStreams completed", "duration", time.Since(totalStart))
	// Filter channels
	filterStart := time.Now()
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
	h.logger.Info("Filtering completed", "duration", time.Since(filterStart))

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

	// Fetch EPG for paginated channels concurrently
	epgStart := time.Now()
	var wg sync.WaitGroup
	for i := range paginated {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			now := time.Now().Unix()
			epg, _, err := h.xtreamClient.GetEpgForStream(paginated[idx].StreamID)
			if err != nil {
				h.logger.Warn("Failed to fetch EPG for stream", "stream_id", paginated[idx].StreamID, "error", err)
				return
			}
			for _, program := range epg {
				if now >= program.Start && now <= program.End {
					current := program
					if len(current.Title) > 20 {
						current.Title = current.Title[:20] + "..."
					}
					paginated[idx].CurrentProgram = &current
				} else if now < program.Start && paginated[idx].NextProgram == nil {
					next := program
					if len(next.Title) > 20 {
						next.Title = next.Title[:20] + "..."
					}
					paginated[idx].NextProgram = &next
				}
			}
		}(i)
	}
	wg.Wait()
	h.logger.Info("EPG fetch completed", "duration", time.Since(epgStart))

	// Check if this is an HTMX request for partial rendering
	isHTMX := r.Header.Get("HX-Request") == "true"
	if isHTMX {
		templates.Results(paginated, page, limit, total, h.basePath, query, categoryStr).Render(r.Context(), w)
	} else {
		catStart := time.Now()
		categories, err := h.xtreamClient.GetCategories()
		if err != nil {
			h.logger.Error("Failed to fetch categories for search", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		h.logger.Info("GetCategories completed", "duration", time.Since(catStart))

		renderStart := time.Now()
		templates.Home(paginated, page, limit, total, h.basePath, h.hasAuth, categories, query, categoryStr).Render(r.Context(), w)
		h.logger.Info("Template render completed", "duration", time.Since(renderStart))
		h.logger.Info("SearchHandler total duration", "duration", time.Since(totalStart))
	}
}

// RefreshCacheHandler clears the cache and returns refreshed results
func (h *Handlers) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	// Clear the media cache
	h.xtreamClient.Cache.Clear()
	// Clear the EPG cache
	h.xtreamClient.EpgCache.Clear()
	// Reset EPG fetch time to force refetch
	h.xtreamClient.EpgFetchTime = time.Time{}

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

	// Fetch EPG for paginated channels concurrently
	var wg sync.WaitGroup
	for i := range paginated {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			now := time.Now().Unix()
			epg, _, err := h.xtreamClient.GetEpgForStream(paginated[idx].StreamID)
			if err != nil {
				h.logger.Warn("Failed to fetch EPG for stream", "stream_id", paginated[idx].StreamID, "error", err)
				return
			}
			h.logger.Info("Fetched EPG", "stream_id", paginated[idx].StreamID, "program_count", len(epg))
			for _, program := range epg {
				if now >= program.Start && now <= program.End {
					current := program
					if len(current.Title) > 20 {
						current.Title = current.Title[:20] + "..."
					}
					paginated[idx].CurrentProgram = &current
				} else if now < program.Start && paginated[idx].NextProgram == nil {
					next := program
					if len(next.Title) > 20 {
						next.Title = next.Title[:20] + "..."
					}
					paginated[idx].NextProgram = &next
				}
			}
			if paginated[idx].CurrentProgram != nil {
				h.logger.Info("Set current program", "stream_id", paginated[idx].StreamID, "title", paginated[idx].CurrentProgram.Title)
			}
			if paginated[idx].NextProgram != nil {
				h.logger.Info("Set next program", "stream_id", paginated[idx].StreamID, "title", paginated[idx].NextProgram.Title)
			}
			if paginated[idx].CurrentProgram == nil && paginated[idx].NextProgram == nil {
				h.logger.Info("No current or next program found", "stream_id", paginated[idx].StreamID)
			}
		}(i)
	}
	wg.Wait()

	templates.Results(paginated, page, limit, total, h.basePath, "", "").Render(r.Context(), w)
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

	h.logger.Info("Sending command", "channel", req.ChannelID, "url", streamURL)
	err := h.discordClient.Send(fmt.Sprintf("%sload %s", h.commandPrefix, streamURL))
	if err != nil {
		h.logger.Error("Failed to send Discord message", "error", err)
		http.Error(w, "Failed to send command", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ClearCacheHandler(w http.ResponseWriter, r *http.Request) {
	// Clear media cache
	h.xtreamClient.Cache.Clear()
	// Clear EPG cache
	h.xtreamClient.EpgCache.Clear()
	// Clear categories cache if exists
	h.xtreamClient.CategoryCache.Clear()
	// Reset EPG fetch time
	h.xtreamClient.EpgFetchTime = time.Time{}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Cache cleared"))
}

// EpgHandler handles GET /api/epg?stream_id=XXX requests and returns HTML for HTMX
func (h *Handlers) EpgHandler(w http.ResponseWriter, r *http.Request) {
	streamIDStr := r.URL.Query().Get("stream_id")
	streamID, err := strconv.Atoi(streamIDStr)
	if err != nil {
		h.logger.Warn("Invalid stream_id", "stream_id", streamIDStr, "error", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<p class='text-red-500'>Invalid stream ID</p>"))
		return
	}

	if r.URL.Query().Get("close") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(""))
		return
	}

	epg, rawResponse, err := h.xtreamClient.GetEpgForStream(streamID)
	if err != nil {
		h.logger.Error("Failed to fetch EPG", "stream_id", streamID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("<p class='text-red-500'>Failed to load EPG: " + err.Error() + "</p><details><summary>Debug Info</summary><pre>" + rawResponse + "</pre></details>"))
		return
	}

	if len(epg) == 0 {
		if strings.Contains(rawResponse, "user_info") {
			w.Write([]byte("<p class='text-gray-500'>EPG not supported by your Xtream provider.</p>"))
		} else {
			debugInfo := fmt.Sprintf("<p class='text-gray-500'>No EPG available.</p><details><summary>Debug Info</summary><pre>%s</pre></details>", rawResponse)
			w.Write([]byte(debugInfo))
		}
		return
	}

	// Build simple HTML for EPG listings
	html := "<div class='mt-2'><button class='btn btn-xs btn-ghost float-right' hx-get='" + h.basePath + fmt.Sprintf("api/epg?stream_id=%d&close=true", streamID) + "' hx-target='#epg-" + strconv.Itoa(streamID) + "' hx-swap='innerHTML'>×</button><ul class='list-disc list-inside clear-both'>"
	for _, program := range epg {
		startTime := time.Unix(program.Start, 0).Format("15:04")
		endTime := time.Unix(program.End, 0).Format("15:04")
		html += fmt.Sprintf("<li><strong>%s</strong> (%s - %s): %s</li>",
			program.Title, startTime, endTime, program.Description)
	}
	html += "</ul></div>"

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
