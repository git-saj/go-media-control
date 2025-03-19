package xtream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/git-saj/go-media-control/internal/cache"
	"github.com/git-saj/go-media-control/internal/config"
)

// Client represents an Xtream Code API client
type Client struct {
	BaseURL    string
	Username   string
	Password   string
	httpClient *http.Client
	cache      *cache.Cache[[]MediaItem]
}

// MediaItem represents a single media item from the Xtream Code API
type MediaItem struct {
	Name      string `json:"name"`
	StreamID  int    `json:"stream_id"` // From API response
	Logo      string `json:"stream_icon"`
	StreamURL string `json:"stream_url"`
}

// NewClient creates a new Xtream Code API client from the configuration
func NewClient(cfg *config.Config) *Client {
	return &Client{
		BaseURL:    cfg.XtreamBaseURL,
		Username:   cfg.XtreamUsername,
		Password:   cfg.XtreamPassword,
		httpClient: &http.Client{},
		cache:      cache.New[[]MediaItem](),
	}
}

// FetchLiveStreams fetches live streams from the Xtream Code API and constructs StreamURL
func (c *Client) FetchLiveStreams() ([]MediaItem, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_live_streams",
		c.BaseURL, c.Username, c.Password)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch live streams: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var rawMedia []struct {
		Name     string `json:"name"`
		StreamID int    `json:"stream_id"` // From API response
		Logo     string `json:"stream_icon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawMedia); err != nil {
		return nil, fmt.Errorf("failed to decode live streams: %w", err)
	}

	// Construct MediaItems with StreamURLs
	media := make([]MediaItem, len(rawMedia))
	for i, item := range rawMedia {
		media[i] = MediaItem{
			Name:     item.Name,
			StreamID: item.StreamID,
			Logo:     item.Logo,
			StreamURL: fmt.Sprintf("%s/%s/%s/%d.ts",
				c.BaseURL, c.Username, c.Password, item.StreamID),
		}
	}

	return media, nil
}

// GetLiveStreams fetches live streams, using the cache if available
func (c *Client) GetLiveStreams() ([]MediaItem, error) {
	// Check cache first
	if cached, ok := c.cache.Get(); ok {
		return cached, nil
	}

	// Fetch from API if cache is empty or expired
	media, err := c.FetchLiveStreams()
	if err != nil {
		return nil, err
	}

	// Store in cache with a 25-hour TTL
	c.cache.Set(media, time.Hour*25)

	return media, nil
}
