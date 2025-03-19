package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/git-saj/go-media-control/internal/config"
	"github.com/git-saj/go-media-control/internal/xtream"
)

// Handlers golds dependencies for HTTP handlers.
type Handlers struct {
	logger       *slog.Logger
	xtreamClient *xtream.Client
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(logger *slog.Logger, cfg *config.Config) *Handlers {
	return &Handlers{
		logger:       logger,
		xtreamClient: xtream.NewClient(cfg),
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
