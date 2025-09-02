package xtream

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/git-saj/go-media-control/internal/cache"
	"github.com/git-saj/go-media-control/internal/config"
)

// Client represents an Xtream Code API client
type Client struct {
	BaseURL            string
	Username           string
	Password           string
	Cache              *cache.Cache[[]MediaItem]
	CategoryCache      *cache.Cache[[]Category]
	EpgCache           *cache.Cache[map[int]EpgData]
	httpClient         *http.Client
	mu                 sync.RWMutex
	streamURLs         map[int]string
	EpgFetchTime       time.Time
	streamIDs          []int
	disableEpgPrefetch bool
}

// MediaItem represents a single media item from the Xtream Code API
type MediaItem struct {
	Name           string `json:"name"`
	StreamID       int    `json:"stream_id"` // From API response
	Logo           string `json:"stream_icon"`
	StreamURL      string `json:"stream_url"`
	CategoryID     string `json:"category_id"`
	CurrentProgram *EpgListing
	NextProgram    *EpgListing
}

// EpgListing represents a single EPG entry for a media item
type EpgListing struct {
	Id          string `json:"id"`
	EpgId       string `json:"epg_id"`
	ChannelId   string `json:"channel_id"`
	Start       int64  `json:"start"`
	End         int64  `json:"end"`
	Lang        string `json:"lang"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// EpgData holds both parsed EPG and raw response for caching
type EpgData struct {
	Epg []EpgListing
	Raw string
}

// Category represents a category from the Xtream Code API
type Category struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
}

// NewClient creates a new Xtream Code API client from the configuration
func NewClient(cfg *config.Config) *Client {
	client := &Client{
		BaseURL:            cfg.XtreamBaseURL,
		Username:           cfg.XtreamUsername,
		Password:           cfg.XtreamPassword,
		Cache:              cache.New[[]MediaItem](),
		CategoryCache:      cache.New[[]Category](),
		EpgCache:           cache.New[map[int]EpgData](),
		httpClient:         &http.Client{},
		EpgFetchTime:       time.Time{},
		streamIDs:          []int{},
		streamURLs:         make(map[int]string),
		mu:                 sync.RWMutex{},
		disableEpgPrefetch: cfg.DisableEpgPrefetch,
	}

	// Start background EPG prefetching only if not disabled
	if !client.disableEpgPrefetch {
		go client.prefetchEPGs()
	}

	return client
}

// FetchLiveStreams fetches live streams from the Xtream Code API and constructs StreamURL
func (c *Client) fetchLiveStreams() ([]MediaItem, error) {
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

	c.streamIDs = make([]int, 0, len(media))
	for _, m := range media {
		c.streamIDs = append(c.streamIDs, m.StreamID)
	}

	return media, nil
}

// GetLiveStreams retrieves live streams with caching and EPG prefetching
func (c *Client) GetLiveStreams() ([]MediaItem, error) {
	c.mu.RLock()
	if items, ok := c.Cache.Get(); ok {
		slog.Info("LiveStreams cache hit")
		c.mu.RUnlock()
		return items, nil
	}
	c.mu.RUnlock()

	items, err := c.fetchLiveStreams()
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.Cache.Set(items, time.Minute*10)
	c.streamIDs = make([]int, 0, len(items))
	for _, m := range items {
		c.streamIDs = append(c.streamIDs, m.StreamID)
	}

	// Prefetch EPG asynchronously if needed and not disabled
	if !c.disableEpgPrefetch && (c.EpgFetchTime.IsZero() || time.Since(c.EpgFetchTime) > 24*time.Hour) {
		c.EpgFetchTime = time.Now()
		go c.doPrefetchEPGs()
	}
	c.mu.Unlock()

	return items, nil
}

// FetchCategories fetches live categories from the Xtream Code API
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

// FetchEpgForStream fetches EPG data for a specific stream
func (c *Client) FetchEpgForStream(streamID int) ([]EpgListing, string, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_epg&stream_id=%d",
		c.BaseURL, c.Username, c.Password, streamID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch EPG for stream %d: %w", streamID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status code for EPG stream %d: %d", streamID, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read EPG response for stream %d: %w", streamID, err)
	}
	rawBody := string(bodyBytes)

	var epg []EpgListing
	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&epg); err != nil {
		return nil, rawBody, fmt.Errorf("failed to decode EPG for stream %d: %w\nRaw Response: %s", streamID, err, rawBody)
	}

	// Decode base64 in title and description
	for i := range epg {
		if epg[i].Title != "" {
			title, _ := base64.StdEncoding.DecodeString(epg[i].Title)
			epg[i].Title = string(title)
		}
		if epg[i].Description != "" {
			desc, _ := base64.StdEncoding.DecodeString(epg[i].Description)
			epg[i].Description = string(desc)
		}
	}

	return epg, rawBody, nil
}

// GetEpgForStream fetches EPG for a stream, using cache if available
func (c *Client) prefetchEPGs() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.doPrefetchEPGs()
		}
	}
}

func (c *Client) doPrefetchEPGs() {
	if c.disableEpgPrefetch {
		return
	}
	// Get current media items from cache
	c.mu.RLock()
	items, ok := c.Cache.Get()
	c.mu.RUnlock()
	if !ok {
		return // No cached items
	}

	// Fetch categories if not cached, to filter UK ones
	categories, err := c.GetCategories()
	if err != nil {
		return // Can't filter without categories
	}

	// Create category ID to name map
	catMap := make(map[string]string)
	for _, cat := range categories {
		catMap[cat.CategoryID] = cat.CategoryName
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Limit concurrent requests
	for _, item := range items {
		// Only prefetch if category contains "UK"
		if catName, exists := catMap[item.CategoryID]; exists && strings.Contains(strings.ToLower(catName), "uk") {
			wg.Add(1)
			go func(streamID int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				// Fetch and cache EPG
				_, _, err := c.GetEpgForStream(streamID)
				if err != nil {
					// Could log error, but for now ignore to avoid spam
				}
			}(item.StreamID)
		}
	}
	wg.Wait()
}

func (c *Client) GetEpgForStream(streamID int) ([]EpgListing, string, error) {
	// Check cache first
	c.mu.RLock()
	if cachedMap, ok := c.EpgCache.Get(); ok {
		if epgData, exists := cachedMap[streamID]; exists {
			// Return a copy to avoid modifying cache
			epgCopy := make([]EpgListing, len(epgData.Epg))
			copy(epgCopy, epgData.Epg)
			c.mu.RUnlock()
			slog.Info("EPG cache hit", "stream_id", streamID)
			return epgCopy, epgData.Raw, nil
		}
	}
	c.mu.RUnlock()

	// Fetch from API if cache is empty or expired
	epg, rawBody, err := c.FetchEpgForStream(streamID)
	if err != nil {
		return nil, "", err
	}
	slog.Info("EPG fetched from API", "stream_id", streamID, "program_count", len(epg))

	// Store parsed epg and raw in cache with a 24-hour TTL
	c.mu.Lock()
	if cachedMap, ok := c.EpgCache.Get(); ok {
		// Copy the existing map to avoid concurrent modification
		newMap := make(map[int]EpgData, len(cachedMap)+1)
		for k, v := range cachedMap {
			newMap[k] = EpgData{
				Epg: append([]EpgListing(nil), v.Epg...),
				Raw: v.Raw,
			}
		}
		newMap[streamID] = EpgData{
			Epg: append([]EpgListing(nil), epg...),
			Raw: rawBody,
		}
		c.EpgCache.Set(newMap, time.Hour*24)
	} else {
		newMap := map[int]EpgData{streamID: EpgData{
			Epg: append([]EpgListing(nil), epg...),
			Raw: rawBody,
		}}
		c.EpgCache.Set(newMap, time.Hour*24)
	}
	c.mu.Unlock()

	return epg, rawBody, nil
}

// GetStreamURL retrieves the stream URL for a given stream ID
func (c *Client) GetStreamURL(streamID int) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	url, ok := c.streamURLs[streamID]
	return url, ok
}

// ClearCache clears both the data cache, EPG cache, and the URL map
func (c *Client) ClearCache() {
	c.Cache.Clear()
	c.EpgCache.Clear()
	c.mu.Lock()
	c.streamURLs = make(map[int]string)
	c.mu.Unlock()
}
