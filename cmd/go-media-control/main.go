package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"go-media-control/internal/app"
	"go-media-control/internal/config"
	"go-media-control/internal/discord"
	"go-media-control/internal/media"

	goapp "github.com/maxence-charriere/go-app/v10/pkg/app"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

//go:embed static/*
var staticFiles embed.FS

// contentTypeHandler adds proper content type headers to responses
type contentTypeHandler struct {
	handler http.Handler
}

func (h *contentTypeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set content type based on file extension
	if strings.HasSuffix(r.URL.Path, ".css") {
		w.Header().Set("Content-Type", "text/css")
	} else if strings.HasSuffix(r.URL.Path, ".js") {
		w.Header().Set("Content-Type", "application/javascript")
	} else if strings.HasSuffix(r.URL.Path, ".wasm") {
		w.Header().Set("Content-Type", "application/wasm")
	} else if strings.HasSuffix(r.URL.Path, ".html") || strings.HasSuffix(r.URL.Path, ".htm") {
		w.Header().Set("Content-Type", "text/html")
	} else if strings.HasSuffix(r.URL.Path, ".json") {
		w.Header().Set("Content-Type", "application/json")
	}

	// Call the underlying handler
	h.handler.ServeHTTP(w, r)
}

// Create a content type aware file server
func contentTypeAwareFileServer(root http.FileSystem) http.Handler {
	return &contentTypeHandler{
		handler: http.FileServer(root),
	}
}

func main() {
	// Initialize structured logging
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Setup slog
	slogHandler := slog.NewJSONHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(slogHandler))

	// Only initialize and validate config on the server side
	if goapp.IsServer {
		config.InitConfig()
		if err := config.ValidateConfig(); err != nil {
			logger.Fatal("Configuration error", zap.Error(err))
		}
	}

	// Set up go-app routing (works both for WASM and server)
	goapp.Route("/", func() goapp.Composer {
		return &app.MediaApp{}
	})

	// Run the app directly if we're in WASM mode
	if goapp.IsClient {
		goapp.RunWhenOnBrowser()
		return // Exit early as we're running in the browser
	}

	// Create API handler for media
	apiHandler := http.NewServeMux()

	// Media list endpoint
	apiHandler.HandleFunc("/api/media", func(w http.ResponseWriter, r *http.Request) {
		// Check for force-refresh parameter
		forceRefresh := r.URL.Query().Get("refresh") == "true"

		medias, err := media.FetchMedia(forceRefresh)
		if err != nil {
			logger.Error("Failed to fetch media", zap.Error(err))
			http.Error(w, fmt.Sprintf("Failed to fetch media: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(medias); err != nil {
			logger.Error("Failed to encode media response", zap.Error(err))
			http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
			return
		}
	})

	// Discord webhook endpoint
	apiHandler.HandleFunc("/api/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var mediaItem media.Media
		if err := json.NewDecoder(r.Body).Decode(&mediaItem); err != nil {
			logger.Error("Failed to decode media", zap.Error(err))
			http.Error(w, fmt.Sprintf("Failed to decode request: %v", err), http.StatusBadRequest)
			return
		}

		if err := discord.SendWebhook(mediaItem); err != nil {
			logger.Error("Failed to send Discord webhook", zap.Error(err))
			http.Error(w, fmt.Sprintf("Failed to send Discord webhook: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Set up go-app handler with custom HTML
	appHandler := &goapp.Handler{
		Name:        "Media Control",
		Description: "A media streaming PWA",
		Styles:      []string{"https://cdn.jsdelivr.net/npm/bulma@1.0.2/css/bulma.min.css", "/static/styles.css"},
		RawHeaders: []string{
			`<script>
				// Blocking style tag to prevent FOUC and layout warnings
				document.write('<style>body { visibility: hidden; }</style>');

				// When everything is loaded, remove the blocking style
				window.addEventListener('load', function() {
					document.documentElement.setAttribute('data-styles-loaded', 'true');
					document.querySelector('body').style.visibility = 'visible';
				});
			</script>`,
		},
	}

	// Create file servers for both web and static directories with proper MIME types
	webFileServer := contentTypeAwareFileServer(http.Dir("./out/web"))

	// Use embedded files for development, and the out/static directory for production
	var staticFS http.FileSystem
	_, err = os.Stat("./out/static")
	if err == nil {
		// Use the out/static directory when available (production)
		staticFS = http.Dir("./out/static")
		slog.Info("Using static files from ./out/static directory")
	} else {
		// Fall back to embedded files (development)
		staticFS = http.FS(staticFiles)
		slog.Info("Using embedded static files")
	}
	staticFileServer := contentTypeAwareFileServer(staticFS)

	// Combine API, static files, and app handlers
	combinedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For API endpoints
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}

		// For web directory files (WASM)
		if strings.HasPrefix(r.URL.Path, "/web/") {
			slog.Info("Serving web file", "path", r.URL.Path)
			http.StripPrefix("/web/", webFileServer).ServeHTTP(w, r)
			return
		}

		// For static assets (CSS, images, etc.)
		if strings.HasPrefix(r.URL.Path, "/static/") {
			slog.Info("Serving static file", "path", r.URL.Path)
			http.StripPrefix("/static/", staticFileServer).ServeHTTP(w, r)
			return
		}

		// Default to go-app handler
		appHandler.ServeHTTP(w, r)
	})

	// Start HTTP server with configurable port
	port := viper.GetString("PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf(":%s", port)
	srv := &http.Server{Addr: addr, Handler: combinedHandler}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		logger.Info("Starting server", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	<-sigChan
	logger.Info("Shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Shutdown failed", zap.Error(err))
	}
}
