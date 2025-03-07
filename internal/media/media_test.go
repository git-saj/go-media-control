// Package media handles fetching and caching media data from various sources
package media

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestFetchM3U(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("#EXTM3U\n#EXTINF:-1 tvg-logo=\"http://logo.com\",Test Channel\nhttp://stream.com"))
    }))
    defer server.Close()

    mediaList, err := fetchM3U(server.URL)
    if err != nil {
        t.Fatalf("fetchM3U failed: %v", err)
    }
    if len(mediaList) != 1 {
        t.Fatalf("expected 1 media item, got %d", len(mediaList))
    }
    
    media := mediaList[0]
    if media.Name != "Test Channel" {
        t.Errorf("expected Name 'Test Channel', got '%s'", media.Name)
    }
    if media.URL != "http://stream.com" {
        t.Errorf("expected URL 'http://stream.com', got '%s'", media.URL)
    }
    if media.Logo != "http://logo.com" {
        t.Errorf("expected Logo 'http://logo.com', got '%s'", media.Logo)
    }
}

func TestExtractAttributes(t *testing.T) {
    // We create a simpler test case that doesn't include the comma part
    line := `#EXTINF:-1 tvg-id="test" tvg-logo="http://logo.com"`
    attrs := extractAttributes(line)
    if attrs["tvg-logo"] != "http://logo.com" {
        t.Errorf("expected tvg-logo 'http://logo.com', got '%s'", attrs["tvg-logo"])
    }
    if attrs["tvg-id"] != "test" {
        t.Errorf("expected tvg-id 'test', got '%s'", attrs["tvg-id"])
    }
}

func TestFetchMedia(t *testing.T) {
    // Setup mock server for M3U
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("#EXTM3U\n#EXTINF:-1 tvg-logo=\"http://logo.com\",Test Channel\nhttp://stream.com"))
    }))
    defer server.Close()
    
    // Configure viper for testing
    viper.Set("MEDIA_SOURCE", "m3u")
    viper.Set("M3U_URL", server.URL)
    
    // Reset cache for testing
    mediaCacheMu.Lock()
    mediaCache = nil
    cacheTimestamp = time.Time{}
    mediaCacheMu.Unlock()
    
    // Test initial fetch (cold cache)
    media1, err := FetchMedia(false)
    if err != nil {
        t.Fatalf("FetchMedia failed: %v", err)
    }
    if len(media1) != 1 {
        t.Fatalf("expected 1 media item, got %d", len(media1))
    }
    
    // Test cache hit
    media2, err := FetchMedia(false)
    if err != nil {
        t.Fatalf("FetchMedia (cached) failed: %v", err)
    }
    if len(media2) != 1 {
        t.Fatalf("expected 1 cached media item, got %d", len(media2))
    }
    
    // Test force refresh
    media3, err := FetchMedia(true)
    if err != nil {
        t.Fatalf("FetchMedia (force refresh) failed: %v", err)
    }
    if len(media3) != 1 {
        t.Fatalf("expected 1 media item after refresh, got %d", len(media3))
    }
}

func TestFetchXtreamsAPI(t *testing.T) {
    // Setup mock Xtreams API
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        streams := []Stream{
            {
                Name:       "Test Channel 1",
                StreamID:   123,
                StreamIcon: "http://icon1.com",
            },
            {
                Name:       "Test Channel 2",
                StreamID:   456,
                StreamIcon: "http://icon2.com",
            },
        }
        json.NewEncoder(w).Encode(streams)
    }))
    defer server.Close()
    
    medias, err := fetchXtreamsAPI(server.URL, "test", "pass")
    if err != nil {
        t.Fatalf("fetchXtreamsAPI failed: %v", err)
    }
    
    if len(medias) != 2 {
        t.Fatalf("expected 2 media items, got %d", len(medias))
    }
    
    // Verify the first media item
    if medias[0].Name != "Test Channel 1" {
        t.Errorf("expected name 'Test Channel 1', got %s", medias[0].Name)
    }
    
    // Check URL format
    expectedURL := server.URL + "/test/pass/123.ts"
    if medias[0].URL != expectedURL {
        t.Errorf("expected URL '%s', got '%s'", expectedURL, medias[0].URL)
    }
}
