// Package media handles fetching, parsing, and caching media data from various sources.
// It supports M3U playlists and Xtreams API as data sources and implements
// thread-safe caching with TTL-based expiration.
package media

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Media represents a single media item with name, URL, and logo.
// It is used for both M3U and Xtreams API data sources.
type Media struct {
	Name string // Display name of the media
	URL  string // Streaming URL
	Logo string // URL to the logo/thumbnail image
}

var (
	client = &http.Client{Timeout: 10 * time.Second}

	// Cache for media data
	mediaCache     []Media
	mediaCacheMu   sync.RWMutex
	cacheTimestamp time.Time
	cacheTTL       = 30 * time.Minute // Cache TTL of 30 minutes
)

// FetchMedia retrieves media data from the configured source with caching.
// It returns a list of Media items either from cache or by fetching from the source.
//
// The source is determined by the MEDIA_SOURCE environment variable and can be
// either "m3u" or "xtreams". Cache TTL is 30 minutes by default.
//
// Parameters:
//   - forceRefresh: When true, bypass cache and fetch fresh data from the source
//
// Returns:
//   - []Media: List of media items
//   - error: Any error encountered during fetching
func FetchMedia(forceRefresh bool) ([]Media, error) {
	// Check if we have a valid cache and are not forced to refresh
	mediaCacheMu.RLock()
	cacheValid := !forceRefresh && len(mediaCache) > 0 && time.Since(cacheTimestamp) < cacheTTL
	mediaCacheMu.RUnlock()

	if cacheValid {
		slog.Info("Using cached media data", "count", len(mediaCache), "age", time.Since(cacheTimestamp).String())
		mediaCacheMu.RLock()
		cachedMedia := append([]Media{}, mediaCache...) // Create a copy to avoid race conditions
		mediaCacheMu.RUnlock()
		return cachedMedia, nil
	}

	if forceRefresh {
		slog.Info("Force refreshing media data")
	}

	// No valid cache, fetch new data
	source := strings.ToLower(viper.GetString("MEDIA_SOURCE"))
	slog.Info("Fetching fresh media data", "source", source)

	var medias []Media
	var err error

	switch source {
	case "m3u":
		medias, err = fetchM3U(viper.GetString("M3U_URL"))
	case "xtreams":
		medias, err = fetchXtreamsAPI(
			viper.GetString("XTREAMS_BASE_URL"),
			viper.GetString("XTREAMS_USERNAME"),
			viper.GetString("XTREAMS_PASSWORD"),
		)
	default:
		return nil, fmt.Errorf("unsupported media source: %s", source)
	}

	if err != nil {
		return nil, err
	}

	// Update the cache with the new data
	mediaCacheMu.Lock()
	mediaCache = medias
	cacheTimestamp = time.Now()
	mediaCacheMu.Unlock()

	slog.Info("Updated media cache", "count", len(medias))
	return medias, nil
}

// fetchM3U retrieves and parses a M3U playlist from the given URL.
// It extracts media names, URLs, and logo information from the playlist.
//
// Parameters:
//   - url: The URL of the M3U playlist to fetch
//
// Returns:
//   - []Media: List of media items parsed from the M3U file
//   - error: Any error encountered during fetching or parsing
func fetchM3U(url string) ([]Media, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching m3u: %w", err)
	}
	defer resp.Body.Close()

	var mediaList []Media
	scanner := bufio.NewScanner(resp.Body)
	var current Media

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#EXTM3U") {
			continue
		}
		if strings.HasPrefix(line, "#EXTINF") {
			current = Media{} // Reset for each new entry

			// Extract attributes like tvg-logo before splitting by comma
			attrs := extractAttributes(line)
			if logo, ok := attrs["tvg-logo"]; ok {
				current.Logo = logo
			}

			// Extract channel name - everything after the last comma
			parts := strings.SplitN(line, ",", 2)
			if len(parts) == 2 {
				current.Name = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "http") {
			current.URL = line
			if current.Name != "" && current.URL != "" {
				mediaList = append(mediaList, current)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning m3u: %w", err)
	}
	slog.Info("Fetched m3u media", "count", len(mediaList))
	return mediaList, nil
}

// Helper to extract attributes from #EXTINF line
func extractAttributes(line string) map[string]string {
	attrs := make(map[string]string)

	// Find attributes before the comma
	if idx := strings.Index(line, ","); idx > 0 {
		line = line[:idx]
	}

	// Split by space to get individual attributes
	parts := strings.Fields(line)

	for _, part := range parts {
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				value := kv[1]

				// Remove surrounding quotes if present
				value = strings.Trim(value, "\"")

				attrs[key] = value
			}
		}
	}

	return attrs
}

type Stream struct {
	Num          int    `json:"num"`
	Name         string `json:"name"`
	StreamType   string `json:"stream_type"`
	StreamID     int    `json:"stream_id"`
	StreamIcon   string `json:"stream_icon"`
	EPGChannelID string `json:"epg_channel_id"`
	Added        string `json:"added"`
	CustomSID    string `json:"custom_sid"`
	TVArchive    int    `json:"tv_archive"`
	DirectSource string `json:"direct_source"`
	CategoryID   string `json:"category_id"`
	Thumbnail    string `json:"thumbnail"`
}

type XtreamsResponse struct {
	Streams []Stream `json:"streams"`
}

func fetchXtreamsAPI(baseURL, username, password string) ([]Media, error) {
	url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_live_streams", baseURL, username, password)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching xtreams API: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body for debugging
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// Log the first 500 characters to see structure
	bodyPreview := string(respBody)
	if len(bodyPreview) > 1000 {
		bodyPreview = bodyPreview[:1000] + "..."
	}
	slog.Info("Xtreams API response preview", "response", bodyPreview)

	// Try decoding as array first
	var streams []Stream
	if err := json.Unmarshal(respBody, &streams); err != nil {
		// If that fails, try the original object format
		var xtreams XtreamsResponse
		if err := json.Unmarshal(respBody, &xtreams); err != nil {
			return nil, fmt.Errorf("decoding xtreams response: %w", err)
		}
		streams = xtreams.Streams
	}

	var mediaList []Media
	for _, stream := range streams {
		// Construct the stream URL based on ID, username, and password
		streamURL := fmt.Sprintf("%s/%s/%s/%d.ts",
			baseURL,
			username,
			password,
			stream.StreamID)

		// Create Media object with the right fields
		mediaList = append(mediaList, Media{
			Name: stream.Name,
			URL:  streamURL,
			Logo: stream.StreamIcon,
		})
	}
	slog.Info("Fetched xtreams media", "count", len(mediaList))
	return mediaList, nil
}
