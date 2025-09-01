package xtream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/git-saj/go-media-control/internal/cache"
	"github.com/git-saj/go-media-control/internal/config"
)

// Client represents an Xtream Code API client
type Client struct {
	BaseURL       string
	Username      string
	Password      string
	Cache         *cache.Cache[[]MediaItem]
	CategoryCache *cache.Cache[[]Category]
	httpClient    *http.Client
	mu            sync.RWMutex
	streamURLs    map[int]string
}

// MediaItem represents a single media item from the Xtream Code API
type MediaItem struct {
	Name       string `json:"name"`
	StreamID   int    `json:"stream_id"` // From API response
	Logo       string `json:"stream_icon"`
	StreamURL  string `json:"stream_url"`
	CategoryID string `json:"category_id"`
}

// Category represents a category from the Xtream Code API
type Category struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
}

// NewClient creates a new Xtream Code API client from the configuration
func NewClient(cfg *config.Config) *Client {
	return &Client{
		BaseURL:       cfg.XtreamBaseURL,
		Username:      cfg.XtreamUsername,
		Password:      cfg.XtreamPassword,
		Cache:         cache.New[[]MediaItem](),
		CategoryCache: cache.New[[]Category](),
		httpClient:    &http.Client{},
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
		Name       string      `json:"name"`
		StreamID   json.Number `json:"stream_id"` // From API response, as json.Number for flexibility
		Logo       string      `json:"stream_icon"`
		CategoryID json.Number `json:"category_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawMedia); err != nil {
		return nil, fmt.Errorf("failed to decode live streams: %w", err)
	}

	// Construct MediaItems with StreamURLs
	media := make([]MediaItem, len(rawMedia))
	for i, item := range rawMedia {
		streamIDInt, _ := strconv.Atoi(string(item.StreamID))
		media[i] = MediaItem{
			Name:       item.Name,
			StreamID:   streamIDInt,
			Logo:       item.Logo,
			CategoryID: string(item.CategoryID),
			StreamURL: fmt.Sprintf("%s/%s/%s/%d.ts",
				c.BaseURL, c.Username, c.Password, streamIDInt),
		}
	}

	return media, nil
}

// FetchCategories fetches categories from the Xtream Code API
func (c *Client) FetchCategories() ([]Category, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_live_categories",
		c.BaseURL, c.Username, c.Password)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code for categories: %d", resp.StatusCode)
	}

	var categories []Category
	if err := json.NewDecoder(resp.Body).Decode(&categories); err != nil {
		return nil, fmt.Errorf("failed to decode categories: %w", err)
	}

	return categories, nil
}

// GetCategories fetches categories, using the cache if available
func (c *Client) GetCategories() ([]Category, error) {
	// Check cache first
	if cached, ok := c.CategoryCache.Get(); ok {
		return cached, nil
	}

	// Fetch from API if cache is empty or expired
	categories, err := c.FetchCategories()
	if err != nil {
		return nil, err
	}

	// Store in cache with a 24-hour TTL
	c.CategoryCache.Set(categories, time.Hour*24)

	return categories, nil
}

// GetLiveStreams fetches live streams, using the cache if available
func (c *Client) GetLiveStreams() ([]MediaItem, error) {
	// Check cache first
	if cached, ok := c.Cache.Get(); ok {
		return cached, nil
	}

	// Fetch from API if cache is empty or expired
	media, err := c.FetchLiveStreams()
	if err != nil {
		return nil, err
	}

	// Store in cache with a 24-hour TTL
	c.Cache.Set(media, time.Hour*24)

	// Populate the stream URL map
	c.mu.Lock()
	c.streamURLs = make(map[int]string, len(media))
	for _, item := range media {
		c.streamURLs[item.StreamID] = item.StreamURL
	}
	c.mu.Unlock()

	return media, nil
}

// GetStreamURL retrieves the stream URL for a given stream ID
func (c *Client) GetStreamURL(streamID int) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	url, ok := c.streamURLs[streamID]
	return url, ok
}

// ClearCache clears both the data cache and the URL map
func (c *Client) ClearCache() {
	c.Cache.Clear()
	c.mu.Lock()
	c.streamURLs = make(map[int]string)
	c.mu.Unlock()
}
